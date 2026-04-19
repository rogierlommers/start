package httpapi

import (
	"net/http"
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

// Register registers JSON API routes.
func Register(router gin.IRouter, svc *service.Service) {
	h := handlers{svc: svc}

	router.GET("/healthz", h.healthz)
	router.GET("/", h.rootStatus)
	router.GET("/openapi.yaml", h.openapiSpec)

	api := router.Group("/api")
	api.GET("/status", h.rootStatus)
}

// healthz godoc
// @Summary Health check
// @Tags system
// @Produce json
// @Success 200 {object} healthResponse
// @Router /healthz [get]
func (h handlers) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.HealthStatus())
}

// rootStatus godoc
// @Summary Root service status
// @Tags system
// @Produce json
// @Success 200 {object} serviceStatusResponse
// @Router / [get]
// @Router /api/status [get]
func (h handlers) rootStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.ServiceStatus())
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
