package httpmiddleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"start/internal/config"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	guiSessionCookieName = "start_gui_session"
	guiSessionPayload    = "gui"
	guiSessionTTL        = 7 * 24 * time.Hour
)

// GUIAuth provides cookie-backed authentication for the dashboard and API.
type GUIAuth struct {
	enabled  bool
	username string
	password string
	apiUser  string
	apiPass  string
	secret   []byte
}

// NewGUIAuth builds a GUI auth helper from runtime configuration.
func NewGUIAuth(cfg config.Config) (*GUIAuth, error) {
	username := strings.TrimSpace(cfg.GUIUsername)
	password := cfg.GUIPassword
	if username == "" || password == "" {
		logrus.Warn("GUI auth is disabled because GUI_USERNAME or GUI_PASSWORD is not configured")
		return &GUIAuth{}, nil
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate GUI auth secret: %w", err)
	}

	return &GUIAuth{
		enabled:  true,
		username: username,
		password: password,
		apiUser:  strings.TrimSpace(cfg.APIUsername),
		apiPass:  cfg.APIPassword,
		secret:   secret,
	}, nil
}

// Enabled reports whether GUI authentication is active.
func (a *GUIAuth) Enabled() bool {
	return a != nil && a.enabled
}

// Authenticate validates submitted login credentials.
func (a *GUIAuth) Authenticate(username, password string) bool {
	if !a.Enabled() {
		return true
	}

	return hmac.Equal([]byte(strings.TrimSpace(username)), []byte(a.username)) &&
		hmac.Equal([]byte(password), []byte(a.password))
}

// IsAuthenticated reports whether the request carries a valid GUI session cookie.
func (a *GUIAuth) IsAuthenticated(c *gin.Context) bool {
	if !a.Enabled() {
		return true
	}

	rawToken, err := c.Cookie(guiSessionCookieName)
	if err != nil || rawToken == "" {
		return false
	}

	return a.validateSessionToken(rawToken)
}

// StartSession attaches an authenticated session cookie to the response.
func (a *GUIAuth) StartSession(c *gin.Context) {
	if !a.Enabled() {
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(guiSessionCookieName, a.sessionToken(), int(guiSessionTTL.Seconds()), "/", "", c.Request.TLS != nil, true)
}

// ClearSession removes the GUI session cookie.
func (a *GUIAuth) ClearSession(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(guiSessionCookieName, "", -1, "/", "", c.Request.TLS != nil, true)
}

// LoginURL returns the login route with a safe post-login redirect target.
func (a *GUIAuth) LoginURL(next string) string {
	target := NormalizeNextPath(next)
	if target == "/" {
		return "/login"
	}

	return "/login?next=" + url.QueryEscape(target)
}

// RequireAuth enforces GUI authentication on protected routes.
func (a *GUIAuth) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.IsAuthenticated(c) {
			c.Next()
			return
		}

		if a.isAPIBasicAuthorized(c.Request) {
			c.Next()
			return
		}

		if expectsHTML(c.Request) {
			c.Redirect(http.StatusSeeOther, a.LoginURL(c.Request.URL.RequestURI()))
			c.Abort()
			return
		}

		if expectsAPIAuth(c.Request) {
			c.Header("WWW-Authenticate", `Basic realm="start-api", charset="UTF-8"`)
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
	}
}

// NormalizeNextPath reduces a user-provided redirect target to an internal path.
func NormalizeNextPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "//") {
		return "/"
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() {
		return "/"
	}

	target := parsed.Path
	if target == "" {
		target = "/"
	}
	if !strings.HasPrefix(target, "/") {
		return "/"
	}
	if parsed.RawQuery != "" {
		target += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		target += "#" + parsed.Fragment
	}

	return target
}

func expectsHTML(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "text/html")
}

func expectsAPIAuth(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/openapi.yaml"
}

func (a *GUIAuth) isAPIBasicAuthorized(r *http.Request) bool {
	if !a.Enabled() || !expectsAPIAuth(r) {
		return false
	}

	if strings.TrimSpace(a.apiUser) == "" || a.apiPass == "" {
		return false
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	return hmac.Equal([]byte(strings.TrimSpace(username)), []byte(a.apiUser)) &&
		hmac.Equal([]byte(password), []byte(a.apiPass))
}

func (a *GUIAuth) sessionToken() string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(guiSessionPayload))
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(guiSessionPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func (a *GUIAuth) validateSessionToken(rawToken string) bool {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 2 {
		return false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || string(payload) != guiSessionPayload {
		return false
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, a.secret)
	mac.Write(payload)
	return hmac.Equal(signature, mac.Sum(nil))
}
