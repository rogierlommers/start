// @title start API
// @version 0.1.0
// @description API documentation generated from handler annotations.
// @description Protected API routes accept HTTP Basic auth with API_USERNAME and API_PASSWORD.
// @securityDefinitions.basic ApiBasicAuth
// @BasePath /
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"start/internal/config"
	"start/internal/server"
	"syscall"

	"github.com/sirupsen/logrus"
)

var appVersion = "<< replaced during build >>"

func main() {

	cfg, err := config.Load()
	if err != nil {
		logrus.Fatalf("failed to load config: %v", err)
	}

	// get a configured HTTP server instance
	ctx, err := server.NewHTTPServer(cfg, appVersion)
	if err != nil {
		logrus.Fatalf("failed to initialize HTTP server: %v", err)
	}
	defer ctx.Service.Close()

	// set up graceful shutdown on SIGINT/SIGTERM
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh

		logrus.Infof("received signal %s, shutting down", sig)

		c, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		// attempt graceful shutdown with timeout
		if err := ctx.Server.Shutdown(c); err != nil {
			logrus.Infof("graceful shutdown failed: %v", err)
		}
	}()

	// run the server
	logrus.Infof("start backend listening on http://%s", cfg.HostPort)
	if err := ctx.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.Fatalf("server error: %v", err)
	}

	// wait for shutdown to complete
	<-shutdownDone
	logrus.Infof("server stopped")
}

func init() {
	printBanner()
}
