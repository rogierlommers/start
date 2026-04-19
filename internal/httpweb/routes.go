package httpweb

import (
	"net/http"
	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type handlers struct {
	svc *service.Service
}

// Register registers HTML routes.
func Register(router gin.IRouter, svc *service.Service) {
	h := handlers{svc: svc}

	router.GET("/app", h.appHome)
}

// appHome godoc
// @Summary Dashboard HTML page
// @Tags web
// @Produce html
// @Success 200 {string} string "HTML page"
// @Router /app [get]
func (h handlers) appHome(c *gin.Context) {
	status := h.svc.ServiceStatus()

	html := "<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>start</title></head><body><main><h1>start dashboard</h1><p>service: " + status["service"] + "</p><p>status: " + status["status"] + "</p></main></body></html>"
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
