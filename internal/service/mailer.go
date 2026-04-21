package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"start/internal/config"
	"start/internal/mailer"

	"github.com/sirupsen/logrus"
)

var (
	ErrInvalidMailInput = errors.New("invalid mail input")
	ErrMailQueueFull    = errors.New("mail queue is full")
	ErrDisabledMailer   = errors.New("mailer is not configured")
)

const generatedSubjectMaxLen = 80

type SendMailAttachment struct {
	Filename string
	Data     []byte
}

type SendMailInput struct {
	Body        string
	Attachments []SendMailAttachment
}

func (s *Service) SendMail(ctx context.Context, in SendMailInput) error {
	// Check if mailer is disabled
	if _, ok := s.mailer.(mailer.DisabledSender); ok {
		return ErrDisabledMailer
	}

	// determine to address
	to, body, subject := deterimeRecipientBodyAndSubject(s.cfg, in.Body)

	if to == "" || subject == "" || body == "" {
		logrus.Error("invalid mail input: missing required fields")
		return ErrInvalidMailInput
	}

	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("%w: invalid recipient", ErrInvalidMailInput)
	}

	// Convert service attachments to mailer attachments
	attachments := make([]mailer.Attachment, len(in.Attachments))
	for i, att := range in.Attachments {
		// skip attachment if filename equals body text (common when using Apple Shortcuts to
		// send mail with a single attachment, where the filename is used as the body text and
		// the actual attachment data is empty)
		if att.Filename == body+".txt" {
			continue
		}

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

// deterimeRecipientBodyAndSubject checks if the body starts with a "w " or "W ".
// if so, then use the work email, otherwise use the private email.
// also, if work email, then strip the leading "w" or "W" from the body to avoid confusion in the email content.
// as the subject, use the first line of the body, truncated by 80 characters
func deterimeRecipientBodyAndSubject(cfg config.Config, body string) (string, string, string) {
	trimmedLeft := strings.TrimLeft(body, " \t\r\n")
	recipient := cfg.MailerEmailPrivate

	if strings.HasPrefix(trimmedLeft, "w ") || strings.HasPrefix(trimmedLeft, "W ") {
		recipient = cfg.MailerEmailWork
		trimmedLeft = trimmedLeft[2:]
	}

	cleanBody := strings.TrimSpace(trimmedLeft)
	subject := deriveSubject(cleanBody)

	return recipient, cleanBody, subject
}

func deriveSubject(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}

	parts := strings.SplitSeq(trimmed, "\n")
	for part := range parts {
		line := strings.TrimSpace(part)
		if line == "" {
			continue
		}

		runes := []rune(line)
		if len(runes) <= generatedSubjectMaxLen {
			return line
		}

		return strings.TrimSpace(string(runes[:generatedSubjectMaxLen]))
	}

	return ""
}
