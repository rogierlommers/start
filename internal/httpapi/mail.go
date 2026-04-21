package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"start/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type sendMailRequest struct {
	Body string `form:"body" binding:"required"`
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
// @Param attachments formData file false "File attachments (can be multiple)"
// @Success 202 {object} sendMailResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 503 {object} apiErrorResponse
// @Router /api/mail/send [post]
func (h handlers) sendMail(c *gin.Context) {
	var req sendMailRequest
	if err := c.ShouldBind(&req); err != nil {
		logrus.Infof("invalid send mail request: %v", err)
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid request: missing required fields"})
		return
	}

	var attachments []service.SendMailAttachment

	// Process attachments from multipart form
	skipMultipart := strings.ToLower(c.Request.Header.Get("x-skip-multipart"))

	if skipMultipart != "true" {
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

	}

	// send actual mail via service layer (non-blocking, returns immediately after queuing the mail task)
	err := h.svc.SendMail(c.Request.Context(), service.SendMailInput{
		Body:        req.Body,
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
