package service

import (
	"context"
	"errors"
	"strings"
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
	transactions         map[string][]banking.Transaction
	transactionErrors    map[string]error
	balanceCalls         int
	transactionCalls     int
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

func (f *fakeBankingClient) GetTransactions(_ context.Context, accountID string, _, _ time.Time) ([]banking.Transaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transactionCalls++
	return f.transactions[accountID], f.transactionErrors[accountID]
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
			Accounts: []banking.Account{{ID: "account-1", IdentificationHash: "stable-1"}, {ID: "account-2", IdentificationHash: "stable-2"}},
		},
		accountDetails: map[string]banking.Account{
			"account-1": {ID: "account-1", IdentificationHash: "stable-1", Name: "Current account", Currency: "EUR"},
			"account-2": {ID: "account-2", IdentificationHash: "stable-2", Name: "Savings account", Currency: "EUR"},
		},
		balances: map[string][]banking.Balance{
			"account-1": {{Type: "CLAV", Amount: "100.00", Currency: "EUR"}},
			"account-2": {{Type: "CLAV", Amount: "250.00", Currency: "EUR"}},
		},
		balanceErrors:     make(map[string]error),
		transactionErrors: make(map[string]error),
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
	transactions, err := svc.GetBankTransactions(context.Background(), now, 5, false)
	if err != nil || transactions.Status != "permission_required" || client.transactionCalls != 0 {
		t.Fatalf("transactions before renewed consent = (%+v, %v), calls = %d", transactions, err, client.transactionCalls)
	}
}

