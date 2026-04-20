package httpapi

import (
	"errors"
	"net/http"

	"start/internal/mailer"
	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type sendMailRequest struct {
	To      string `json:"to" binding:"required"`
	Subject string `json:"subject" binding:"required"`
	Body    string `json:"body" binding:"required"`
}

type sendMailResponse struct {
	Status string `json:"status"`
}

// sendMail godoc
// @Summary Send email message
// @Tags mail
// @Accept json
// @Produce json
// @Param request body sendMailRequest true "Mail payload"
// @Success 202 {object} sendMailResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 503 {object} apiErrorResponse
// @Router /api/mail/send [post]
func (h handlers) sendMail(c *gin.Context) {
	var req sendMailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid JSON body"})
		return
	}

	err := h.svc.SendMail(c.Request.Context(), service.SendMailInput{
		To:      req.To,
		Subject: req.Subject,
		Body:    req.Body,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidMailInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid mail payload"})
		case errors.Is(err, mailer.ErrDisabled):
			c.JSON(http.StatusServiceUnavailable, apiErrorResponse{Error: "mailer is not configured"})
		default:
			c.JSON(http.StatusServiceUnavailable, apiErrorResponse{Error: "failed to send mail"})
		}
		return
	}

	c.JSON(http.StatusAccepted, sendMailResponse{Status: "accepted"})
}
