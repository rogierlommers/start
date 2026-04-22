package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"start/internal/config"
	"start/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const generatedSubjectMaxLen = 80

type sendMailRequest struct {
	Body    string `form:"body" binding:"required"`
	Subject string `form:"subject"`
	To      string `form:"to"`
}

type sendMailResponse struct {
	Status string `json:"status"`
}

// sendMail godoc
// @Summary Send email message with optional attachments
// @Tags mail
// @Accept mpfd
// @Produce json
// @Param body formData string true "Email body"
// @Param subject formData string false "Email subject"
// @Param to formData string false "Explicit recipient email address"
// @Param attachments formData file false "File attachments (can be multiple)"
// @Success 202 {object} sendMailResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 503 {object} apiErrorResponse
// @Router /api/mail/send [post]
func (h handlers) sendMail(c *gin.Context) {
	var req sendMailRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid request: missing required fields"})
		return
	}

	// if To and Subject are empry, then determine recipient and
	// subject based on bodycontent (e.g. leading "w " for work email)
	if req.To == "" && req.Subject == "" {
		req.To, req.Body, req.Subject = deterimeRecipientBodyAndSubject(h.cfg, req.Body)
	}

	// if parse multipart form
	var attachments []service.SendMailAttachment

	// Process attachments from multipart form
	// if strings.ToLower(c.Request.Header.Get("x-skip-multipart")) != "true" {
	form, err := c.MultipartForm()
	if err != nil {
		logrus.Errorf("failed to parse multipart form: %v", err)
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid request: failed to parse form data"})
		return
	}

	// Extract files from "attachments" form field
	if files, ok := form.File["attachments"]; ok {
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				logrus.Infof("failed to open attachment %s: %v", fileHeader.Filename, err)
				c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid request: failed to read attachment"})
				return
			}
			defer file.Close()

			// Limit attachment size to 10MB per file
			data, err := io.ReadAll(io.LimitReader(file, 10*1024*1024))
			if err != nil {
				c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid request: attachment too large or read error"})
				return
			}

			attachments = append(attachments, service.SendMailAttachment{
				Filename: fileHeader.Filename,
				Data:     data,
			})
		}
	}

	// send actual mail via service layer (non-blocking, returns immediately after queuing the mail task)
	logrus.Infof("queuing mail to %s with subject '%s' and %d attachment(s)", req.To, req.Subject, len(attachments))
	err = h.svc.SendMail(c.Request.Context(), service.SendMailInput{
		Body:        req.Body,
		Subject:     req.Subject,
		To:          req.To,
		Attachments: attachments,
	})

	if err != nil {
		logrus.Errorf("failed to send mail: %v", err)
		switch {
		case errors.Is(err, service.ErrInvalidMailInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid mail payload"})
		case errors.Is(err, service.ErrMailQueueFull):
			c.JSON(http.StatusServiceUnavailable, apiErrorResponse{Error: "mail service temporarily unavailable; try again later"})
		case errors.Is(err, service.ErrDisabledMailer):
			c.JSON(http.StatusServiceUnavailable, apiErrorResponse{Error: "mailer is not configured"})
		default:
			c.JSON(http.StatusServiceUnavailable, apiErrorResponse{Error: "failed to send mail"})
		}
		return
	}

	c.JSON(http.StatusAccepted, sendMailResponse{Status: "accepted"})
}

// deterimeRecipientBodyAndSubject checks if the body starts with a "w " or "W ".
// if so, then use the work email, otherwise use the private email.
// also, if work email, then strip the leading "w" or "W" from the body to avoid confusion in the email content.
// as the subject, use the first line of the body, truncated by 80 characters
func deterimeRecipientBodyAndSubject(cfg config.Config, body string) (string, string, string) {
	trimmedLeft := strings.TrimLeft(body, " \t\r\n")
	recipient := cfg.MailerEmailPrivate

	if strings.HasPrefix(trimmedLeft, "w ") || strings.HasPrefix(trimmedLeft, "W ") {
		recipient = cfg.MailerEmailWork
		trimmedLeft = trimmedLeft[2:]
	}

	cleanBody := strings.TrimSpace(trimmedLeft)
	subject := deriveSubject(cleanBody)

	logrus.Infof("determined recipient '%s' and subject '%s' from body prefix", recipient, subject)
	return recipient, cleanBody, subject
}

func deriveSubject(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}

	parts := strings.SplitSeq(trimmed, "\n")
	for part := range parts {
		line := strings.TrimSpace(part)
		if line == "" {
			continue
		}

		runes := []rune(line)
		if len(runes) <= generatedSubjectMaxLen {
			return line
		}

		return strings.TrimSpace(string(runes[:generatedSubjectMaxLen]))
	}

	return ""
}
