package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"start/internal/mailer"
)

var ErrInvalidMailInput = errors.New("invalid mail input")

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

	return s.mailer.Send(ctx, mailer.Message{
		To:          to,
		Subject:     subject,
		Body:        body,
		Attachments: attachments,
	})
}
