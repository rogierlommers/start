package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"start/internal/mailer"
	"start/internal/repository"
)

var ErrInvalidMailInput = errors.New("invalid mail input")

// Service contains application use-cases.
type Service struct {
	store  repository.Store
	mailer mailer.Sender
}

type SendMailInput struct {
	To      string
	Subject string
	Body    string
}

func New(store repository.Store, sender mailer.Sender) *Service {
	if sender == nil {
		sender = mailer.DisabledSender{}
	}

	return &Service{store: store, mailer: sender}
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
