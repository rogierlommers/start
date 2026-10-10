package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type bankBalanceResponse struct {
	Status              string     `json:"status"`
	AccountKey          string     `json:"account_key,omitempty"`
	AccountName         string     `json:"account_name,omitempty"`
	ProviderAccountName string     `json:"provider_account_name,omitempty"`
	Alias               string     `json:"alias,omitempty"`
	Amount              string     `json:"amount,omitempty"`
	Currency            string     `json:"currency,omitempty"`
	BalanceType         string     `json:"balance_type,omitempty"`
	BalanceName         string     `json:"balance_name,omitempty"`
	ProviderTime        *time.Time `json:"provider_time,omitempty"`
	FetchedAt           *time.Time `json:"fetched_at,omitempty"`
	ValidUntil          *time.Time `json:"valid_until,omitempty"`
	Stale               bool       `json:"stale"`
}

type bankBalancesResponse struct {
	Status   string                `json:"status"`
	Accounts []bankBalanceResponse `json:"accounts"`

	// Deprecated single-account fields remain populated from the first account.
	AccountName  string     `json:"account_name,omitempty"`
	Amount       string     `json:"amount,omitempty"`
	Currency     string     `json:"currency,omitempty"`
	BalanceType  string     `json:"balance_type,omitempty"`
	BalanceName  string     `json:"balance_name,omitempty"`
	ProviderTime *time.Time `json:"provider_time,omitempty"`
	FetchedAt    *time.Time `json:"fetched_at,omitempty"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	Stale        bool       `json:"stale"`
}

type bankTransactionResponse struct {
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	Direction    string `json:"direction"`
	Date         string `json:"date"`
	Counterparty string `json:"counterparty,omitempty"`
	Description  string `json:"description,omitempty"`
}

type bankAccountTransactionsResponse struct {
	Status              string                    `json:"status"`
	AccountKey          string                    `json:"account_key"`
	AccountName         string                    `json:"account_name"`
	ProviderAccountName string                    `json:"provider_account_name,omitempty"`
	Alias               string                    `json:"alias,omitempty"`
	Transactions        []bankTransactionResponse `json:"transactions"`
	FetchedAt           *time.Time                `json:"fetched_at,omitempty"`
	ValidUntil          *time.Time                `json:"valid_until,omitempty"`
	Stale               bool                      `json:"stale"`
}

type bankTransactionsResponse struct {
	Status   string                            `json:"status"`
	Accounts []bankAccountTransactionsResponse `json:"accounts"`
}

type bankAccountAliasRequest struct {
	Alias string `json:"alias"`
}

type bankAccountAliasResponse struct {
	Alias string `json:"alias"`
}

// getBankBalance godoc
// @Summary Get all connected ING account balances
// @Tags banking
// @Produce json
// @Security ApiBasicAuth
// @Success 200 {object} bankBalancesResponse
// @Failure 502 {object} apiErrorResponse
// @Router /api/banking/balance [get]
func (h handlers) getBankBalance(c *gin.Context) {
	overview, err := h.svc.GetBankBalances(c.Request.Context(), time.Now(), c.Query("refresh") == "1")
	if err != nil {
		c.JSON(http.StatusBadGateway, apiErrorResponse{Error: "failed to load bank balance"})
		return
	}
	response := bankBalancesResponse{
		Status:   overview.Status,
		Accounts: make([]bankBalanceResponse, 0, len(overview.Accounts)),
	}
	for _, account := range overview.Accounts {
		response.Accounts = append(response.Accounts, bankBalanceAPIResponse(account))
	}
	if len(response.Accounts) > 0 {
		first := response.Accounts[0]
		response.AccountName = first.AccountName
		response.Amount = first.Amount
		response.Currency = first.Currency
		response.BalanceType = first.BalanceType
		response.BalanceName = first.BalanceName
		response.ProviderTime = first.ProviderTime
		response.FetchedAt = first.FetchedAt
		response.ValidUntil = first.ValidUntil
		response.Stale = first.Stale
	}
	c.JSON(http.StatusOK, response)
}

func bankBalanceAPIResponse(overview service.BankBalanceOverview) bankBalanceResponse {
	response := bankBalanceResponse{
		Status: overview.Status, AccountKey: overview.AccountKey,
		AccountName: overview.AccountName, ProviderAccountName: overview.ProviderAccountName, Alias: overview.Alias,
		Amount:   overview.Amount,
		Currency: overview.Currency, BalanceType: overview.BalanceType,
		BalanceName: overview.BalanceName, Stale: overview.Stale,
	}
	if !overview.ProviderTime.IsZero() {
		providerTime := overview.ProviderTime
		response.ProviderTime = &providerTime
	}
	if !overview.FetchedAt.IsZero() {
		fetchedAt := overview.FetchedAt
		response.FetchedAt = &fetchedAt
	}
	if !overview.ValidUntil.IsZero() {
		validUntil := overview.ValidUntil
		response.ValidUntil = &validUntil
	}
	return response
}

// getBankTransactions godoc
// @Summary Get recent booked transactions for all connected ING accounts
// @Tags banking
// @Produce json
// @Security ApiBasicAuth
// @Param limit query int false "Transactions per account (1-50)" default(10)
// @Success 200 {object} bankTransactionsResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 502 {object} apiErrorResponse
// @Router /api/banking/transactions [get]
func (h handlers) getBankTransactions(c *gin.Context) {
	limit := 10
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 50 {
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "limit must be between 1 and 50"})
			return
		}
		limit = parsed
	}
	overview, err := h.svc.GetBankTransactions(c.Request.Context(), time.Now(), limit, c.Query("refresh") == "1")
	if err != nil {
		c.JSON(http.StatusBadGateway, apiErrorResponse{Error: "failed to load bank transactions"})
		return
	}
	response := bankTransactionsResponse{
		Status:   overview.Status,
		Accounts: make([]bankAccountTransactionsResponse, 0, len(overview.Accounts)),
	}
	for _, account := range overview.Accounts {
		mapped := bankAccountTransactionsResponse{
			Status: account.Status, AccountKey: account.AccountKey,
			AccountName: account.AccountName, ProviderAccountName: account.ProviderAccountName, Alias: account.Alias,
			Stale:        account.Stale,
			Transactions: make([]bankTransactionResponse, 0, len(account.Transactions)),
		}
		for _, transaction := range account.Transactions {
			mapped.Transactions = append(mapped.Transactions, bankTransactionResponse{
				Amount: transaction.Amount, Currency: transaction.Currency, Direction: transaction.Direction,
				Date: transaction.Date, Counterparty: transaction.Counterparty, Description: transaction.Description,
			})
		}
		if !account.FetchedAt.IsZero() {
			fetchedAt := account.FetchedAt
			mapped.FetchedAt = &fetchedAt
		}
		if !account.ValidUntil.IsZero() {
			validUntil := account.ValidUntil
			mapped.ValidUntil = &validUntil
		}
		response.Accounts = append(response.Accounts, mapped)
	}
	c.JSON(http.StatusOK, response)
}

// updateBankAccountAlias godoc
// @Summary Set or clear a local bank account alias
// @Tags banking
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param accountKey path string true "Opaque account key"
// @Param request body bankAccountAliasRequest true "Alias; an empty value clears it"
// @Success 200 {object} bankAccountAliasResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 404 {object} apiErrorResponse
// @Router /api/banking/accounts/{accountKey}/alias [patch]
func (h handlers) updateBankAccountAlias(c *gin.Context) {
	var request bankAccountAliasRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid alias request"})
		return
	}
	alias, err := h.svc.SetBankAccountAlias(c.Request.Context(), c.Param("accountKey"), request.Alias, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBankingDisabled):
			c.JSON(http.StatusServiceUnavailable, apiErrorResponse{Error: "banking integration is not configured"})
		case errors.Is(err, service.ErrInvalidBankAlias):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "alias must be at most 80 characters and contain no control characters"})
		case errors.Is(err, service.ErrBankAccountNotFound):
			c.JSON(http.StatusNotFound, apiErrorResponse{Error: "bank account not found"})
		default:
			c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to save bank account alias"})
		}
		return
	}
	c.JSON(http.StatusOK, bankAccountAliasResponse{Alias: alias})
}

func (h handlers) connectBank(c *gin.Context) {
	redirectURL, err := h.svc.StartBankAuthorization(c.Request.Context(), time.Now())
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, service.ErrBankingDisabled) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, apiErrorResponse{Error: "failed to start bank connection"})
		return
	}
	c.Redirect(http.StatusSeeOther, redirectURL)
}

func (h handlers) bankCallback(c *gin.Context) {
	if c.Query("error") != "" {
		c.Redirect(http.StatusSeeOther, "/?banking=cancelled")
		return
	}
	if err := h.svc.CompleteBankAuthorization(
		c.Request.Context(), c.Query("state"), c.Query("code"), time.Now(),
	); err != nil {
		c.Redirect(http.StatusSeeOther, "/?banking=failed")
		return
	}
	c.Redirect(http.StatusSeeOther, "/?banking=connected")
}
