package httpmiddleware

import (
	"net/http"
	"net/http/httptest"
	"start/internal/config"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNormalizeNextPath(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty defaults home", raw: "", want: "/"},
		{name: "relative path rejected", raw: "dashboard", want: "/"},
		{name: "absolute url rejected", raw: "https://example.com/", want: "/"},
		{name: "double slash rejected", raw: "//example.com", want: "/"},
		{name: "query preserved", raw: "/login?next=%2Fapi", want: "/login?next=%2Fapi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeNextPath(tt.raw); got != tt.want {
				t.Fatalf("NormalizeNextPath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestRequireAuthRedirectsHTMLRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &GUIAuth{enabled: true, secret: []byte("01234567890123456789012345678901")}

	router := gin.New()
	router.Use(auth.RequireAuth())
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/login" {
		t.Fatalf("location = %q, want %q", got, "/login")
	}
}

func TestRequireAuthRejectsAPIRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &GUIAuth{enabled: true, secret: []byte("01234567890123456789012345678901")}

	router := gin.New()
	router.Use(auth.RequireAuth())
	router.GET("/api/bookmarks", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/bookmarks", nil)
	req.Header.Set("Accept", "*/*")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatalf("WWW-Authenticate header = %q, want non-empty", got)
	}
}

func TestRequireAuthAllowsValidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &GUIAuth{enabled: true, secret: []byte("01234567890123456789012345678901")}

	router := gin.New()
	router.Use(auth.RequireAuth())
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: guiSessionCookieName, Value: auth.sessionToken()})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireAuthAllowsValidAPIBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &GUIAuth{
		enabled: true,
		apiUser: "api-user",
		apiPass: "api-pass",
		secret:  []byte("01234567890123456789012345678901"),
	}

	router := gin.New()
	router.Use(auth.RequireAuth())
	router.GET("/api/bookmarks", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/bookmarks", nil)
	req.SetBasicAuth("api-user", "api-pass")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestNewGUIAuthUsesConfiguredSessionSecret(t *testing.T) {
	auth, err := NewGUIAuth(config.Config{
		GUIUsername:      "user",
		GUIPassword:      "pass",
		GUISessionSecret: "01234567890123456789012345678901",
	})
	if err != nil {
		t.Fatalf("NewGUIAuth() error = %v", err)
	}

	other, err := NewGUIAuth(config.Config{
		GUIUsername:      "user",
		GUIPassword:      "pass",
		GUISessionSecret: "01234567890123456789012345678901",
	})
	if err != nil {
		t.Fatalf("NewGUIAuth() second call error = %v", err)
	}

	if got, want := auth.sessionToken(), other.sessionToken(); got != want {
		t.Fatalf("sessionToken() = %q, want stable token %q", got, want)
	}
}

func TestNewGUIAuthRejectsShortSessionSecret(t *testing.T) {
	_, err := NewGUIAuth(config.Config{
		GUIUsername:      "user",
		GUIPassword:      "pass",
		GUISessionSecret: "too-short",
	})
	if err == nil {
		t.Fatal("NewGUIAuth() error = nil, want error")
	}
}
