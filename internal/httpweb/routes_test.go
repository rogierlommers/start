package httpweb

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"start/internal/config"
	"start/internal/httpmiddleware"

	"github.com/gin-gonic/gin"
)

func TestLoginFormRedirectsWhenAuthDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPublic(router, nil)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want %q", got, "/")
	}
}

func TestLoginFormRendersForUnauthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := mustNewTestAuth(t)
	router := gin.New()
	RegisterPublic(router, auth)

	req := httptest.NewRequest(http.MethodGet, "/login?next=/docs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
	if body := rec.Body.String(); !strings.Contains(body, "Sign in to start.") || !strings.Contains(body, `name="next" value="/docs"`) {
		t.Fatalf("response body missing login page markers: %q", body)
	}
}

func TestLoginFormRedirectsAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := mustNewTestAuth(t)
	router := gin.New()
	RegisterPublic(router, auth)

	req := httptest.NewRequest(http.MethodGet, "/login?next=/docs", nil)
	req.AddCookie(&http.Cookie{Name: "start_gui_session", Value: authSessionToken(t, auth)})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/docs" {
		t.Fatalf("Location = %q, want %q", got, "/docs")
	}
}

func TestLoginSubmitRejectsInvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := mustNewTestAuth(t)
	router := gin.New()
	RegisterPublic(router, auth)

	form := url.Values{
		"username": {"wrong-user"},
		"password": {"wrong-pass"},
		"next":     {"/docs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid username or password.") {
		t.Fatalf("body missing invalid credentials error: %q", body)
	}
	if !strings.Contains(body, `value="wrong-user"`) {
		t.Fatalf("body missing echoed username: %q", body)
	}
}

func TestLoginSubmitStartsSessionOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := mustNewTestAuth(t)
	router := gin.New()
	RegisterPublic(router, auth)

	form := url.Values{
		"username": {"user"},
		"password": {"pass"},
		"next":     {"/docs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/docs" {
		t.Fatalf("Location = %q, want %q", got, "/docs")
	}
	cookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "start_gui_session=") || !strings.Contains(cookie, "HttpOnly") {
		t.Fatalf("Set-Cookie = %q, want session cookie", cookie)
	}
}

func TestLogoutRedirectsToLoginAndClearsSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := mustNewTestAuth(t)
	router := gin.New()
	RegisterPublic(router, auth)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want %q", got, "/login")
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "start_gui_session=") {
		t.Fatalf("Set-Cookie = %q, want cleared session cookie", cookie)
	}
}

func TestLogoutRedirectsHomeWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPublic(router, nil)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want %q", got, "/")
	}
}

func mustNewTestAuth(t *testing.T) *httpmiddleware.GUIAuth {
	t.Helper()

	auth, err := httpmiddleware.NewGUIAuth(config.Config{
		GUIUsername:      "user",
		GUIPassword:      "pass",
		GUISessionSecret: "01234567890123456789012345678901",
	})
	if err != nil {
		t.Fatalf("NewGUIAuth() error = %v", err)
	}

	return auth
}

func authSessionToken(t *testing.T, auth *httpmiddleware.GUIAuth) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	auth.StartSession(c)

	setCookie := rec.Header().Get("Set-Cookie")
	parts := strings.SplitN(setCookie, ";", 2)
	if len(parts) == 0 {
		t.Fatalf("Set-Cookie = %q, want cookie", setCookie)
	}
	nameValue := strings.SplitN(parts[0], "=", 2)
	if len(nameValue) != 2 {
		t.Fatalf("cookie pair = %q, want name=value", parts[0])
	}

	return nameValue[1]
}
