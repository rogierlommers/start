package httpmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"start/internal/config"

	"github.com/gin-gonic/gin"
)

func TestRegisterGlobalAddsSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterGlobal(router)
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want %q", got, "DENY")
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q, want %q", got, "no-referrer")
	}
}

func TestAuthenticateAndSessionCookieLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth, err := NewGUIAuth(config.Config{
		GUIUsername:      "user",
		GUIPassword:      "pass",
		GUISessionSecret: "01234567890123456789012345678901",
	})
	if err != nil {
		t.Fatalf("NewGUIAuth() error = %v", err)
	}

	if !auth.Authenticate(" user ", "pass") {
		t.Fatal("Authenticate(valid) = false, want true")
	}
	if auth.Authenticate("user", "wrong") {
		t.Fatal("Authenticate(invalid) = true, want false")
	}

	router := gin.New()
	router.GET("/start", func(c *gin.Context) {
		auth.StartSession(c)
		c.Status(http.StatusNoContent)
	})
	router.GET("/clear", func(c *gin.Context) {
		auth.ClearSession(c)
		c.Status(http.StatusNoContent)
	})
	router.GET("/check", func(c *gin.Context) {
		if auth.IsAuthenticated(c) {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusUnauthorized)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/start", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("/start status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("/start no cookies set")
	}

	checkReq := httptest.NewRequest(http.MethodGet, "/check", nil)
	checkReq.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, checkReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("/check status = %d, want %d", rec.Code, http.StatusOK)
	}

	bogusReq := httptest.NewRequest(http.MethodGet, "/check", nil)
	bogusReq.AddCookie(&http.Cookie{Name: guiSessionCookieName, Value: "bogus"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, bogusReq)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/check bogus cookie status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/clear", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("/clear status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	cleared := rec.Result().Cookies()
	if len(cleared) == 0 || cleared[0].MaxAge != -1 {
		t.Fatalf("clear cookie max-age = %d, want -1", firstCookieMaxAge(cleared))
	}
}

func firstCookieMaxAge(cookies []*http.Cookie) int {
	if len(cookies) == 0 {
		return 0
	}
	return cookies[0].MaxAge
}
