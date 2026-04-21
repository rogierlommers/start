package service

import (
	"start/internal/mailer"
	"start/internal/repository"
)

// Service contains application use-cases.
type Service struct {
	store     repository.Store
	mailer    mailer.Sender
	mailQueue chan mailTask
	done      chan struct{}
}

type mailTask struct {
	msg mailer.Message
}

func New(store repository.Store, sender mailer.Sender) *Service {
	if sender == nil {
		sender = mailer.DisabledSender{}
	}

	return &Service{
		store:     store,
		mailer:    sender,
		mailQueue: make(chan mailTask, 100), // buffered queue for up to 100 pending emails
		done:      make(chan struct{}),
	}
}
