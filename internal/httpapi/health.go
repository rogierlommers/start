package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type healthResponse struct {
	Status string `json:"status"`
}

// health godoc
// @Summary Health check
// @Tags health
// @Produce json
// @Success 200 {object} healthResponse
// @Router /api/health [get]
func (h handlers) health(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}
