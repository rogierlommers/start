package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

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

func (s *Service) SendMail(ctx context.Context, in SendMailInput) error {
	// Check if mailer is disabled
	if _, ok := s.mailer.(mailer.DisabledSender); ok {
		return ErrDisabledMailer
	}

	to := strings.TrimSpace(in.To)
	subject := strings.TrimSpace(in.Subject)
	body := strings.TrimSpace(in.Body)

	if to == "" || subject == "" || body == "" {
		return ErrInvalidMailInput
	}

	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("%w: invalid recipient", ErrInvalidMailInput)
	}

	// Convert service attachments to mailer attachments
	attachments := make([]mailer.Attachment, len(in.Attachments))
	for i, att := range in.Attachments {
		attachments[i] = mailer.Attachment{
			Filename: att.Filename,
			Data:     att.Data,
		}
	}

	// Queue the mail task (non-blocking) instead of sending directly
	task := mailTask{
		msg: mailer.Message{
			To:          to,
			Subject:     subject,
			Body:        body,
			Attachments: attachments,
		},
	}

	select {
	case s.mailQueue <- task:
		// Task queued successfully
		return nil
	case <-ctx.Done():
		// Request context cancelled while queuing
		return ctx.Err()
	default:
		// Queue is full; reject the request to apply backpressure
		return ErrMailQueueFull
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

// Close gracefully shuts down the mail worker, processing any queued emails.
func (s *Service) Close() {
	close(s.done)
	close(s.mailQueue)
}
