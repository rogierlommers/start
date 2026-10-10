package server

import (
	"fmt"
	"net/http"
	"start/internal/banking"
	"start/internal/config"
	"start/internal/httpapi"
	"start/internal/httpmiddleware"
	"start/internal/httpweb"
	"start/internal/mailer"
	"start/internal/repository"
	"start/internal/service"
	"time"

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
func NewHTTPServer(cfg config.Config, appBuildTime string) (*ServerContext, error) {

	// set Gin to release mode for production use
	gin.SetMode(gin.ReleaseMode)

	// create a new router with logging and recovery middleware.
	router := gin.New()
	router.Use(gin.Recovery())

	if cfg.EnableAccessLogs {
		logrus.Info("access logs are enabled")
		router.Use(gin.Logger())
	} else {
		logrus.Info("access logs are disabled")
	}

	httpmiddleware.RegisterGlobal(router)
	guiAuth, err := httpmiddleware.NewGUIAuth(cfg)
	if err != nil {
		return nil, err
	}

	// persistency layer
	store, err := repository.NewSQLiteStore(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}

	var bankingClient banking.Client
	if cfg.EnableBankingEnabled() {
		bankingClient, err = banking.NewEnableBankingClient(
			cfg.EnableBankingAppID,
			cfg.EnableBankingPrivateKey,
			&http.Client{Timeout: 15 * time.Second},
		)
		if err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("configure Enable Banking: %w", err)
		}
	}

	// mailer setup, using SMTP if configured, otherwise a disabled sender
	var sender mailer.Sender = mailer.DisabledSender{}
	if cfg.SMTPHost != "" && cfg.SMTPFrom != "" {
		logrus.Infof("configured SMTP mailer with host %s and from address %s", cfg.SMTPHost, cfg.SMTPFrom)
		sender = mailer.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	}

	// service layer
	svc := service.NewWithOptions(store, sender, cfg, service.Options{BankingClient: bankingClient})

	// start background workers
	svc.StartMailWorker()
	svc.StartStorageCleanupWorker()
	svc.StartReadingListCleanupWorker()
	svc.StartIncidentManagerWorker()
	svc.StartBankingCacheWorker()

	httpweb.RegisterPublicWithContact(router, guiAuth, cfg.DataProtectionEmail)
	httpapi.RegisterPublic(router, svc, cfg)

	protected := router.Group("")
	protected.Use(guiAuth.RequireAuth())

	// API documentation endpoint
	protected.GET("/docs/*any", openapiui.WrapHandler(openapiui.Config{
		SpecURL:      "/openapi.yaml",
		SpecFilePath: "./swagger-docs/swagger.yaml",
		Title:        "start API",
		Theme:        "light",
	}))

	// register API and web handlers
	httpapi.Register(protected, svc, cfg)
	httpweb.Register(protected, svc, appBuildTime, cfg.StorageSecretKey)

	return &ServerContext{
		Server: &http.Server{
			Addr:              cfg.HostPort,
			Handler:           router,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		},
		Service: svc,
	}, nil
}
