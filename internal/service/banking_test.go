package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"start/internal/banking"
	"start/internal/config"
	"start/internal/mailer"
	"start/internal/repository"
)

type fakeBankingClient struct {
	mu                   sync.Mutex
	authorizationRequest banking.AuthorizationRequest
	session              banking.Session
	balances             map[string][]banking.Balance
	balanceErrors        map[string]error
	accountDetails       map[string]banking.Account
	balanceCalls         int
}

func (f *fakeBankingClient) StartAuthorization(_ context.Context, request banking.AuthorizationRequest) (string, error) {
	f.authorizationRequest = request
	return "https://auth.example/start", nil
}

func (f *fakeBankingClient) AuthorizeSession(_ context.Context, _ string) (banking.Session, error) {
	return f.session, nil
}

func (f *fakeBankingClient) GetSession(_ context.Context, _ string) (banking.Session, error) {
	return f.session, nil
}

func (f *fakeBankingClient) GetAccountDetails(_ context.Context, accountID string) (banking.Account, error) {
	account, found := f.accountDetails[accountID]
	if !found {
		return banking.Account{}, errors.New("account details unavailable")
	}
	return account, nil
}

func (f *fakeBankingClient) GetBalances(_ context.Context, accountID string) ([]banking.Balance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.balanceCalls++
	return f.balances[accountID], f.balanceErrors[accountID]
}

func TestBankingRecoversAllAccountsFromExistingSession(t *testing.T) {
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	store := repository.NewMemoryStore()
	if err := store.ReplaceBankConnections(context.Background(), []repository.BankConnection{{
		SessionID: "session-1", AccountID: "account-1", AccountName: "Current account",
		Currency: "EUR", ValidUntil: now.AddDate(0, 0, 180), UpdatedAt: now,
	}}); err != nil {
		t.Fatalf("ReplaceBankConnections() error = %v", err)
	}
	client := &fakeBankingClient{
		session: banking.Session{
			ID: "session-1", ValidUntil: now.AddDate(0, 0, 180),
			Accounts: []banking.Account{{ID: "account-1"}, {ID: "account-2"}},
		},
		accountDetails: map[string]banking.Account{
			"account-1": {ID: "account-1", Name: "Current account", Currency: "EUR"},
			"account-2": {ID: "account-2", Name: "Savings account", Currency: "EUR"},
		},
		balances: map[string][]banking.Balance{
			"account-1": {{Type: "CLAV", Amount: "100.00", Currency: "EUR"}},
			"account-2": {{Type: "CLAV", Amount: "250.00", Currency: "EUR"}},
		},
		balanceErrors: make(map[string]error),
	}
	svc := NewWithOptions(store, mailer.DisabledSender{}, bankingConfig(), Options{BankingClient: client})
	t.Cleanup(svc.Close)

	overview, err := svc.GetBankBalances(context.Background(), now, false)
	if err != nil || len(overview.Accounts) != 2 {
		t.Fatalf("GetBankBalances() = (%+v, %v), want two accounts", overview, err)
	}
	connections, err := store.ListBankConnections(context.Background())
	if err != nil || len(connections) != 2 || connections[1].AccountName != "Savings account" {
		t.Fatalf("persisted connections = (%+v, %v)", connections, err)
	}
}

func TestBankingConnectionAndCachedBalance(t *testing.T) {
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	client := &fakeBankingClient{
		session: banking.Session{
			ID: "session-1", ValidUntil: now.AddDate(0, 0, 180),
			Accounts: []banking.Account{
				{ID: "account-1", Name: "Oranje account", Currency: "EUR"},
				{ID: "account-2", Product: "Savings account", Currency: "EUR"},
			},
		},
		balances: map[string][]banking.Balance{
			"account-1": {
				{Type: "CLBD", Name: "Booked", Amount: "120.00", Currency: "EUR"},
				{Type: "CLAV", Name: "Available", Amount: "100.00", Currency: "EUR"},
			},
			"account-2": {{Type: "CLAV", Name: "Available", Amount: "250.00", Currency: "EUR"}},
		},
		balanceErrors: make(map[string]error),
	}
	store := repository.NewMemoryStore()
	svc := NewWithOptions(store, mailer.DisabledSender{}, bankingConfig(), Options{BankingClient: client})
	t.Cleanup(svc.Close)

	redirectURL, err := svc.StartBankAuthorization(context.Background(), now)
	if err != nil || redirectURL != "https://auth.example/start" {
		t.Fatalf("StartBankAuthorization() = (%q, %v)", redirectURL, err)
	}
	if client.authorizationRequest.Country != "NL" || client.authorizationRequest.ASPSPName != "ING" {
		t.Fatalf("authorization request = %+v", client.authorizationRequest)
	}
	if err := svc.CompleteBankAuthorization(context.Background(), client.authorizationRequest.State, "code", now); err != nil {
		t.Fatalf("CompleteBankAuthorization() error = %v", err)
	}

	overview, err := svc.GetBankBalances(context.Background(), now, false)
	if err != nil || len(overview.Accounts) != 2 || overview.Accounts[0].Amount != "100.00" || overview.Accounts[1].Amount != "250.00" {
		t.Fatalf("GetBankBalances() = (%+v, %v)", overview, err)
	}
	_, err = svc.GetBankBalances(context.Background(), now.Add(time.Minute), false)
	if err != nil || client.balanceCalls != 2 {
		t.Fatalf("cached GetBankBalances() error = %v, calls = %d", err, client.balanceCalls)
	}

	client.balanceErrors["account-2"] = errors.New("provider down")
	stale, err := svc.GetBankBalances(context.Background(), now.Add(10*time.Minute), true)
	if err != nil || stale.Accounts[0].Stale || !stale.Accounts[1].Stale || stale.Accounts[1].Amount != "250.00" {
		t.Fatalf("stale GetBankBalances() = (%+v, %v)", stale, err)
	}
}

func TestBankingRejectsReusedState(t *testing.T) {
	now := time.Now().UTC()
	client := &fakeBankingClient{session: banking.Session{
		ID: "session", ValidUntil: now.Add(time.Hour), Accounts: []banking.Account{{ID: "account"}},
	}}
	svc := NewWithOptions(repository.NewMemoryStore(), mailer.DisabledSender{}, bankingConfig(), Options{BankingClient: client})
	t.Cleanup(svc.Close)
	_, _ = svc.StartBankAuthorization(context.Background(), now)
	state := client.authorizationRequest.State
	if err := svc.CompleteBankAuthorization(context.Background(), state, "code", now); err != nil {
		t.Fatalf("first CompleteBankAuthorization() error = %v", err)
	}
	if err := svc.CompleteBankAuthorization(context.Background(), state, "code", now); !errors.Is(err, ErrBankingStateInvalid) {
		t.Fatalf("reused state error = %v, want %v", err, ErrBankingStateInvalid)
	}
}

func bankingConfig() config.Config {
	return config.Config{
		EnableBankingAppID: "app", EnableBankingPrivateKey: "/private.pem",
		EnableBankingCallbackURL: "https://dashboard.example/api/banking/callback",
		EnableBankingASPSPName:   "ING",
	}
}
