package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var configEnvKeys = []string{
	"HTTP_BIND_ADDR",
	"LOG_LEVEL",
	"SQLITE_PATH",
	"GUI_USERNAME",
	"GUI_PASSWORD",
	"GUI_SESSION_SECRET",
	"API_USERNAME",
	"API_PASSWORD",
	"STORAGE_UPLOAD_DIR",
	"STORAGE_MAX_UPLOAD_MB",
	"STORAGE_CLEANUP_DAYS",
	"READING_LIST_CLEANUP_DAYS",
	"MAILER_SMTP_HOST",
	"MAILER_SMTP_USERNAME",
	"MAILER_SMTP_PASSWORD",
	"MAILER_SMTP_FROM",
	"MAILER_EMAIL_PRIVATE",
	"MAILER_EMAIL_WORK",
	"SMTP_PORT",
	"ENABLE_ACCESS_LOGS",
}

func TestLoadDefaults(t *testing.T) {
	withConfigTestEnv(t, map[string]string{}, true, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.ShutdownTimeout != 10*time.Second {
			t.Fatalf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
		}
		if cfg.ReadHeaderTimeout != 5*time.Second {
			t.Fatalf("ReadHeaderTimeout = %v, want %v", cfg.ReadHeaderTimeout, 5*time.Second)
		}
		if cfg.StorageMaxUploadMB != 100 {
			t.Fatalf("StorageMaxUploadMB = %d, want %d", cfg.StorageMaxUploadMB, 100)
		}
		if cfg.StorageCleanupDays != 30 {
			t.Fatalf("StorageCleanupDays = %d, want %d", cfg.StorageCleanupDays, 30)
		}
		if cfg.ReadingListCleanupDays != 30 {
			t.Fatalf("ReadingListCleanupDays = %d, want %d", cfg.ReadingListCleanupDays, 30)
		}
		if cfg.SMTPPort != 587 {
			t.Fatalf("SMTPPort = %d, want %d", cfg.SMTPPort, 587)
		}
		if cfg.EnableAccessLogs {
			t.Fatal("EnableAccessLogs = true, want false")
		}
	})
}

func TestLoadParsesOverrides(t *testing.T) {
	withConfigTestEnv(t, map[string]string{
		"HTTP_BIND_ADDR":            "127.0.0.1:3000",
		"LOG_LEVEL":                 "debug",
		"SQLITE_PATH":               "  /tmp/start.db  ",
		"GUI_USERNAME":              "gui-user",
		"GUI_PASSWORD":              "gui-pass",
		"GUI_SESSION_SECRET":        "stable-session-secret-012345678901",
		"API_USERNAME":              "api-user",
		"API_PASSWORD":              "api-pass",
		"STORAGE_UPLOAD_DIR":        "/tmp/uploads",
		"STORAGE_MAX_UPLOAD_MB":     "256",
		"STORAGE_CLEANUP_DAYS":      "14",
		"READING_LIST_CLEANUP_DAYS": "7",
		"MAILER_SMTP_HOST":          "smtp.example.com",
		"MAILER_SMTP_USERNAME":      "mailer-user",
		"MAILER_SMTP_PASSWORD":      "mailer-pass",
		"MAILER_SMTP_FROM":          "start@example.com",
		"MAILER_EMAIL_PRIVATE":      "private@example.com",
		"MAILER_EMAIL_WORK":         "work@example.com",
		"SMTP_PORT":                 "2525",
		"ENABLE_ACCESS_LOGS":        "true",
	}, true, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.HostPort != "127.0.0.1:3000" {
			t.Fatalf("HostPort = %q, want %q", cfg.HostPort, "127.0.0.1:3000")
		}
		if cfg.LogLevel != "debug" {
			t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
		}
		if cfg.SQLitePath != "/tmp/start.db" {
			t.Fatalf("SQLitePath = %q, want %q", cfg.SQLitePath, "/tmp/start.db")
		}
		if cfg.GUIUsername != "gui-user" || cfg.GUIPassword != "gui-pass" {
			t.Fatalf("GUI credentials = (%q, %q), want (%q, %q)", cfg.GUIUsername, cfg.GUIPassword, "gui-user", "gui-pass")
		}
		if cfg.GUISessionSecret != "stable-session-secret-012345678901" {
			t.Fatalf("GUISessionSecret = %q, want %q", cfg.GUISessionSecret, "stable-session-secret-012345678901")
		}
		if cfg.APIUsername != "api-user" || cfg.APIPassword != "api-pass" {
			t.Fatalf("API credentials = (%q, %q), want (%q, %q)", cfg.APIUsername, cfg.APIPassword, "api-user", "api-pass")
		}
		if cfg.StorageUploadDir != "/tmp/uploads" {
			t.Fatalf("StorageUploadDir = %q, want %q", cfg.StorageUploadDir, "/tmp/uploads")
		}
		if cfg.StorageMaxUploadMB != 256 {
			t.Fatalf("StorageMaxUploadMB = %d, want %d", cfg.StorageMaxUploadMB, 256)
		}
		if cfg.StorageCleanupDays != 14 {
			t.Fatalf("StorageCleanupDays = %d, want %d", cfg.StorageCleanupDays, 14)
		}
		if cfg.ReadingListCleanupDays != 7 {
			t.Fatalf("ReadingListCleanupDays = %d, want %d", cfg.ReadingListCleanupDays, 7)
		}
		if cfg.SMTPHost != "smtp.example.com" || cfg.SMTPFrom != "start@example.com" {
			t.Fatalf("SMTP settings = (%q, %q), want (%q, %q)", cfg.SMTPHost, cfg.SMTPFrom, "smtp.example.com", "start@example.com")
		}
		if cfg.SMTPPort != 2525 {
			t.Fatalf("SMTPPort = %d, want %d", cfg.SMTPPort, 2525)
		}
		if !cfg.EnableAccessLogs {
			t.Fatal("EnableAccessLogs = false, want true")
		}
	})
}

func TestLoadReturnsErrorWhenDotEnvMissing(t *testing.T) {
	withConfigTestEnv(t, map[string]string{}, false, func() {
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want missing .env error")
		}
	})
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "invalid smtp port", env: map[string]string{"SMTP_PORT": "abc"}},
		{name: "non-positive smtp port", env: map[string]string{"SMTP_PORT": "0"}},
		{name: "invalid upload size", env: map[string]string{"STORAGE_MAX_UPLOAD_MB": "abc"}},
		{name: "non-positive upload size", env: map[string]string{"STORAGE_MAX_UPLOAD_MB": "0"}},
		{name: "invalid storage cleanup", env: map[string]string{"STORAGE_CLEANUP_DAYS": "-1"}},
		{name: "invalid reading list cleanup", env: map[string]string{"READING_LIST_CLEANUP_DAYS": "-1"}},
		{name: "invalid access logs", env: map[string]string{"ENABLE_ACCESS_LOGS": "maybe"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withConfigTestEnv(t, tt.env, true, func() {
				if _, err := Load(); err == nil {
					t.Fatal("Load() error = nil, want error")
				}
			})
		})
	}
}

func withConfigTestEnv(t *testing.T, env map[string]string, createDotEnv bool, fn func()) {
	t.Helper()

	for _, key := range configEnvKeys {
		t.Setenv(key, "")
	}
	for key, value := range env {
		t.Setenv(key, value)
	}

	dir := t.TempDir()
	if createDotEnv {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(.env) error = %v", err)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	fn()
}
