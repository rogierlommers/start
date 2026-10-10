package httpapi

import (
	"errors"
	"net/http"
	"time"

	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type bankBalanceResponse struct {
	Status       string     `json:"status"`
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

// getBankBalance godoc
// @Summary Get the connected ING account balance
// @Tags banking
// @Produce json
// @Security ApiBasicAuth
// @Success 200 {object} bankBalanceResponse
// @Failure 502 {object} apiErrorResponse
// @Router /api/banking/balance [get]
func (h handlers) getBankBalance(c *gin.Context) {
	overview, err := h.svc.GetBankBalance(c.Request.Context(), time.Now(), c.Query("refresh") == "1")
	if err != nil {
		c.JSON(http.StatusBadGateway, apiErrorResponse{Error: "failed to load bank balance"})
		return
	}
	response := bankBalanceResponse{
		Status: overview.Status, AccountName: overview.AccountName, Amount: overview.Amount,
		Currency: overview.Currency, BalanceType: overview.BalanceType,
		BalanceName: overview.BalanceName, Stale: overview.Stale,
	}
	if !overview.ProviderTime.IsZero() {
		response.ProviderTime = &overview.ProviderTime
	}
	if !overview.FetchedAt.IsZero() {
		response.FetchedAt = &overview.FetchedAt
	}
	if !overview.ValidUntil.IsZero() {
		response.ValidUntil = &overview.ValidUntil
	}
	c.JSON(http.StatusOK, response)
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
