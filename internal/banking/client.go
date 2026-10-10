package banking

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.enablebanking.com"
	maxResponseBytes  = 1 << 20
)

// Client provides the read-only Enable Banking operations used by the application.
type Client interface {
	StartAuthorization(ctx context.Context, request AuthorizationRequest) (string, error)
	AuthorizeSession(ctx context.Context, code string) (Session, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	GetAccountDetails(ctx context.Context, accountID string) (Account, error)
	GetBalances(ctx context.Context, accountID string) ([]Balance, error)
	GetTransactions(ctx context.Context, accountID string, dateFrom, dateTo time.Time) ([]Transaction, error)
}

type AuthorizationRequest struct {
	State       string
	RedirectURL string
	ASPSPName   string
	Country     string
	ValidUntil  time.Time
}

type Session struct {
	ID                  string
	Accounts            []Account
	ValidUntil          time.Time
	TransactionsEnabled bool
}

type Account struct {
	ID       string
	Name     string
	Product  string
	Currency string
}

type Balance struct {
	Name          string
	Type          string
	Amount        string
	Currency      string
	LastChange    time.Time
	ReferenceDate string
}

type Transaction struct {
	Amount                string
	Currency              string
	Direction             string
	Status                string
	BookingDate           string
	TransactionDate       string
	ValueDate             string
	CreditorName          string
	DebtorName            string
	RemittanceInformation []string
	Note                  string
}

// HTTPError reports a provider status without exposing a potentially sensitive response body.
type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Enable Banking API returned HTTP %d", e.StatusCode)
}

type EnableBankingClient struct {
	applicationID string
	privateKey    *rsa.PrivateKey
	httpClient    *http.Client
	baseURL       string
}

func NewEnableBankingClient(applicationID, privateKeyPath string, httpClient *http.Client) (*EnableBankingClient, error) {
	keyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read Enable Banking private key: %w", err)
	}
	privateKey, err := parseRSAPrivateKey(keyPEM)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &EnableBankingClient{
		applicationID: strings.TrimSpace(applicationID),
		privateKey:    privateKey,
		httpClient:    httpClient,
		baseURL:       defaultAPIBaseURL,
	}, nil
}

func (c *EnableBankingClient) StartAuthorization(ctx context.Context, input AuthorizationRequest) (string, error) {
	payload := struct {
		Access struct {
			Balances     bool   `json:"balances"`
			Transactions bool   `json:"transactions"`
			ValidUntil   string `json:"valid_until"`
		} `json:"access"`
		ASPSP struct {
			Name    string `json:"name"`
			Country string `json:"country"`
		} `json:"aspsp"`
		State       string `json:"state"`
		RedirectURL string `json:"redirect_url"`
		PSUType     string `json:"psu_type"`
	}{}
	payload.Access.Balances = true
	payload.Access.Transactions = true
	payload.Access.ValidUntil = input.ValidUntil.UTC().Format(time.RFC3339)
	payload.ASPSP.Name = input.ASPSPName
	payload.ASPSP.Country = input.Country
	payload.State = input.State
	payload.RedirectURL = input.RedirectURL
	payload.PSUType = "personal"

	var response struct {
		URL string `json:"url"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/auth", payload, &response); err != nil {
		return "", err
	}
	authorizationURL, err := url.Parse(response.URL)
	if err != nil || authorizationURL.Scheme != "https" || authorizationURL.Host == "" {
		return "", errors.New("Enable Banking returned an invalid authorization URL")
	}
	return response.URL, nil
}

func (c *EnableBankingClient) AuthorizeSession(ctx context.Context, code string) (Session, error) {
	var response struct {
		SessionID string `json:"session_id"`
		Accounts  []struct {
			UID      string `json:"uid"`
			Name     string `json:"name"`
			Product  string `json:"product"`
			Currency string `json:"currency"`
		} `json:"accounts"`
		Access struct {
			ValidUntil   string `json:"valid_until"`
			Transactions bool   `json:"transactions"`
		} `json:"access"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/sessions", map[string]string{"code": code}, &response); err != nil {
		return Session{}, err
	}
	validUntil, err := time.Parse(time.RFC3339Nano, response.Access.ValidUntil)
	if err != nil {
		return Session{}, errors.New("Enable Banking returned an invalid consent expiry")
	}
	session := Session{ID: response.SessionID, ValidUntil: validUntil, TransactionsEnabled: response.Access.Transactions}
	seenAccountIDs := make(map[string]struct{}, len(response.Accounts))
	for _, account := range response.Accounts {
		if account.UID == "" {
			return Session{}, errors.New("Enable Banking returned an account without an ID")
		}
		if _, exists := seenAccountIDs[account.UID]; exists {
			return Session{}, errors.New("Enable Banking returned a duplicate account ID")
		}
		seenAccountIDs[account.UID] = struct{}{}
		session.Accounts = append(session.Accounts, Account{
			ID: account.UID, Name: account.Name, Product: account.Product, Currency: account.Currency,
		})
	}
	if session.ID == "" || len(session.Accounts) == 0 {
		return Session{}, errors.New("Enable Banking returned no accessible account")
	}
	return session, nil
}

