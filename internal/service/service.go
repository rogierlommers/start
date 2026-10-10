package service

import (
	"net/http"
	"sync"
	"time"

	"start/internal/banking"
	"start/internal/config"
	"start/internal/mailer"
	"start/internal/repository"

	"github.com/sirupsen/logrus"
)

// Service contains application use-cases.
type Service struct {
	store                   repository.Store
	mailer                  mailer.Sender
	mailQueue               chan mailTask
	done                    chan struct{}
	cfg                     config.Config
	httpClient              *http.Client
	bankingClient           banking.Client
	bankingMu               sync.Mutex
	bankingSyncMu           sync.Mutex
	bankingStates           map[string]time.Time
	bankingCache            map[string]BankBalanceOverview
	bankingTransactionCache map[string]BankAccountTransactions
	bankingAccountsSynced   bool
}

type Options struct {
	BankingClient banking.Client
}

type mailTask struct {
	msg mailer.Message
}

func New(store repository.Store, sender mailer.Sender, cfg config.Config) *Service {
	return NewWithOptions(store, sender, cfg, Options{})
}

func NewWithOptions(store repository.Store, sender mailer.Sender, cfg config.Config, options Options) *Service {
	if sender == nil {
		sender = mailer.DisabledSender{}
	}

	return &Service{
		store:                   store,
		mailer:                  sender,
		mailQueue:               make(chan mailTask, 100), // buffered queue for up to 100 pending emails
		done:                    make(chan struct{}),
		cfg:                     cfg,
		httpClient:              &http.Client{Timeout: 15 * time.Second},
		bankingClient:           options.BankingClient,
		bankingStates:           make(map[string]time.Time),
		bankingCache:            make(map[string]BankBalanceOverview),
		bankingTransactionCache: make(map[string]BankAccountTransactions),
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
