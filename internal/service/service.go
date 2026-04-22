package service

import (
	"start/internal/config"
	"start/internal/mailer"
	"start/internal/repository"

	"github.com/sirupsen/logrus"
)

// Service contains application use-cases.
type Service struct {
	store     repository.Store
	mailer    mailer.Sender
	mailQueue chan mailTask
	done      chan struct{}
	cfg       config.Config
}

type mailTask struct {
	msg mailer.Message
}

func New(store repository.Store, sender mailer.Sender, cfg config.Config) *Service {
	if sender == nil {
		sender = mailer.DisabledSender{}
	}

	return &Service{
		store:     store,
		mailer:    sender,
		mailQueue: make(chan mailTask, 100), // buffered queue for up to 100 pending emails
		done:      make(chan struct{}),
		cfg:       cfg,
	}
}

// Close gracefully shuts down all sevices, including background workers and the data store.
func (s *Service) Close() {
	close(s.done)
	close(s.mailQueue)

	if closer, ok := s.store.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			logrus.Warnf("failed to close store: %v", err)
		}
	}
}
