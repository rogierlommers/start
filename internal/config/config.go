package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultShutdownTimeout   = 10 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
)

// Config contains runtime settings sourced from environment variables.
type Config struct {
	HostPort           string
	ShutdownTimeout    time.Duration
	ReadHeaderTimeout  time.Duration
	EnableAccessLogs   bool
	LogLevel           string
	SQLitePath         string
	StorageUploadDir   string
	StorageMaxUploadMB int64
	StorageCleanupDays int
	SMTPHost           string
	SMTPPort           int
	SMTPUsername       string
	SMTPPassword       string
	SMTPFrom           string
	MailerEmailPrivate string
	MailerEmailWork    string
	GUIUsername        string
	GUIPassword        string
	APIUsername        string
	APIPassword        string
}

// Load reads runtime configuration from environment variables with defaults.
func Load() (Config, error) {

	// load env vars from .env file.
	if err := godotenv.Load(); err != nil {
		return Config{}, fmt.Errorf("failed to load .env file: %w", err)
	}

	cfg := Config{
		// set defaults for timeouts and other settings
		ShutdownTimeout:   defaultShutdownTimeout,
		ReadHeaderTimeout: defaultReadHeaderTimeout,

		// server settings
		HostPort:         os.Getenv("HTTP_BIND_ADDR"),
		LogLevel:         os.Getenv("LOG_LEVEL"),
		SQLitePath:       os.Getenv("SQLITE_PATH"),
		GUIUsername:      os.Getenv("GUI_USERNAME"),
		GUIPassword:      os.Getenv("GUI_PASSWORD"),
		APIUsername:      os.Getenv("API_USERNAME"),
		APIPassword:      os.Getenv("API_PASSWORD"),
		EnableAccessLogs: false, // default to false, can be enabled with env var

		// storage settings
		StorageUploadDir:   os.Getenv("STORAGE_UPLOAD_DIR"),
		StorageMaxUploadMB: 100, // default max upload size of 100 MB
		StorageCleanupDays: 30,

		// mailer settings
		SMTPHost:           os.Getenv("MAILER_SMTP_HOST"),
		SMTPUsername:       os.Getenv("MAILER_SMTP_USERNAME"),
		SMTPPassword:       os.Getenv("MAILER_SMTP_PASSWORD"),
		SMTPFrom:           os.Getenv("MAILER_SMTP_FROM"),
		SMTPPort:           587,
		MailerEmailPrivate: os.Getenv("MAILER_EMAIL_PRIVATE"),
		MailerEmailWork:    os.Getenv("MAILER_EMAIL_WORK"),
	}

	if rawPort := os.Getenv("SMTP_PORT"); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port <= 0 {
			return Config{}, fmt.Errorf("invalid SMTP_PORT value %q", rawPort)
		}
		cfg.SMTPPort = port
	}

	if rawUploadDir := os.Getenv("STORAGE_UPLOAD_DIR"); rawUploadDir != "" {
		cfg.StorageUploadDir = rawUploadDir
	}

	if rawSQLitePath := strings.TrimSpace(os.Getenv("SQLITE_PATH")); rawSQLitePath != "" {
		cfg.SQLitePath = rawSQLitePath
	}

	if rawUploadMB := os.Getenv("STORAGE_MAX_UPLOAD_MB"); rawUploadMB != "" {
		uploadMB, err := strconv.ParseInt(rawUploadMB, 10, 64)
		if err != nil || uploadMB <= 0 {
			return Config{}, fmt.Errorf("invalid STORAGE_MAX_UPLOAD_MB value %q", rawUploadMB)
		}
		cfg.StorageMaxUploadMB = uploadMB
	}

	if rawCleanupDays := os.Getenv("STORAGE_CLEANUP_DAYS"); rawCleanupDays != "" {
		cleanupDays, err := strconv.Atoi(rawCleanupDays)
		if err != nil || cleanupDays < 0 {
			return Config{}, fmt.Errorf("invalid STORAGE_CLEANUP_DAYS value %q", rawCleanupDays)
		}
		cfg.StorageCleanupDays = cleanupDays
	}

	if raw := os.Getenv("ENABLE_ACCESS_LOGS"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ENABLE_ACCESS_LOGS value %q", raw)
		}
		cfg.EnableAccessLogs = v
	}
	return cfg, nil
}