func (c *EnableBankingClient) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var response struct {
		Accounts     []string `json:"accounts"`
		AccountsData []struct {
			UID string `json:"uid"`
		} `json:"accounts_data"`
		Access struct {
			ValidUntil   string `json:"valid_until"`
			Transactions bool   `json:"transactions"`
		} `json:"access"`
	}
	path := "/sessions/" + url.PathEscape(sessionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return Session{}, err
	}
	validUntil, err := time.Parse(time.RFC3339Nano, response.Access.ValidUntil)
	if err != nil {
		return Session{}, errors.New("Enable Banking returned an invalid consent expiry")
	}
	accountIDs := response.Accounts
	if len(response.AccountsData) > 0 {
		accountIDs = make([]string, 0, len(response.AccountsData))
		for _, account := range response.AccountsData {
			accountIDs = append(accountIDs, account.UID)
		}
	}
	seen := make(map[string]struct{}, len(accountIDs))
	session := Session{ID: sessionID, ValidUntil: validUntil, TransactionsEnabled: response.Access.Transactions}
	for _, accountID := range accountIDs {
		if accountID == "" {
			return Session{}, errors.New("Enable Banking returned an account without an ID")
		}
		if _, exists := seen[accountID]; exists {
			return Session{}, errors.New("Enable Banking returned a duplicate account ID")
		}
		seen[accountID] = struct{}{}
		session.Accounts = append(session.Accounts, Account{ID: accountID})
	}
	if len(session.Accounts) == 0 {
		return Session{}, errors.New("Enable Banking returned no accessible account")
	}
	return session, nil
}

func (c *EnableBankingClient) GetAccountDetails(ctx context.Context, accountID string) (Account, error) {
	var response struct {
		UID      string `json:"uid"`
		Name     string `json:"name"`
		Product  string `json:"product"`
		Currency string `json:"currency"`
	}
	path := "/accounts/" + url.PathEscape(accountID) + "/details"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return Account{}, err
	}
	return Account{ID: accountID, Name: response.Name, Product: response.Product, Currency: response.Currency}, nil
}

func (c *EnableBankingClient) GetBalances(ctx context.Context, accountID string) ([]Balance, error) {
	var response struct {
		Balances []struct {
			Name   string `json:"name"`
			Type   string `json:"balance_type"`
			Amount struct {
				Amount   string `json:"amount"`
				Currency string `json:"currency"`
			} `json:"balance_amount"`
			LastChange    string `json:"last_change_date_time"`
			ReferenceDate string `json:"reference_date"`
		} `json:"balances"`
	}
	path := "/accounts/" + url.PathEscape(accountID) + "/balances"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}

	balances := make([]Balance, 0, len(response.Balances))
	for _, item := range response.Balances {
		var lastChange time.Time
		if item.LastChange != "" {
			lastChange, _ = time.Parse(time.RFC3339Nano, item.LastChange)
		}
		balances = append(balances, Balance{
			Name: item.Name, Type: item.Type, Amount: item.Amount.Amount,
			Currency: item.Amount.Currency, LastChange: lastChange, ReferenceDate: item.ReferenceDate,
		})
	}
	return balances, nil
}

