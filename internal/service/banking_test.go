package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"start/internal/banking"
	"start/internal/config"
	"start/internal/mailer"
	"start/internal/repository"
)

type fakeBankingClient struct {
	authorizationRequest banking.AuthorizationRequest
	session              banking.Session
	balances             []banking.Balance
	balanceErr           error
	balanceCalls         int
}

func (f *fakeBankingClient) StartAuthorization(_ context.Context, request banking.AuthorizationRequest) (string, error) {
	f.authorizationRequest = request
	return "https://auth.example/start", nil
}

func (f *fakeBankingClient) AuthorizeSession(_ context.Context, _ string) (banking.Session, error) {
	return f.session, nil
}

func (f *fakeBankingClient) GetBalances(_ context.Context, _ string) ([]banking.Balance, error) {
	f.balanceCalls++
	return f.balances, f.balanceErr
}

func TestBankingConnectionAndCachedBalance(t *testing.T) {
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	client := &fakeBankingClient{
		session: banking.Session{
			ID: "session-1", ValidUntil: now.AddDate(0, 0, 180),
			Accounts: []banking.Account{{ID: "account-1", Name: "Oranje account", Currency: "EUR"}},
		},
		balances: []banking.Balance{
			{Type: "CLBD", Name: "Booked", Amount: "120.00", Currency: "EUR"},
			{Type: "CLAV", Name: "Available", Amount: "100.00", Currency: "EUR"},
		},
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

	overview, err := svc.GetBankBalance(context.Background(), now, false)
	if err != nil || overview.Amount != "100.00" || overview.BalanceType != "CLAV" {
		t.Fatalf("GetBankBalance() = (%+v, %v)", overview, err)
	}
	_, err = svc.GetBankBalance(context.Background(), now.Add(time.Minute), false)
	if err != nil || client.balanceCalls != 1 {
		t.Fatalf("cached GetBankBalance() error = %v, calls = %d", err, client.balanceCalls)
	}

	client.balanceErr = errors.New("provider down")
	stale, err := svc.GetBankBalance(context.Background(), now.Add(10*time.Minute), true)
	if err != nil || !stale.Stale || stale.Amount != "100.00" {
		t.Fatalf("stale GetBankBalance() = (%+v, %v)", stale, err)
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
