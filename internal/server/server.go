package server

import (
	"net/http"
	"start/internal/httpapi"
	"start/internal/httpmiddleware"
	"start/internal/httpweb"
	"start/internal/repository"
	"start/internal/service"
	"time"

	openapiui "github.com/PeterTakahashi/gin-openapi/openapiui"
	"github.com/gin-gonic/gin"
)

// NewHTTPServer builds an HTTP server configured with the project's Gin router.
func NewHTTPServer(addr string, readHeaderTimeout time.Duration) *http.Server {

	// set Gin to release mode for production use
	gin.SetMode(gin.ReleaseMode)

	// create a new router with logging and recovery middleware.
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	httpmiddleware.RegisterGlobal(router)

	store := repository.NewNoopStore()
	svc := service.New(store)

	router.GET("/docs/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/openapi.yaml",
		SpecFilePath: "./docs/swagger.yaml",
		Title:        "start API",
		Theme:        "light",
	}))

	httpapi.Register(router, svc)
	httpweb.Register(router, svc)

	return &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
