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
	"github.com/sirupsen/logrus"
)

// ServerContext holds the HTTP server and service components.
type ServerContext struct {
	Server  *http.Server
	Service *service.Service
}

// NewHTTPServer builds an HTTP server configured with the project's Gin router.
func NewHTTPServer(cfg config.Config) *ServerContext {

	// set Gin to release mode for production use
	gin.SetMode(gin.ReleaseMode)

	// create a new router with logging and recovery middleware.
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	httpmiddleware.RegisterGlobal(router)

	// persistency layer
	store := repository.NewMemoryStore()

	// mailer setup, using SMTP if configured, otherwise a disabled sender
	var sender mailer.Sender = mailer.DisabledSender{}
	if cfg.SMTPHost != "" && cfg.SMTPFrom != "" {
		logrus.Infof("configured SMTP mailer with host %s and from address %s", cfg.SMTPHost, cfg.SMTPFrom)
		sender = mailer.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	}

	// service layer
	svc := service.New(store, sender, cfg)

	// start background mail worker
	svc.StartMailWorker()

	// API documentation endpoint
	router.GET("/docs/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/openapi.yaml",
		SpecFilePath: "./docs/swagger.yaml",
		Title:        "start API",
		Theme:        "light",
	}))

	// register API and web handlers
	httpapi.Register(router, svc, cfg)
	httpweb.Register(router, svc)

	return &ServerContext{
		Server: &http.Server{
			Addr:              cfg.HostPort,
			Handler:           router,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		},
		Service: svc,
	}
}
