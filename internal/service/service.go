package service

import (
	"start/internal/mailer"
	"start/internal/repository"
)

// Service contains application use-cases.
type Service struct {
	store  repository.Store
	mailer mailer.Sender
}

func New(store repository.Store, sender mailer.Sender) *Service {
	if sender == nil {
		sender = mailer.DisabledSender{}
	}

	return &Service{store: store, mailer: sender}
}
