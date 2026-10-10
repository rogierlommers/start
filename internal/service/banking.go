package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
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
	account := session.Accounts[0]
	accountName := strings.TrimSpace(account.Name)
	if accountName == "" {
		accountName = strings.TrimSpace(account.Product)
	}
	if accountName == "" {
		accountName = "ING account"
	}
	if err := s.store.SaveBankConnection(ctx, repository.BankConnection{
		SessionID: session.ID, AccountID: account.ID, AccountName: accountName,
		Currency: account.Currency, ValidUntil: session.ValidUntil, UpdatedAt: now.UTC(),
	}); err != nil {
		return fmt.Errorf("save bank connection: %w", err)
	}

	s.bankingMu.Lock()
	s.bankingCache = nil
	s.bankingMu.Unlock()
	return nil
}

func (s *Service) GetBankBalance(ctx context.Context, now time.Time, forceRefresh bool) (BankBalanceOverview, error) {
	if !s.cfg.EnableBankingEnabled() || s.bankingClient == nil {
		return BankBalanceOverview{Status: "disabled"}, nil
	}
	connection, err := s.store.GetBankConnection(ctx)
	if errors.Is(err, repository.ErrBankConnectionNotFound) {
		return BankBalanceOverview{Status: "disconnected"}, nil
	}
	if err != nil {
		return BankBalanceOverview{}, fmt.Errorf("load bank connection: %w", err)
	}
	if !connection.ValidUntil.After(now) {
		return BankBalanceOverview{
			Status: "reauthorization_required", AccountName: connection.AccountName,
			Currency: connection.Currency, ValidUntil: connection.ValidUntil,
		}, nil
	}

	s.bankingMu.Lock()
	defer s.bankingMu.Unlock()
	if !forceRefresh && s.bankingCache != nil && s.bankingCache.FetchedAt.Add(bankingCacheLifetime).After(now) {
		return *s.bankingCache, nil
	}

	balances, err := s.bankingClient.GetBalances(ctx, connection.AccountID)
	if err != nil {
		if s.bankingCache != nil {
			stale := *s.bankingCache
			stale.Stale = true
			return stale, nil
		}
		return BankBalanceOverview{}, fmt.Errorf("refresh bank balance: %w", ErrBankingUnavailable)
	}
	selected, err := selectBalance(balances)
	if err != nil {
		return BankBalanceOverview{}, err
	}
	overview := BankBalanceOverview{
		Status: "connected", AccountName: connection.AccountName,
		Amount: selected.Amount, Currency: selected.Currency, BalanceType: selected.Type,
		BalanceName: selected.Name, ProviderTime: selected.LastChange,
		FetchedAt: now.UTC(), ValidUntil: connection.ValidUntil,
	}
	s.bankingCache = &overview
	return overview, nil
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
