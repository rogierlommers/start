package httpapi

import (
	"encoding/xml"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type addReadingListItemRequest struct {
	URL   string `json:"url" binding:"required"`
	Title string `json:"title"`
}

type readingListItemResponse struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string  `xml:"title"`
	Link    string  `xml:"link"`
	GUID    rssGUID `xml:"guid"`
	PubDate string  `xml:"pubDate"`
}

type rssGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

var readingListSavedPage = template.Must(template.New("reading-list-saved").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Saved to your reading list</title>
  <style>
    :root { color-scheme: light dark; font-family: ui-sans-serif, system-ui, sans-serif; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #f4f1eb; color: #25231f; }
    main { box-sizing: border-box; width: min(92vw, 600px); padding: clamp(2rem, 8vw, 4rem); background: #fffdf9; border: 1px solid #ded8cf; border-radius: 24px; box-shadow: 0 18px 60px #5147351f; }
    .check { color: #28724f; font-size: 2rem; line-height: 1; }
    h1 { margin: 1rem 0 .6rem; font-size: clamp(1.8rem, 5vw, 2.6rem); letter-spacing: -.04em; }
    p { color: #6b655c; line-height: 1.6; }
    dl { margin: 2rem 0; padding: 1.25rem; background: #f4f1eb; border-radius: 14px; }
    dt { margin-top: 1rem; color: #81796e; font-size: .75rem; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
    dt:first-child { margin-top: 0; }
    dd { margin: .35rem 0 0; overflow-wrap: anywhere; }
    a { color: #2868a5; }
    .button { display: inline-block; padding: .8rem 1.1rem; border-radius: 10px; background: #2868a5; color: white; text-decoration: none; font-weight: 700; }
    @media (prefers-color-scheme: dark) {
      body { background: #1f211f; color: #f5f1e9; }
      main { background: #292b28; border-color: #454740; box-shadow: none; }
      p, dt { color: #b8b3a9; }
      dl { background: #20221f; }
    }
  </style>
</head>
<body>
  <main>
    <div class="check" aria-hidden="true">&#10003;</div>
    <h1>Saved to your reading list</h1>
    <p>This page was saved successfully. You can keep browsing here or open it when you are ready.</p>
    <dl>
      <dt>Title</dt>
      <dd>{{ .Title }}</dd>
      <dt>Site</dt>
      <dd>{{ .Host }}</dd>
      <dt>Saved</dt>
      <dd>{{ .SavedAt }}</dd>
      <dt>Address</dt>
      <dd><a href="{{ .URL }}">{{ .URL }}</a></dd>
    </dl>
    <a class="button" href="{{ .URL }}">Open saved page</a>
  </main>
</body>
</html>`))

type readingListSavedPageData struct {
	Title   string
	Host    string
	URL     string
	SavedAt string
}

func readingListItemToResponse(item service.ReadingListItem) readingListItemResponse {
	return readingListItemResponse{
		ID:        item.ID,
		URL:       item.URL,
		Title:     item.Title,
		CreatedAt: item.CreatedAt,
	}
}

// addReadingListItem godoc
// @Summary Add reading-list item
// @Tags reading-list
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param request body addReadingListItemRequest true "Reading-list payload"
// @Success 201 {object} readingListItemResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/reading-list/items [post]
func (h handlers) addReadingListItem(c *gin.Context) {
	var req addReadingListItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid JSON body"})
		return
	}

	item, err := h.svc.AddReadingListItem(c.Request.Context(), service.AddReadingListItemInput{
		URL:   req.URL,
		Title: req.Title,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidReadingListInput) {
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to add reading list item"})
		return
	}

	c.JSON(http.StatusCreated, readingListItemToResponse(item))
}

// addReadingListItemFromBookmarklet godoc
// @Summary Add reading-list item from bookmarklet query
// @Tags reading-list
// @Produce json
// @Param url query string true "URL to add"
// @Param return_to query string false "Deprecated; accepted for bookmarklet compatibility"
// @Success 201 {object} readingListItemResponse
// @Success 200 {string} string "Saved-item confirmation page"
// @Failure 400 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/reading-list/bookmarklet-input [get]
func (h handlers) addReadingListItemFromBookmarklet(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "url query parameter is required"})
		return
	}

	// Be lenient with pasted bookmarklet formats that may include surrounding quotes.
	rawURL = strings.Trim(rawURL, "'\"")

	item, err := h.svc.AddReadingListItem(c.Request.Context(), service.AddReadingListItemInput{URL: rawURL})
	if err != nil {
		if errors.Is(err, service.ErrInvalidReadingListInput) {
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to add reading list item"})
		return
	}

	parsed, _ := url.Parse(item.URL)
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = parsed.Host
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := readingListSavedPage.Execute(c.Writer, readingListSavedPageData{
		Title:   title,
		Host:    parsed.Host,
		URL:     item.URL,
		SavedAt: item.CreatedAt.Format("2 January 2006, 15:04 MST"),
	}); err != nil {
		// The template is static and parsed at startup; this is only a defensive response.
		return
	}
}

// listReadingListItems godoc
// @Summary List reading-list items
// @Tags reading-list
// @Produce json
// @Security ApiBasicAuth
// @Success 200 {array} readingListItemResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/reading-list/items [get]
func (h handlers) listReadingListItems(c *gin.Context) {
	items, err := h.svc.ListReadingListItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to list reading list items"})
		return
	}
	resp := make([]readingListItemResponse, len(items))
	for i, item := range items {
		resp[i] = readingListItemToResponse(item)
	}
	c.JSON(http.StatusOK, resp)
}

// getReadingListRSS godoc
// @Summary Reading-list RSS feed
// @Tags reading-list
// @Produce application/rss+xml
// @Success 200 {string} string "RSS feed XML"
// @Failure 500 {object} apiErrorResponse
// @Router /api/reading-list/rss [get]
func (h handlers) getReadingListRSS(c *gin.Context) {
	items, err := h.svc.ListReadingListItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to list reading list items"})
		return
	}

	channelItems := make([]rssItem, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = item.URL
		}

		channelItems = append(channelItems, rssItem{
			Title: title,
			Link:  item.URL,
			GUID: rssGUID{
				IsPermaLink: true,
				Value:       item.URL,
			},
			PubDate: item.CreatedAt.Format(time.RFC1123Z),
		})
	}

	baseURL := requestBaseURL(c)
	doc := rssDocument{
		Version: "2.0",
		Channel: rssChannel{
			Title:       "start reading list",
			Link:        baseURL + "/api/reading-list/rss",
			Description: "Reading list feed from start",
			Items:       channelItems,
		},
	}

	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to build rss feed"})
		return
	}

	body = append([]byte(xml.Header), body...)
	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", body)
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
