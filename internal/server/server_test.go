package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"start/internal/config"

	"github.com/gin-gonic/gin"
)

func TestNewHTTPServerBuildsRouterAndServesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, err := NewHTTPServer(config.Config{
		HostPort:               "127.0.0.1:0",
		ReadHeaderTimeout:      2 * time.Second,
		SQLitePath:             filepath.Join(t.TempDir(), "start.db"),
		GUIUsername:            "user",
		GUIPassword:            "pass",
		GUISessionSecret:       "01234567890123456789012345678901",
		StorageCleanupDays:     0,
		ReadingListCleanupDays: 0,
	}, "build-123")
	if err != nil {
		t.Fatalf("NewHTTPServer() error = %v", err)
	}
	t.Cleanup(func() {
		ctx.Service.Close()
	})

	if ctx.Server.Addr != "127.0.0.1:0" {
		t.Fatalf("Server.Addr = %q, want %q", ctx.Server.Addr, "127.0.0.1:0")
	}
	if ctx.Server.ReadHeaderTimeout != 2*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", ctx.Server.ReadHeaderTimeout, 2*time.Second)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	ctx.Server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}

	req = httptest.NewRequest(http.MethodGet, "/login", nil)
	rec = httptest.NewRecorder()
	ctx.Server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /login status = %d, want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	rec = httptest.NewRecorder()
	ctx.Server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/login" {
		t.Fatalf("GET / Location = %q, want %q", got, "/login")
	}
}

func TestNewHTTPServerRejectsInvalidGUISecret(t *testing.T) {
	_, err := NewHTTPServer(config.Config{
		SQLitePath:       filepath.Join(t.TempDir(), "start.db"),
		GUIUsername:      "user",
		GUIPassword:      "pass",
		GUISessionSecret: "short",
	}, "build-123")
	if err == nil {
		t.Fatal("NewHTTPServer() error = nil, want invalid GUI secret error")
	}
}
