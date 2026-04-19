package config

import (
	"fmt"
	"os"
	"strconv"
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
	SMTPHost          string
	SMTPPort          int
	SMTPUsername      string
	SMTPPassword      string
	SMTPFrom          string
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
		SMTPHost:          os.Getenv("SMTP_HOST"),
		SMTPUsername:      os.Getenv("SMTP_USERNAME"),
		SMTPPassword:      os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:          os.Getenv("SMTP_FROM"),
		SMTPPort:          587,
	}

	if rawPort := os.Getenv("SMTP_PORT"); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port <= 0 {
			return Config{}, fmt.Errorf("invalid SMTP_PORT value %q", rawPort)
		}
		cfg.SMTPPort = port
	}

	return cfg, nil
}
