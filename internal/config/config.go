package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultShutdownTimeout   = 10 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
)

// Config contains runtime settings sourced from environment variables.
type Config struct {
	HostPort          string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	LogLevel          string
}

// Load reads runtime configuration from environment variables with defaults.
func Load() (Config, error) {

	// load env vars from .env file.
	if err := godotenv.Load(); err != nil {
		return Config{}, fmt.Errorf("failed to load .env file: %w", err)
	}

	cfg := Config{
		HostPort:          os.Getenv("HTTP_BIND_ADDR"),
		LogLevel:          os.Getenv("LOG_LEVEL"),
		ShutdownTimeout:   defaultShutdownTimeout,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	return cfg, nil
}
