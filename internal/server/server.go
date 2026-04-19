package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// NewHTTPServer builds an HTTP server configured with the project's Gin router.
func NewHTTPServer(addr string, readHeaderTimeout time.Duration) *http.Server {

	// set Gin to release mode for production use
	gin.SetMode(gin.ReleaseMode)

	// create a new router with logging and recovery middleware.
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// add a simple health check endpoint and a root endpoint for basic status.
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// root endpoint for basic status information
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "start", "status": "running"})
	})

	return &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
