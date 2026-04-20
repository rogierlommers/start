package httpapi

import (
	"encoding/xml"
	"errors"
	"net/http"
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
