package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"

	"start/internal/mailer"

	"github.com/sirupsen/logrus"
)

var (
	ErrInvalidMailInput = errors.New("invalid mail input")
	ErrMailQueueFull    = errors.New("mail queue is full")
	ErrDisabledMailer   = errors.New("mailer is not configured")
)

type SendMailAttachment struct {
	Filename string
	Data     []byte
}

type SendMailInput struct {
	To          string
	Subject     string
	Body        string
	Attachments []SendMailAttachment
}

func (s *Service) SendMail(ctx context.Context, in SendMailInput) (int, error) {
	// Check if mailer is disabled
	if _, ok := s.mailer.(mailer.DisabledSender); ok {
		return 0, ErrDisabledMailer
	}

	if in.To == "" || in.Subject == "" || in.Body == "" {
		logrus.Error("invalid mail input: missing required fields")
		return 0, ErrInvalidMailInput
	}

	if _, err := mail.ParseAddress(in.To); err != nil {
		return 0, fmt.Errorf("%w: invalid recipient", ErrInvalidMailInput)
	}

	// Convert service attachments to mailer attachments
	attachments := make([]mailer.Attachment, len(in.Attachments))
	var attachmentBytes int
	for i, att := range in.Attachments {
		// skip attachment if filename equals body text (common when using Apple Shortcuts to
		// send mail with a single attachment, where the filename is used as the body text and
		// the actual attachment data is empty)
		if att.Filename == in.Body+".txt" {
			continue
		}

		attachmentBytes += len(att.Data)
		attachments[i] = mailer.Attachment{
			Filename: att.Filename,
			Data:     att.Data,
		}
	}

	// Calculate total byte count
	totalBytes := len(in.To) + len(in.Subject) + len(in.Body) + attachmentBytes

	// Queue the mail task (non-blocking) instead of sending directly
	task := mailTask{
		msg: mailer.Message{
			To:          in.To,
			Subject:     in.Subject,
			Body:        in.Body,
			Attachments: attachments,
		},
	}

	select {
	case s.mailQueue <- task:
		// Task queued successfully
		return totalBytes, nil
	case <-ctx.Done():
		// Request context cancelled while queuing
		return 0, ctx.Err()
	default:
		// Queue is full; reject the request to apply backpressure
		return 0, ErrMailQueueFull
	}
}

// StartMailWorker starts a background goroutine that processes mail tasks.
// Call this once during service initialization.
func (s *Service) StartMailWorker() {
	go func() {
		for {
			select {
			case task, ok := <-s.mailQueue:
				if !ok {
					// mailQueue was closed, exit worker
					return
				}
				// Send email with background context (worker is not tied to request lifetime)
				if err := s.mailer.Send(context.Background(), task.msg); err != nil {
					logrus.Errorf("failed to send mail to %s: %v", task.msg.To, err)
				}
			case <-s.done:
				// Drain remaining tasks before exiting
				for {
					select {
					case task, ok := <-s.mailQueue:
						if !ok {
							return
						}
						if err := s.mailer.Send(context.Background(), task.msg); err != nil {
							logrus.Errorf("failed to send mail to %s: %v", task.msg.To, err)
						}
					default:
						return
					}
				}
			}
		}
	}()
}
