package httpweb

import (
	_ "embed"
	"net/http"
	"start/internal/service"

	"github.com/gin-gonic/gin"
)

//go:embed web/index.html
var indexHTML []byte

type handlers struct {
	svc *service.Service
}

// Register registers HTML routes.
func Register(router gin.IRouter, svc *service.Service) {
	h := handlers{svc: svc}

	router.GET("/", h.appHome)
}

func (h handlers) appHome(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
}
