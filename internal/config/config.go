package config

import (
	"fmt"
	"net/mail"
	"net/url"
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
	HostPort                 string
	ShutdownTimeout          time.Duration
	ReadHeaderTimeout        time.Duration
	EnableAccessLogs         bool
	LogLevel                 string
	SQLitePath               string
	StorageUploadDir         string
	StorageSecretKey         string
	StorageMaxUploadMB       int64
	StorageCleanupDays       int
	ReadingListCleanupDays   int
	SMTPHost                 string
	SMTPPort                 int
	SMTPUsername             string
	SMTPPassword             string
	SMTPFrom                 string
	MailerEmailPrivate       string
	MailerEmailWork          string
	IncidentManagerICALURL   string
	IncidentManagerNotifyAt  string
	GUIUsername              string
	GUIPassword              string
	GUISessionSecret         string
	APIUsername              string
	APIPassword              string
	EnableBankingAppID       string
	EnableBankingPrivateKey  string
	EnableBankingCallbackURL string
	EnableBankingASPSPName   string
	EnableBankingCacheTTL    time.Duration
	DataProtectionEmail      string
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
		HostPort:                 os.Getenv("HTTP_BIND_ADDR"),
		LogLevel:                 os.Getenv("LOG_LEVEL"),
		SQLitePath:               os.Getenv("SQLITE_PATH"),
		GUIUsername:              os.Getenv("GUI_USERNAME"),
		GUIPassword:              os.Getenv("GUI_PASSWORD"),
		GUISessionSecret:         os.Getenv("GUI_SESSION_SECRET"),
		APIUsername:              os.Getenv("API_USERNAME"),
		APIPassword:              os.Getenv("API_PASSWORD"),
		EnableBankingAppID:       strings.TrimSpace(os.Getenv("ENABLE_BANKING_APPLICATION_ID")),
		EnableBankingPrivateKey:  strings.TrimSpace(os.Getenv("ENABLE_BANKING_PRIVATE_KEY_PATH")),
		EnableBankingCallbackURL: strings.TrimSpace(os.Getenv("ENABLE_BANKING_CALLBACK_URL")),
		EnableBankingASPSPName:   "ING",
		EnableBankingCacheTTL:    60 * time.Minute,
		DataProtectionEmail:      strings.TrimSpace(os.Getenv("DATA_PROTECTION_EMAIL")),
		EnableAccessLogs:         false, // default to false, can be enabled with env var

		// storage settings
		StorageUploadDir:       os.Getenv("STORAGE_UPLOAD_DIR"),
		StorageSecretKey:       os.Getenv("STORAGE_SECRET_KEY"),
		StorageMaxUploadMB:     100, // default max upload size of 100 MB
		StorageCleanupDays:     30,
		ReadingListCleanupDays: 30,

		// mailer settings
		SMTPHost:           os.Getenv("MAILER_SMTP_HOST"),
		SMTPUsername:       os.Getenv("MAILER_SMTP_USERNAME"),
		SMTPPassword:       os.Getenv("MAILER_SMTP_PASSWORD"),
		SMTPFrom:           os.Getenv("MAILER_SMTP_FROM"),
		SMTPPort:           587,
		MailerEmailPrivate: os.Getenv("MAILER_EMAIL_PRIVATE"),
		MailerEmailWork:    os.Getenv("MAILER_EMAIL_WORK"),

		// incident-manager notification settings
		IncidentManagerICALURL:  strings.TrimSpace(os.Getenv("INCIDENT_MANAGER_ICAL_URL")),
		IncidentManagerNotifyAt: "17:00",
	}
	if raw := strings.TrimSpace(os.Getenv("ENABLE_BANKING_ASPSP_NAME")); raw != "" {
		cfg.EnableBankingASPSPName = raw
	}
	if raw := strings.TrimSpace(os.Getenv("ENABLE_BANKING_CACHE_MINUTES")); raw != "" {
		cacheTTL, err := time.ParseDuration(raw + "m")
		if err != nil || cacheTTL <= 0 {
			return Config{}, fmt.Errorf("invalid ENABLE_BANKING_CACHE_MINUTES value %q", raw)
		}
		cfg.EnableBankingCacheTTL = cacheTTL
	}
	if err := validateEnableBankingConfig(cfg); err != nil {
		return Config{}, err
	}

	if raw := strings.TrimSpace(os.Getenv("INCIDENT_MANAGER_NOTIFICATION_TIME")); raw != "" {
		cfg.IncidentManagerNotifyAt = raw
	}
	if cfg.IncidentManagerICALURL != "" {
		feedURL, err := url.Parse(cfg.IncidentManagerICALURL)
		if err != nil || (feedURL.Scheme != "http" && feedURL.Scheme != "https") || feedURL.Host == "" {
			return Config{}, fmt.Errorf("invalid INCIDENT_MANAGER_ICAL_URL: expected an absolute HTTP or HTTPS URL")
		}
		if _, err := time.Parse("15:04", cfg.IncidentManagerNotifyAt); err != nil {
			return Config{}, fmt.Errorf("invalid INCIDENT_MANAGER_NOTIFICATION_TIME value %q: expected HH:MM", cfg.IncidentManagerNotifyAt)
		}
		if strings.TrimSpace(cfg.MailerEmailWork) == "" {
			return Config{}, fmt.Errorf("MAILER_EMAIL_WORK is required when INCIDENT_MANAGER_ICAL_URL is configured")
		}
		if _, err := mail.ParseAddress(cfg.MailerEmailWork); err != nil {
			return Config{}, fmt.Errorf("invalid MAILER_EMAIL_WORK value: %w", err)
		}
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

	if rawCleanupDays := os.Getenv("READING_LIST_CLEANUP_DAYS"); rawCleanupDays != "" {
		cleanupDays, err := strconv.Atoi(rawCleanupDays)
		if err != nil || cleanupDays < 0 {
			return Config{}, fmt.Errorf("invalid READING_LIST_CLEANUP_DAYS value %q", rawCleanupDays)
		}
		cfg.ReadingListCleanupDays = cleanupDays
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

// EnableBankingEnabled reports whether all required provider settings are present.
func (c Config) EnableBankingEnabled() bool {
	return c.EnableBankingAppID != "" && c.EnableBankingPrivateKey != "" && c.EnableBankingCallbackURL != ""
}

func validateEnableBankingConfig(cfg Config) error {
	values := []string{cfg.EnableBankingAppID, cfg.EnableBankingPrivateKey, cfg.EnableBankingCallbackURL}
	configured := 0
	for _, value := range values {
		if value != "" {
			configured++
		}
	}
	if configured == 0 {
		return nil
	}
	if configured != len(values) {
		return fmt.Errorf("ENABLE_BANKING_APPLICATION_ID, ENABLE_BANKING_PRIVATE_KEY_PATH, and ENABLE_BANKING_CALLBACK_URL must be configured together")
	}
	callbackURL, err := url.Parse(cfg.EnableBankingCallbackURL)
	if err != nil || callbackURL.Scheme != "https" || callbackURL.Host == "" || callbackURL.User != nil {
		return fmt.Errorf("invalid ENABLE_BANKING_CALLBACK_URL: expected an absolute HTTPS URL without user information")
	}
	if strings.TrimSpace(cfg.GUIUsername) == "" || strings.TrimSpace(cfg.GUIPassword) == "" || len(cfg.GUISessionSecret) < 32 {
		return fmt.Errorf("Enable Banking requires GUI_USERNAME, GUI_PASSWORD, and GUI_SESSION_SECRET of at least 32 characters")
	}
	if cfg.DataProtectionEmail == "" {
		return fmt.Errorf("DATA_PROTECTION_EMAIL is required when Enable Banking is configured")
	}
	address, err := mail.ParseAddress(cfg.DataProtectionEmail)
	if err != nil {
		return fmt.Errorf("invalid DATA_PROTECTION_EMAIL value: %w", err)
	}
	if address.Address != cfg.DataProtectionEmail {
		return fmt.Errorf("invalid DATA_PROTECTION_EMAIL value: expected a bare email address")
	}
	return nil
}
