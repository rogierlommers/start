package httpapi

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type categoryResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type createCategoryRequest struct {
	Name string `json:"name" binding:"required"`
}

type bookmarkResponse struct {
	ID         int64     `json:"id"`
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	Tag        string    `json:"tag,omitempty"`
	CategoryID int64     `json:"category_id"`
	Position   int       `json:"position"`
	Hidden     bool      `json:"hidden"`
	CreatedAt  time.Time `json:"created_at"`
}

type createBookmarkRequest struct {
	URL        string `json:"url"             binding:"required"`
	Title      string `json:"title"`
	Tag        string `json:"tag"`
	CategoryID int64  `json:"category_id"     binding:"required"`
}

type bookmarkCSVRequest struct {
	Content string `json:"content"`
}

type bookmarkCSVResponse struct {
	Content string `json:"content"`
}

type updateBookmarkRequest struct {
	URL        string `json:"url"             binding:"required"`
	Title      string `json:"title"`
	Tag        string `json:"tag"`
	CategoryID int64  `json:"category_id"     binding:"required"`
}

type reorderBookmarksRequest struct {
	IDs []int64 `json:"ids" binding:"required"`
}

type reorderBookmarksResponse struct {
	Status string `json:"status"`
}

type toggleBookmarkHiddenRequest struct {
	Hidden bool `json:"hidden"`
}

type alfredCacheResponse struct {
	Seconds int `json:"seconds"`
}

type alfredBookmarkItemResponse struct {
	UID   string `json:"uid"`
	ID    int64  `json:"id,omitempty"`
	Title string `json:"title"`
	Arg   string `json:"arg"`
}

type alfredBookmarksResponse struct {
	Cache alfredCacheResponse          `json:"cache"`
	Items []alfredBookmarkItemResponse `json:"items"`
}

func bookmarkToResponse(b service.Bookmark) bookmarkResponse {
	return bookmarkResponse{
		ID:         b.ID,
		URL:        b.URL,
		Title:      b.Title,
		Tag:        b.Tag,
		CategoryID: b.CategoryID,
		Position:   b.Position,
		Hidden:     b.Hidden,
		CreatedAt:  b.CreatedAt,
	}
}

// listCategories godoc
// @Summary List categories
// @Tags bookmarks
// @Produce json
// @Security ApiBasicAuth
// @Success 200 {array} categoryResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/categories [get]
func (h handlers) listCategories(c *gin.Context) {
	categories, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to list categories"})
		return
	}

	resp := make([]categoryResponse, len(categories))
	for i, cat := range categories {
		resp[i] = categoryResponse{ID: cat.ID, Name: cat.Name}
	}

	c.JSON(http.StatusOK, resp)
}

// createCategory godoc
// @Summary Create a category
// @Tags bookmarks
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param request body createCategoryRequest true "Category payload"
// @Success 201 {object} categoryResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 409 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/categories [post]
func (h handlers) createCategory(c *gin.Context) {
	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid JSON body"})
		return
	}

	cat, err := h.svc.CreateCategory(c.Request.Context(), service.CreateCategoryInput{Name: req.Name})
	if err != nil {
		if errors.Is(err, service.ErrInvalidCategoryInput) {
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		if errors.Is(err, service.ErrCategoryAlreadyExists) {
			c.JSON(http.StatusConflict, apiErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, categoryResponse{ID: cat.ID, Name: cat.Name})
}

// listBookmarks godoc
// @Summary List bookmarks
// @Tags bookmarks
// @Produce json
// @Security ApiBasicAuth
// @Success 200 {array} bookmarkResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks [get]
func (h handlers) listBookmarks(c *gin.Context) {
	includeHidden := c.Query("include_hidden") == "true"
	bookmarks, err := h.svc.ListBookmarks(c.Request.Context(), includeHidden)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to list bookmarks"})
		return
	}

	resp := make([]bookmarkResponse, len(bookmarks))
	for i, b := range bookmarks {
		resp[i] = bookmarkToResponse(b)
	}

	c.JSON(http.StatusOK, resp)
}

// getBookmarkCSV godoc
// @Summary Get additional bookmark CSV text
// @Tags bookmarks
// @Produce json
// @Security ApiBasicAuth
// @Success 200 {object} bookmarkCSVResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmark-csv [get]
func (h handlers) getBookmarkCSV(c *gin.Context) {
	content, err := h.svc.GetBookmarkCSV(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to get bookmark CSV"})
		return
	}

	c.JSON(http.StatusOK, bookmarkCSVResponse{Content: content})
}

// saveBookmarkCSV godoc
// @Summary Save additional bookmark CSV text
// @Tags bookmarks
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param request body bookmarkCSVRequest true "Bookmark CSV payload"
// @Success 200 {object} bookmarkCSVResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmark-csv [put]
func (h handlers) saveBookmarkCSV(c *gin.Context) {
	var req bookmarkCSVRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid JSON body"})
		return
	}

	if err := h.svc.SaveBookmarkCSV(c.Request.Context(), req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to save bookmark CSV"})
		return
	}

	c.JSON(http.StatusOK, bookmarkCSVResponse{Content: req.Content})
}