func (c *EnableBankingClient) GetTransactions(ctx context.Context, accountID string, dateFrom, dateTo time.Time) ([]Transaction, error) {
	type transactionsResponse struct {
		Transactions []struct {
			Amount struct {
				Amount   string `json:"amount"`
				Currency string `json:"currency"`
			} `json:"transaction_amount"`
			Direction       string `json:"credit_debit_indicator"`
			Status          string `json:"status"`
			BookingDate     string `json:"booking_date"`
			TransactionDate string `json:"transaction_date"`
			ValueDate       string `json:"value_date"`
			Creditor        struct {
				Name string `json:"name"`
			} `json:"creditor"`
			Debtor struct {
				Name string `json:"name"`
			} `json:"debtor"`
			RemittanceInformation []string `json:"remittance_information"`
			Note                  string   `json:"note"`
		} `json:"transactions"`
		ContinuationKey string `json:"continuation_key"`
	}
	transactions := make([]Transaction, 0)
	continuationKey := ""
	for page := 0; page < 10 && len(transactions) < 1000; page++ {
		query := url.Values{
			"date_from":          []string{dateFrom.UTC().Format("2006-01-02")},
			"date_to":            []string{dateTo.UTC().Format("2006-01-02")},
			"transaction_status": []string{"BOOK"},
		}
		if continuationKey != "" {
			query.Set("continuation_key", continuationKey)
		}
		path := "/accounts/" + url.PathEscape(accountID) + "/transactions?" + query.Encode()
		var response transactionsResponse
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Transactions {
			transactions = append(transactions, Transaction{
				Amount: item.Amount.Amount, Currency: item.Amount.Currency,
				Direction: item.Direction, Status: item.Status,
				BookingDate: item.BookingDate, TransactionDate: item.TransactionDate, ValueDate: item.ValueDate,
				CreditorName: item.Creditor.Name, DebtorName: item.Debtor.Name,
				RemittanceInformation: item.RemittanceInformation, Note: item.Note,
			})
			if len(transactions) >= 1000 {
				break
			}
		}
		continuationKey = response.ContinuationKey
		if continuationKey == "" {
			break
		}
	}
	return transactions, nil
}

func (c *EnableBankingClient) doJSON(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode Enable Banking request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create Enable Banking request: %w", err)
	}
	token, err := c.jwt(time.Now())
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call Enable Banking API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return &HTTPError{StatusCode: resp.StatusCode}
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Enable Banking response: %w", err)
	}
	if len(responseBody) > maxResponseBytes {
		return errors.New("Enable Banking response exceeds the size limit")
	}
	if err := json.Unmarshal(responseBody, output); err != nil {
		return fmt.Errorf("decode Enable Banking response: %w", err)
	}
	return nil
}

func (c *EnableBankingClient) jwt(now time.Time) (string, error) {
	header, err := json.Marshal(map[string]string{"typ": "JWT", "alg": "RS256", "kid": c.applicationID})
	if err != nil {
		return "", err
	}
	iat := now.Unix()
	payload, err := json.Marshal(map[string]any{
		"iss": "enablebanking.com", "aud": "api.enablebanking.com", "iat": iat, "exp": iat + 3600,
	})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign Enable Banking JWT: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseRSAPrivateKey(value []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(value)
	if block == nil {
		return nil, errors.New("Enable Banking private key is not valid PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("Enable Banking private key is not valid PKCS#1 or PKCS#8")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("Enable Banking private key must be RSA")
	}
	return key, nil
}
