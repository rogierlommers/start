package httpweb

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/http"
	"start/internal/httpmiddleware"
	"start/internal/service"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed web/index.html
var indexHTML string

//go:embed web/login.html
var loginHTML string

var loginPageTemplate = template.Must(template.New("login").Parse(loginHTML))
var homePageTemplate = template.Must(template.New("index").Parse(indexHTML))

type loginPageData struct {
	Error    string
	Next     string
	Username string
}

type handlers struct {
	svc          *service.Service
	auth         *httpmiddleware.GUIAuth
	appBuildTime string
}

type homePageData struct {
	AppBuildTime string
}

// RegisterPublic registers public HTML routes.
func RegisterPublic(router gin.IRouter, auth *httpmiddleware.GUIAuth) {
	h := handlers{auth: auth}

	router.GET("/login", h.loginForm)
	router.POST("/login", h.loginSubmit)
	router.POST("/logout", h.logout)
}

// Register registers HTML routes.
func Register(router gin.IRouter, svc *service.Service, appBuildTime string) {
	h := handlers{svc: svc, appBuildTime: appBuildTime}

	router.GET("/", h.appHome)
}

func (h handlers) appHome(c *gin.Context) {
	var buf bytes.Buffer
	if err := homePageTemplate.Execute(&buf, homePageData{AppBuildTime: h.appBuildTime}); err != nil {
		c.String(http.StatusInternalServerError, "failed to render home page")
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (h handlers) loginForm(c *gin.Context) {
	if h.auth == nil || !h.auth.Enabled() {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	if h.auth.IsAuthenticated(c) {
		c.Redirect(http.StatusSeeOther, httpmiddleware.NormalizeNextPath(c.Query("next")))
		return
	}

	h.renderLogin(c, http.StatusOK, loginPageData{
		Next: httpmiddleware.NormalizeNextPath(c.Query("next")),
	})
}

func (h handlers) loginSubmit(c *gin.Context) {
	if h.auth == nil || !h.auth.Enabled() {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	next := httpmiddleware.NormalizeNextPath(c.PostForm("next"))
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	if !h.auth.Authenticate(username, password) {
		h.renderLogin(c, http.StatusUnauthorized, loginPageData{
			Error:    "Invalid username or password.",
			Next:     next,
			Username: username,
		})
		return
	}

	h.auth.StartSession(c)
	c.Redirect(http.StatusSeeOther, next)
}

func (h handlers) logout(c *gin.Context) {
	if h.auth != nil {
		h.auth.ClearSession(c)
	}

	target := "/"
	if h.auth != nil && h.auth.Enabled() {
		target = "/login"
	}
	c.Redirect(http.StatusSeeOther, target)
}

func (h handlers) renderLogin(c *gin.Context, status int, data loginPageData) {
	var buf bytes.Buffer
	if err := loginPageTemplate.Execute(&buf, data); err != nil {
		c.String(http.StatusInternalServerError, "failed to render login page")
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}