// listBookmarksAlfred godoc
// @Summary List bookmarks in Alfred workflow format
// @Tags bookmarks
// @Produce json
// @Security ApiBasicAuth
// @Param include_hidden query bool false "Include hidden bookmarks"
// @Success 200 {object} alfredBookmarksResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks/alfred [get]
func (h handlers) listBookmarksAlfred(c *gin.Context) {
	includeHidden := c.Query("include_hidden") == "true"
	bookmarks, err := h.svc.ListBookmarks(c.Request.Context(), includeHidden)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to list bookmarks"})
		return
	}

	items := make([]alfredBookmarkItemResponse, 0, len(bookmarks))
	for _, b := range bookmarks {
		title := strings.TrimSpace(b.Title)
		if title == "" {
			title = b.URL
		}
		tag := strings.TrimSpace(b.Tag)
		if tag != "" {
			title = tag + " - " + title
		}

		sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(b.URL))))
		items = append(items, alfredBookmarkItemResponse{
			UID:   hex.EncodeToString(sum[:]),
			ID:    b.ID,
			Title: title,
			Arg:   b.URL,
		})
	}

	content, err := h.svc.GetBookmarkCSV(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to get bookmark CSV"})
		return
	}
	items = append(items, alfredBookmarkCSVItems(content)...)

	c.JSON(http.StatusOK, alfredBookmarksResponse{
		Cache: alfredCacheResponse{Seconds: 3600},
		Items: items,
	})
}

func alfredBookmarkCSVItems(content string) []alfredBookmarkItemResponse {
	reader := csv.NewReader(strings.NewReader(content))
	reader.Comment = '#'
	reader.FieldsPerRecord = 2

	items := make([]alfredBookmarkItemResponse, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return items
		}
		if err != nil {
			return items
		}

		tag := strings.TrimSpace(record[0])
		url := strings.TrimSpace(record[1])
		if url == "" {
			continue
		}

		title := url
		if tag != "" {
			title = tag + " - " + url
		}
		sum := sha256.Sum256([]byte("bookmark-csv\x00" + tag + "\x00" + url))
		items = append(items, alfredBookmarkItemResponse{
			UID:   hex.EncodeToString(sum[:]),
			Title: title,
			Arg:   url,
		})
	}
}

// createBookmark godoc
// @Summary Create a bookmark
// @Tags bookmarks
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param request body createBookmarkRequest true "Bookmark payload"
// @Success 201 {object} bookmarkResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 409 {object} apiErrorResponse
// @Failure 422 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks [post]
func (h handlers) createBookmark(c *gin.Context) {
	var req createBookmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid JSON body"})
		return
	}

	b, err := h.svc.CreateBookmark(c.Request.Context(), service.CreateBookmarkInput{
		URL:        req.URL,
		Title:      req.Title,
		Tag:        req.Tag,
		CategoryID: req.CategoryID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBookmarkInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrBookmarkAlreadyExists):
			c.JSON(http.StatusConflict, apiErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrCategoryNotFound):
			c.JSON(http.StatusUnprocessableEntity, apiErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to create bookmark"})
		}
		return
	}

	c.JSON(http.StatusCreated, bookmarkToResponse(b))
}

