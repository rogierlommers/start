package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"start/internal/banking"
	"start/internal/repository"
)

const (
	bankingStateLifetime = 15 * time.Minute
	bankingCacheLifetime = 5 * time.Minute
	bankingConsentDays   = 180
)

var (
	ErrBankingDisabled     = errors.New("banking integration is not configured")
	ErrBankingStateInvalid = errors.New("banking authorization state is invalid or expired")
	ErrBankingUnavailable  = errors.New("banking provider is unavailable")
	bankAmountPattern      = regexp.MustCompile(`^-?[0-9]{1,18}(\.[0-9]{1,8})?$`)
	bankCurrencyPattern    = regexp.MustCompile(`^[A-Z]{3}$`)
)

type BankBalanceOverview struct {
	Status       string
	AccountName  string
	Amount       string
	Currency     string
	BalanceType  string
	BalanceName  string
	ProviderTime time.Time
	FetchedAt    time.Time
	ValidUntil   time.Time
	Stale        bool
}

type BankBalancesOverview struct {
	Status   string
	Accounts []BankBalanceOverview
}

func (s *Service) StartBankAuthorization(ctx context.Context, now time.Time) (string, error) {
	if !s.cfg.EnableBankingEnabled() || s.bankingClient == nil {
		return "", ErrBankingDisabled
	}
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("create banking authorization state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	s.bankingMu.Lock()
	for candidate, expiresAt := range s.bankingStates {
		if !expiresAt.After(now) {
			delete(s.bankingStates, candidate)
		}
	}
	s.bankingStates[state] = now.Add(bankingStateLifetime)
	s.bankingMu.Unlock()

	redirectURL, err := s.bankingClient.StartAuthorization(ctx, banking.AuthorizationRequest{
		State:       state,
		RedirectURL: s.cfg.EnableBankingCallbackURL,
		ASPSPName:   s.cfg.EnableBankingASPSPName,
		Country:     "NL",
		ValidUntil:  now.AddDate(0, 0, bankingConsentDays),
	})
	if err != nil {
		s.bankingMu.Lock()
		delete(s.bankingStates, state)
		s.bankingMu.Unlock()
		return "", fmt.Errorf("start bank authorization: %w", ErrBankingUnavailable)
	}
	return redirectURL, nil
}

func (s *Service) CompleteBankAuthorization(ctx context.Context, state, code string, now time.Time) error {
	if !s.cfg.EnableBankingEnabled() || s.bankingClient == nil {
		return ErrBankingDisabled
	}
	if len(code) == 0 || len(code) > 2048 {
		return ErrBankingStateInvalid
	}

	s.bankingMu.Lock()
	expiresAt, found := s.bankingStates[state]
	delete(s.bankingStates, state)
	s.bankingMu.Unlock()
	if !found || !expiresAt.After(now) {
		return ErrBankingStateInvalid
	}

	session, err := s.bankingClient.AuthorizeSession(ctx, code)
	if err != nil {
		return fmt.Errorf("complete bank authorization: %w", ErrBankingUnavailable)
	}
	connections := make([]repository.BankConnection, 0, len(session.Accounts))
	for _, account := range session.Accounts {
		connections = append(connections, repository.BankConnection{
			SessionID: session.ID, AccountID: account.ID, AccountName: bankAccountName(account),
			Currency: account.Currency, ValidUntil: session.ValidUntil, UpdatedAt: now.UTC(),
		})
	}
	s.bankingSyncMu.Lock()
	defer s.bankingSyncMu.Unlock()
	if err := s.store.ReplaceBankConnections(ctx, connections); err != nil {
		return fmt.Errorf("save bank connections: %w", err)
	}

	s.bankingMu.Lock()
	clear(s.bankingCache)
	s.bankingAccountsSynced = true
	s.bankingMu.Unlock()
	return nil
}

func bankAccountName(account banking.Account) string {
	if name := strings.TrimSpace(account.Name); name != "" {
		return name
	}
	if product := strings.TrimSpace(account.Product); product != "" {
		return product
	}
	return "ING account"
}

func (s *Service) GetBankBalances(ctx context.Context, now time.Time, forceRefresh bool) (BankBalancesOverview, error) {
	if !s.cfg.EnableBankingEnabled() || s.bankingClient == nil {
		return BankBalancesOverview{Status: "disabled", Accounts: []BankBalanceOverview{}}, nil
	}
	connections, err := s.store.ListBankConnections(ctx)
	if err != nil {
		return BankBalancesOverview{}, fmt.Errorf("load bank connections: %w", err)
	}
	if len(connections) == 0 {
		return BankBalancesOverview{Status: "disconnected", Accounts: []BankBalanceOverview{}}, nil
	}
	connections = s.syncBankConnections(ctx, connections, now)

	s.bankingMu.Lock()
	cached := make(map[string]BankBalanceOverview, len(s.bankingCache))
	for accountID, overview := range s.bankingCache {
		cached[accountID] = overview
	}
	s.bankingMu.Unlock()

	results := make([]BankBalanceOverview, len(connections))
	type fetchJob struct {
		index      int
		connection repository.BankConnection
		cached     *BankBalanceOverview
	}
	jobs := make(chan fetchJob)
	var workers sync.WaitGroup
	workerCount := min(4, len(connections))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				results[job.index] = s.fetchBankBalance(ctx, now, job.connection, job.cached)
			}
		}()
	}

	for index, connection := range connections {
		if !connection.ValidUntil.After(now) {
			results[index] = BankBalanceOverview{
				Status: "reauthorization_required", AccountName: connection.AccountName,
				Currency: connection.Currency, ValidUntil: connection.ValidUntil,
			}
			continue
		}
		cachedOverview, hasCache := cached[connection.AccountID]
		if !forceRefresh && hasCache && cachedOverview.FetchedAt.Add(bankingCacheLifetime).After(now) {
			results[index] = cachedOverview
			continue
		}
		var cachedPointer *BankBalanceOverview
		if hasCache {
			copy := cachedOverview
			cachedPointer = &copy
		}
		jobs <- fetchJob{index: index, connection: connection, cached: cachedPointer}
	}
	close(jobs)
	workers.Wait()

	s.bankingMu.Lock()
	for index, connection := range connections {
		if results[index].Status == "connected" && !results[index].Stale {
			s.bankingCache[connection.AccountID] = results[index]
		}
	}
	s.bankingMu.Unlock()

	status := "connected"
	allExpired := true
	for _, result := range results {
		if result.Status != "reauthorization_required" {
			allExpired = false
			break
		}
	}
	if allExpired {
		status = "reauthorization_required"
	}
	return BankBalancesOverview{Status: status, Accounts: results}, nil
}