func TestBankingConnectionAndCachedBalance(t *testing.T) {
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	client := &fakeBankingClient{
		session: banking.Session{
			ID: "session-1", ValidUntil: now.AddDate(0, 0, 180),
			Accounts: []banking.Account{
				{ID: "account-1", IdentificationHash: "stable-1", Name: "Oranje account", Currency: "EUR"},
				{ID: "account-2", IdentificationHash: "stable-2", Product: "Savings account", Currency: "EUR"},
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
		transactions: map[string][]banking.Transaction{
			"account-1": {
				{Amount: "10.00", Currency: "EUR", Direction: "DBIT", BookingDate: "2026-10-08", CreditorName: "Shop", RemittanceInformation: []string{"Groceries"}},
				{Amount: "2500.00", Currency: "EUR", Direction: "CRDT", BookingDate: "2026-10-09", DebtorName: "Employer", RemittanceInformation: []string{"Salary"}},
			},
			"account-2": {},
		},
		transactionErrors: make(map[string]error),
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
	connections, err := store.ListBankConnections(context.Background())
	if err != nil || len(connections) != 2 || !connections[0].TransactionsEnabled || !connections[1].TransactionsEnabled {
		t.Fatalf("connections after transaction consent = (%+v, %v)", connections, err)
	}

	overview, err := svc.GetBankBalances(context.Background(), now, false)
	if err != nil || len(overview.Accounts) != 2 || overview.Accounts[0].Amount != "100.00" || overview.Accounts[1].Amount != "250.00" {
		t.Fatalf("GetBankBalances() = (%+v, %v)", overview, err)
	}
	if _, err := svc.SetBankAccountAlias(context.Background(), overview.Accounts[0].AccountKey, "Household", now); err != nil {
		t.Fatalf("SetBankAccountAlias() error = %v", err)
	}
	overview, err = svc.GetBankBalances(context.Background(), now.Add(time.Second), false)
	if err != nil || overview.Accounts[0].AccountName != "Household" || overview.Accounts[0].ProviderAccountName != "Oranje account" || overview.Accounts[0].Alias != "Household" {
		t.Fatalf("aliased GetBankBalances() = (%+v, %v)", overview, err)
	}
	if _, err := svc.SetBankAccountAlias(context.Background(), overview.Accounts[0].AccountKey, strings.Repeat("x", 81), now); !errors.Is(err, ErrInvalidBankAlias) {
		t.Fatalf("long alias error = %v, want %v", err, ErrInvalidBankAlias)
	}
	if _, err := svc.SetBankAccountAlias(context.Background(), strings.Repeat("0", 64), "Unknown", now); !errors.Is(err, ErrBankAccountNotFound) {
		t.Fatalf("unknown account error = %v, want %v", err, ErrBankAccountNotFound)
	}
	_, err = svc.GetBankBalances(context.Background(), now.Add(30*time.Minute), false)
	if err != nil || client.balanceCalls != 2 {
		t.Fatalf("cached GetBankBalances() error = %v, calls = %d", err, client.balanceCalls)
	}
	_, err = svc.GetBankBalances(context.Background(), now.Add(61*time.Minute), false)
	if err != nil || client.balanceCalls != 4 {
		t.Fatalf("expired-cache GetBankBalances() error = %v, calls = %d", err, client.balanceCalls)
	}

	client.balanceErrors["account-2"] = errors.New("provider down")
	stale, err := svc.GetBankBalances(context.Background(), now.Add(70*time.Minute), true)
	if err != nil || stale.Accounts[0].Stale || !stale.Accounts[1].Stale || stale.Accounts[1].Amount != "250.00" {
		t.Fatalf("stale GetBankBalances() = (%+v, %v)", stale, err)
	}
	transactionOverview, err := svc.GetBankTransactions(context.Background(), now, 1, false)
	if err != nil || len(transactionOverview.Accounts) != 2 || len(transactionOverview.Accounts[0].Transactions) != 1 || transactionOverview.Accounts[0].AccountName != "Household" {
		t.Fatalf("GetBankTransactions() = (%+v, %v)", transactionOverview, err)
	}
	transaction := transactionOverview.Accounts[0].Transactions[0]
	if transaction.Counterparty != "Employer" || transaction.Description != "Salary" || transaction.Direction != "CRDT" {
		t.Fatalf("latest transaction = %+v", transaction)
	}
	_, err = svc.GetBankTransactions(context.Background(), now.Add(30*time.Minute), 5, false)
	if err != nil || client.transactionCalls != 2 {
		t.Fatalf("cached GetBankTransactions() error = %v, calls = %d", err, client.transactionCalls)
	}
	_, err = svc.GetBankTransactions(context.Background(), now.Add(61*time.Minute), 5, false)
	if err != nil || client.transactionCalls != 4 {
		t.Fatalf("expired-cache GetBankTransactions() error = %v, calls = %d", err, client.transactionCalls)
	}
	client.transactionErrors["account-1"] = errors.New("provider down")
	staleTransactions, err := svc.GetBankTransactions(context.Background(), now.Add(70*time.Minute), 5, true)
	if err != nil || !staleTransactions.Accounts[0].Stale || len(staleTransactions.Accounts[0].Transactions) != 2 || staleTransactions.Accounts[1].Stale {
		t.Fatalf("stale GetBankTransactions() = (%+v, %v)", staleTransactions, err)
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

func TestRefreshBankingCachesRefreshesBalancesAndTransactions(t *testing.T) {
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	store := repository.NewMemoryStore()
	if err := store.ReplaceBankConnections(context.Background(), []repository.BankConnection{{
		SessionID: "session-1", AccountID: "account-1", AccountName: "Current account",
		Currency: "EUR", ValidUntil: now.AddDate(0, 0, 180), UpdatedAt: now,
		TransactionsEnabled: true, IdentificationHash: "stable-1",
	}}); err != nil {
		t.Fatalf("ReplaceBankConnections() error = %v", err)
	}
	client := &fakeBankingClient{
		balances: map[string][]banking.Balance{
			"account-1": {{Type: "CLAV", Amount: "100.00", Currency: "EUR"}},
		},
		balanceErrors: map[string]error{},
		transactions: map[string][]banking.Transaction{
			"account-1": {{Amount: "10.00", Currency: "EUR", Direction: "DBIT", BookingDate: "2026-10-09"}},
		},
		transactionErrors: map[string]error{},
	}
	svc := NewWithOptions(store, mailer.DisabledSender{}, bankingConfig(), Options{BankingClient: client})
	t.Cleanup(svc.Close)
	svc.bankingAccountsSynced = true

	svc.refreshBankingCaches(context.Background(), now)

	svc.bankingMu.Lock()
	defer svc.bankingMu.Unlock()
	if len(svc.bankingCache) != 1 || len(svc.bankingTransactionCache) != 1 {
		t.Fatalf("cache sizes = (%d balances, %d transactions), want (1, 1)", len(svc.bankingCache), len(svc.bankingTransactionCache))
	}
	if client.balanceCalls != 1 || client.transactionCalls != 1 {
		t.Fatalf("provider calls = (%d balances, %d transactions), want (1, 1)", client.balanceCalls, client.transactionCalls)
	}
}

func bankingConfig() config.Config {
	return config.Config{
		EnableBankingAppID: "app", EnableBankingPrivateKey: "/private.pem",
		EnableBankingCallbackURL: "https://dashboard.example/api/banking/callback",
		EnableBankingASPSPName:   "ING",
		EnableBankingCacheTTL:    60 * time.Minute,
	}
}
