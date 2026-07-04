package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"start/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// maxCallbackBodyBytes caps how much of the incoming request body is read into
// the report to avoid unbounded memory use from untrusted callers.
const maxCallbackBodyBytes = 1 * 1024 * 1024 // 1MB

// callBack godoc
// @Summary Receive a callback and email a report of the incoming request
// @Description Emails a report containing all incoming request headers and the request body to the configured private address.
// @Tags bimcollab
// @Produce json
// @Success 200 {object} healthResponse
// @Router /api/bimcollab-callback [get]
// @Router /api/bimcollab-callback [post]
// @Router /api/bimcollab-callback [put]
func (h handlers) callBack(c *gin.Context) {
	report := buildRequestReport(c)

	// Send the report by email (non-blocking; queued by the service layer).
	if _, err := h.svc.SendMail(c.Request.Context(), service.SendMailInput{
		To:      h.cfg.MailerEmailPrivate,
		Subject: fmt.Sprintf("Incoming request report: %s %s", c.Request.Method, c.Request.URL.Path),
		Body:    report,
	}); err != nil {
		// Log but still return OK so the caller is not affected by mail issues.
		logrus.Errorf("failed to queue request report email: %v", err)
	}

	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

// buildRequestReport formats the incoming request's headers and body into a
// human-readable plain-text report for emailing.
func buildRequestReport(c *gin.Context) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Method: %s\n", c.Request.Method)
	fmt.Fprintf(&b, "URL: %s\n", c.Request.URL.String())
	fmt.Fprintf(&b, "Remote address: %s\n", c.ClientIP())

	b.WriteString("\nHeaders:\n")
	names := make([]string, 0, len(c.Request.Header))
	for name := range c.Request.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, value := range c.Request.Header[name] {
			fmt.Fprintf(&b, "  %s: %s\n", name, value)
		}
	}

	b.WriteString("\nBody:\n")
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxCallbackBodyBytes))
	if err != nil {
		logrus.Errorf("failed to read request body for report: %v", err)
		b.WriteString("  <failed to read request body>\n")
	} else if len(body) == 0 {
		b.WriteString("  <empty>\n")
	} else {
		b.Write(body)
		b.WriteString("\n")
	}

	return b.String()
}