func (s *Service) syncBankConnections(ctx context.Context, existing []repository.BankConnection, now time.Time) []repository.BankConnection {
	s.bankingSyncMu.Lock()
	defer s.bankingSyncMu.Unlock()
	s.bankingMu.Lock()
	if s.bankingAccountsSynced {
		s.bankingMu.Unlock()
		return existing
	}
	s.bankingMu.Unlock()

	session, err := s.bankingClient.GetSession(ctx, existing[0].SessionID)
	if err != nil {
		return existing
	}
	existingByID := make(map[string]repository.BankConnection, len(existing))
	for _, connection := range existing {
		existingByID[connection.AccountID] = connection
	}

	connections := make([]repository.BankConnection, 0, len(session.Accounts))
	for _, sessionAccount := range session.Accounts {
		account, detailsErr := s.bankingClient.GetAccountDetails(ctx, sessionAccount.ID)
		if detailsErr != nil {
			if persisted, found := existingByID[sessionAccount.ID]; found {
				connections = append(connections, persisted)
				continue
			}
			account = sessionAccount
		}
		connections = append(connections, repository.BankConnection{
			SessionID: session.ID, AccountID: account.ID, AccountName: bankAccountName(account),
			Currency: account.Currency, ValidUntil: session.ValidUntil, UpdatedAt: now.UTC(),
		})
	}
	if len(connections) == 0 {
		return existing
	}
	if err := s.store.ReplaceBankConnections(ctx, connections); err != nil {
		return existing
	}
	s.bankingMu.Lock()
	s.bankingAccountsSynced = true
	s.bankingMu.Unlock()
	return connections
}

func (s *Service) fetchBankBalance(ctx context.Context, now time.Time, connection repository.BankConnection, cached *BankBalanceOverview) BankBalanceOverview {
	balances, err := s.bankingClient.GetBalances(ctx, connection.AccountID)
	if err != nil {
		return unavailableBankBalance(connection, cached)
	}
	selected, err := selectBalance(balances)
	if err != nil {
		return unavailableBankBalance(connection, cached)
	}
	return BankBalanceOverview{
		Status: "connected", AccountName: connection.AccountName,
		Amount: selected.Amount, Currency: selected.Currency, BalanceType: selected.Type,
		BalanceName: selected.Name, ProviderTime: selected.LastChange,
		FetchedAt: now.UTC(), ValidUntil: connection.ValidUntil,
	}
}

func unavailableBankBalance(connection repository.BankConnection, cached *BankBalanceOverview) BankBalanceOverview {
	if cached != nil {
		stale := *cached
		stale.Stale = true
		return stale
	}
	return BankBalanceOverview{
		Status: "unavailable", AccountName: connection.AccountName,
		Currency: connection.Currency, ValidUntil: connection.ValidUntil,
	}
}

func selectBalance(balances []banking.Balance) (banking.Balance, error) {
	priorities := []string{"CLAV", "ITAV", "CLBD", "ITBD"}
	for _, desiredType := range priorities {
		for _, balance := range balances {
			if balance.Type == desiredType && validBalance(balance) {
				return balance, nil
			}
		}
	}
	for _, balance := range balances {
		if validBalance(balance) {
			return balance, nil
		}
	}
	return banking.Balance{}, errors.New("banking provider returned no usable balance")
}

func validBalance(balance banking.Balance) bool {
	return bankAmountPattern.MatchString(balance.Amount) && bankCurrencyPattern.MatchString(balance.Currency)
}
