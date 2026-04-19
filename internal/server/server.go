package server

import (
	"net/http"
	"start/internal/config"
	"start/internal/httpapi"
	"start/internal/httpmiddleware"
	"start/internal/httpweb"
	"start/internal/mailer"
	"start/internal/repository"
	"start/internal/service"

	openapiui "github.com/PeterTakahashi/gin-openapi/openapiui"
	"github.com/gin-gonic/gin"
)

// NewHTTPServer builds an HTTP server configured with the project's Gin router.
func NewHTTPServer(cfg config.Config) *http.Server {

	// set Gin to release mode for production use
	gin.SetMode(gin.ReleaseMode)

	// create a new router with logging and recovery middleware.
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	httpmiddleware.RegisterGlobal(router)

	store := repository.NewMemoryStore()
	var sender mailer.Sender = mailer.DisabledSender{}
	if cfg.SMTPHost != "" && cfg.SMTPFrom != "" {
		sender = mailer.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	}

	svc := service.New(store, sender)

	router.GET("/docs/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/openapi.yaml",
		SpecFilePath: "./docs/swagger.yaml",
		Title:        "start API",
		Theme:        "light",
	}))

	httpapi.Register(router, svc)
	httpweb.Register(router, svc)

	return &http.Server{
		Addr:              cfg.HostPort,
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}
}
