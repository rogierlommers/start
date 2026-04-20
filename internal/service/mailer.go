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

type SendMailInput struct {
	To      string
	Subject string
	Body    string
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

	return s.mailer.Send(ctx, mailer.Message{
		To:      to,
		Subject: subject,
		Body:    body,
	})
}