// updateBookmark godoc
// @Summary Update a bookmark
// @Tags bookmarks
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param id path int true "Bookmark ID"
// @Param request body updateBookmarkRequest true "Bookmark payload"
// @Success 200 {object} bookmarkResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 409 {object} apiErrorResponse
// @Failure 422 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks/{id} [patch]
func (h handlers) updateBookmark(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "id must be a positive integer"})
		return
	}

	var req updateBookmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid JSON body"})
		return
	}

	b, err := h.svc.UpdateBookmark(c.Request.Context(), service.UpdateBookmarkInput{
		ID:         id,
		URL:        req.URL,
		Title:      req.Title,
		Tag:        req.Tag,
		CategoryID: req.CategoryID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBookmarkInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrBookmarkAlreadyExists):
			c.JSON(http.StatusConflict, apiErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrBookmarkNotFound), errors.Is(err, service.ErrCategoryNotFound):
			c.JSON(http.StatusUnprocessableEntity, apiErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to update bookmark"})
		}
		return
	}

	c.JSON(http.StatusOK, bookmarkToResponse(b))
}

// reorderBookmarks godoc
// @Summary Reorder bookmarks
// @Tags bookmarks
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param request body reorderBookmarksRequest true "Ordered bookmark IDs"
// @Success 200 {object} reorderBookmarksResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 422 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks/reorder [patch]
func (h handlers) reorderBookmarks(c *gin.Context) {
	var req reorderBookmarksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid JSON body"})
		return
	}

	err := h.svc.ReorderBookmarks(c.Request.Context(), req.IDs)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBookmarkInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrBookmarkNotFound):
			c.JSON(http.StatusUnprocessableEntity, apiErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to reorder bookmarks"})
		}
		return
	}

	c.JSON(http.StatusOK, reorderBookmarksResponse{Status: "updated"})
}

// deleteBookmark godoc
// @Summary Delete a bookmark
// @Tags bookmarks
// @Produce json
// @Security ApiBasicAuth
// @Param id path int true "Bookmark ID"
// @Success 204
// @Failure 400 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks/{id} [delete]
func (h handlers) deleteBookmark(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "id must be a positive integer"})
		return
	}

	if err := h.svc.DeleteBookmark(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrInvalidBookmarkInput) {
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to delete bookmark"})
		return
	}

	c.Status(http.StatusNoContent)
}

// toggleBookmarkHidden godoc
// @Summary Toggle bookmark hidden status
// @Tags bookmarks
// @Accept json
// @Produce json
// @Security ApiBasicAuth
// @Param id path int true "Bookmark ID"
// @Param body body toggleBookmarkHiddenRequest true "Hidden status"
// @Success 200 {object} bookmarkResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 422 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks/{id}/hidden [patch]
func (h handlers) toggleBookmarkHidden(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "id must be a positive integer"})
		return
	}

	var req toggleBookmarkHiddenRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
		return
	}

	bm, err := h.svc.ToggleBookmarkHidden(c.Request.Context(), id, req.Hidden)
	if err != nil {
		if errors.Is(err, service.ErrBookmarkNotFound) {
			c.JSON(http.StatusUnprocessableEntity, apiErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to toggle bookmark hidden status"})
		return
	}

	c.JSON(http.StatusOK, bookmarkResponse{
		ID:         bm.ID,
		URL:        bm.URL,
		Title:      bm.Title,
		Tag:        bm.Tag,
		CategoryID: bm.CategoryID,
		Position:   bm.Position,
		Hidden:     bm.Hidden,
		CreatedAt:  bm.CreatedAt,
	})
}
