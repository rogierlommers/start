package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"start/internal/config"
	"start/internal/mailer"
	"start/internal/repository"
	"start/internal/service"

	"github.com/gin-gonic/gin"
)

func TestDetermineRecipientBodyAndSubject(t *testing.T) {
	cfg := config.Config{
		MailerEmailPrivate: "private@example.com",
		MailerEmailWork:    "work@example.com",
	}

	to, body, subject := deterimeRecipientBodyAndSubject(cfg, "  hello\nsecond")
	if to != "private@example.com" || body != "hello\nsecond" || subject != "hello" {
		t.Fatalf("private routing = (%q, %q, %q)", to, body, subject)
	}

	to, body, subject = deterimeRecipientBodyAndSubject(cfg, "  w finish report\nnow")
	if to != "work@example.com" || body != "finish report\nnow" || subject != "finish report" {
		t.Fatalf("work routing = (%q, %q, %q)", to, body, subject)
	}
}

func TestDeriveSubjectTrimsAndTruncates(t *testing.T) {
	if got := deriveSubject("  \n first line \nsecond"); got != "first line" {
		t.Fatalf("deriveSubject() = %q, want %q", got, "first line")
	}

	long := strings.Repeat("a", generatedSubjectMaxLen+5)
	if got := deriveSubject(long); got != strings.Repeat("a", generatedSubjectMaxLen) {
		t.Fatalf("deriveSubject(long) len = %d, want %d", len(got), generatedSubjectMaxLen)
	}
}

func TestCategoryAndBookmarkHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := newAPITestRouter(t)

	rec := performJSONRequest(router, http.MethodPost, "/api/categories", `{"name":"General"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category status = %d, want %d", rec.Code, http.StatusCreated)
	}

	rec = performJSONRequest(router, http.MethodPost, "/api/categories", `{"name":"General"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate category status = %d, want %d", rec.Code, http.StatusConflict)
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/categories", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list categories status = %d, want %d", rec.Code, http.StatusOK)
	}

	rec = performJSONRequest(router, http.MethodPost, "/api/bookmarks", `{"url":"https://example.com","title":"Example","tag":"work","category_id":1}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create bookmark status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"tag":"work"`) {
		t.Fatalf("create bookmark body = %q, want tag", rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodPost, "/api/bookmarks", `{"url":"https://example.com","title":"Duplicate","category_id":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate bookmark status = %d, want %d", rec.Code, http.StatusConflict)
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/bookmarks", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://example.com") {
		t.Fatalf("list bookmarks response = status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodPatch, "/api/bookmarks/1", `{"url":"https://example.org","title":"Updated","tag":"reference","category_id":1}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Updated") || !strings.Contains(rec.Body.String(), `"tag":"reference"`) {
		t.Fatalf("update bookmark response = status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodPatch, "/api/bookmarks/1/hidden", `{"hidden":true}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hidden":true`) {
		t.Fatalf("toggle hidden response = status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/bookmarks", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "example.org") {
		t.Fatalf("hidden bookmark should be excluded, status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/bookmarks?include_hidden=true", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "example.org") || !strings.Contains(rec.Body.String(), `"tag":"reference"`) {
		t.Fatalf("include_hidden response = status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/bookmarks/alfred?include_hidden=true", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"arg":"https://example.org"`) {
		t.Fatalf("alfred response = status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodPatch, "/api/bookmarks/reorder", `{"ids":[1]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, want %d", rec.Code, http.StatusOK)
	}

	rec = performJSONRequest(router, http.MethodDelete, "/api/bookmarks/1", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestBookmarkHandlersValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := newAPITestRouter(t)

	rec := performJSONRequest(router, http.MethodPost, "/api/categories", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid category status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performJSONRequest(router, http.MethodPost, "/api/bookmarks", `{"url":"notaurl","category_id":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid bookmark status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performJSONRequest(router, http.MethodPatch, "/api/bookmarks/not-a-number", `{"url":"https://example.com","title":"x","category_id":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performJSONRequest(router, http.MethodPatch, "/api/bookmarks/reorder", `{"ids":[]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid reorder status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performJSONRequest(router, http.MethodPatch, "/api/bookmarks/99/hidden", `{"hidden":true}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing bookmark toggle status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	rec = performJSONRequest(router, http.MethodDelete, "/api/bookmarks/not-a-number", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete invalid id status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performJSONRequest(router, http.MethodDelete, "/api/bookmarks/999", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("delete missing bookmark status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestReadingListHandlersAndRSS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := newAPITestRouter(t)

	rec := performJSONRequest(router, http.MethodPost, "/api/reading-list/items", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid reading list item status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performJSONRequest(router, http.MethodPost, "/api/reading-list/items", `{"url":"https://example.com/article","title":"Article"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add reading list item status = %d, want %d", rec.Code, http.StatusCreated)
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/reading-list/items", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Article") {
		t.Fatalf("list reading items response = status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/reading-list/rss", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<rss") || !strings.Contains(rec.Body.String(), "start reading list") {
		t.Fatalf("rss response = status %d body %q", rec.Code, rec.Body.String())
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/reading-list/bookmarklet-input", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing bookmarklet url status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performJSONRequest(router, http.MethodGet, "/api/reading-list/bookmarklet-input?url='https://example.net'&return_to=https://reader.example.net/back", "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("bookmarklet redirect status = %d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); !strings.Contains(got, "reading_list_saved=1") {
		t.Fatalf("bookmarklet redirect location = %q, want reading_list_saved marker", got)
	}
}

func TestOpenAPISpecRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)

	handlers{}.openapiSpec(c)

	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Fatalf("openapi status = %d, want %d or %d", rec.Code, http.StatusOK, http.StatusNotFound)
	}
}

func TestSendMailHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disabledRouter, _ := newAPITestRouter(t)

	rec := performMultipartRequest(t, disabledRouter, http.MethodPost, "/api/mail/send", map[string]string{
		"body":    "hello",
		"subject": "subject",
		"to":      "person@example.com",
	}, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled mailer status = %d, want %d body=%q", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	enabledRouter, _ := newAPITestRouterWithSender(t, noopMailSender{})
	rec = performMultipartRequest(t, enabledRouter, http.MethodPost, "/api/mail/send", map[string]string{
		"body": "w finish quarterly report",
	}, map[string]string{"attachments": "note.txt:hello world"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("accepted mail status = %d, want %d body=%q", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"accepted"`) {
		t.Fatalf("accepted mail body = %q", rec.Body.String())
	}
}

func TestStorageHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := newAPITestRouter(t)

	// missing file field
	rec := performMultipartRequest(t, router, http.MethodPost, "/api/storage/upload", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing file status = %d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = performMultipartRequest(t, router, http.MethodPost, "/api/storage/upload", nil, map[string]string{"file": "one.txt:hello"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("single upload status = %d, want %d body=%q", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// duplicate
	rec = performMultipartRequest(t, router, http.MethodPost, "/api/storage/uploads", nil, map[string]string{
		"files": "one.txt:again",
	})
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("duplicate multi upload status = %d, want %d body=%q", rec.Code, http.StatusMultiStatus, rec.Body.String())
	}

	rec = performMultipartRequest(t, router, http.MethodPost, "/api/storage/uploads", nil, map[string]string{
		"files": "two.txt:world",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("second multi upload status = %d, want %d", rec.Code, http.StatusCreated)
	}

	// too many files
	rec = performMultipartFilesRequest(t, router, http.MethodPost, "/api/storage/uploads", maxUploadFilesPerRequest+1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("too many files status = %d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	// list
	rec = performJSONRequest(router, http.MethodGet, "/api/storage/files", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "one.txt") || !strings.Contains(rec.Body.String(), "two.txt") {
		t.Fatalf("list storage files response = status %d body %q", rec.Code, rec.Body.String())
	}

	// download regular file (attachment)
	rec = performJSONRequest(router, http.MethodGet, "/api/storage/files/one.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("download storage file status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "one.txt") {
		t.Fatalf("Content-Disposition = %q, want attachment header", got)
	}

	// download missing file
	rec = performJSONRequest(router, http.MethodGet, "/api/storage/files/nope.txt", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing file status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// download file with path traversal
	rec = performJSONRequest(router, http.MethodGet, "/api/storage/files/%2F%2F..%2Fetc%2Fpasswd", "")
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("path traversal status = %d, want bad request or not found", rec.Code)
	}
}

func TestStorageHandlerImageInline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := newAPITestRouter(t)

	// Upload a PNG file (the data will be tiny, content-type from extension).
	rec := performMultipartRequest(t, router, http.MethodPost, "/api/storage/upload", nil, map[string]string{"file": "photo.png:PNG"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload png status = %d, want %d body=%q", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// Downloading an image should serve it inline via c.File (no Content-Disposition attachment).
	rec = performJSONRequest(router, http.MethodGet, "/api/storage/files/photo.png", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("download png status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Disposition"); strings.Contains(got, "attachment") {
		t.Fatalf("Content-Disposition = %q, image should be served inline", got)
	}
}

type noopMailSender struct{}

func (noopMailSender) Send(_ context.Context, _ mailer.Message) error {
	return nil
}

func newAPITestRouter(t *testing.T) (*gin.Engine, config.Config) {
	t.Helper()
	return newAPITestRouterWithSender(t, mailer.DisabledSender{})
}

func newAPITestRouterWithSender(t *testing.T, sender mailer.Sender) (*gin.Engine, config.Config) {
	t.Helper()

	cfg := config.Config{
		StorageUploadDir:   t.TempDir(),
		StorageMaxUploadMB: 5,
		MailerEmailPrivate: "private@example.com",
		MailerEmailWork:    "work@example.com",
	}

	svc := service.New(repository.NewMemoryStore(), sender, cfg)
	router := gin.New()
	Register(router, svc, cfg)
	RegisterPublic(router, svc, cfg)

	t.Cleanup(func() {
		svc.Close()
	})

	return router, cfg
}

func performJSONRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var reqBody *strings.Reader
	if body == "" {
		reqBody = strings.NewReader("")
	} else {
		reqBody = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// performMultipartFilesRequest sends n distinct files under the "files" field to test limits.
func performMultipartFilesRequest(t *testing.T, router *gin.Engine, method, path string, n int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for i := 0; i < n; i++ {
		part, err := writer.CreateFormFile("files", fmt.Sprintf("file%d.txt", i))
		if err != nil {
			t.Fatalf("CreateFormFile(%d) error = %v", i, err)
		}
		if _, err := part.Write([]byte("data")); err != nil {
			t.Fatalf("Write(%d) error = %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func performMultipartRequest(t *testing.T, router *gin.Engine, method, path string, fields map[string]string, files map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField(%q) error = %v", key, err)
		}
	}
	for field, spec := range files {
		parts := strings.SplitN(spec, ":", 2)
		filename := parts[0]
		content := ""
		if len(parts) == 2 {
			content = parts[1]
		}
		part, err := writer.CreateFormFile(field, filename)
		if err != nil {
			t.Fatalf("CreateFormFile(%q) error = %v", field, err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("Write file content error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart writer close error = %v", err)
	}

	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
