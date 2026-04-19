package httpapi

import (
	"errors"
	"net/http"
	"start/internal/mailer"
	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type handlers struct {
	svc *service.Service
}

type healthResponse struct {
	Status string `json:"status"`
}

type serviceStatusResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

type sendMailRequest struct {
	To      string `json:"to" binding:"required"`
	Subject string `json:"subject" binding:"required"`
	Body    string `json:"body" binding:"required"`
}

type sendMailResponse struct {
	Status string `json:"status"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

// Register registers JSON API routes.
func Register(router gin.IRouter, svc *service.Service) {
	h := handlers{svc: svc}

	router.GET("/openapi.yaml", h.openapiSpec)

	api := router.Group("/api")
	api.POST("/mail/send", h.sendMail)

	api.GET("/categories", h.listCategories)
	api.POST("/categories", h.createCategory)
	api.DELETE("/categories/:id", h.deleteCategory)

	api.GET("/bookmarks", h.listBookmarks)
	api.POST("/bookmarks", h.createBookmark)
	api.PATCH("/bookmarks/reorder", h.reorderBookmarks)
	api.DELETE("/bookmarks/:id", h.deleteBookmark)
}

// openapiSpec godoc
// @Summary OpenAPI specification
// @Tags docs
// @Produce application/x-yaml
// @Success 200 {string} string "OpenAPI YAML"
// @Router /openapi.yaml [get]
func (h handlers) openapiSpec(c *gin.Context) {
	c.File("docs/swagger.yaml")
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
