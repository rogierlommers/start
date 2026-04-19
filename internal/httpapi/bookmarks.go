package httpapi

import (
	"errors"
	"net/http"
	"strconv"
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
	CategoryID int64     `json:"category_id"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

type createBookmarkRequest struct {
	URL        string `json:"url"             binding:"required"`
	Title      string `json:"title"`
	CategoryID int64  `json:"category_id"     binding:"required"`
}

type reorderBookmarksRequest struct {
	IDs []int64 `json:"ids" binding:"required"`
}

type reorderBookmarksResponse struct {
	Status string `json:"status"`
}

func bookmarkToResponse(b service.Bookmark) bookmarkResponse {
	return bookmarkResponse{
		ID:         b.ID,
		URL:        b.URL,
		Title:      b.Title,
		CategoryID: b.CategoryID,
		Position:   b.Position,
		CreatedAt:  b.CreatedAt,
	}
}

// listCategories godoc
// @Summary List categories
// @Tags bookmarks
// @Produce json
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
// @Param request body createCategoryRequest true "Category payload"
// @Success 201 {object} categoryResponse
// @Failure 400 {object} apiErrorResponse
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
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, categoryResponse{ID: cat.ID, Name: cat.Name})
}

// deleteCategory godoc
// @Summary Delete a category
// @Tags bookmarks
// @Param id path int true "Category ID"
// @Success 204
// @Failure 400 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/categories/{id} [delete]
func (h handlers) deleteCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "id must be a positive integer"})
		return
	}

	if err := h.svc.DeleteCategory(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrInvalidCategoryInput) {
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to delete category"})
		return
	}

	c.Status(http.StatusNoContent)
}

// listBookmarks godoc
// @Summary List bookmarks
// @Tags bookmarks
// @Produce json
// @Success 200 {array} bookmarkResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/bookmarks [get]
func (h handlers) listBookmarks(c *gin.Context) {
	bookmarks, err := h.svc.ListBookmarks(c.Request.Context())
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

// createBookmark godoc
// @Summary Create a bookmark
// @Tags bookmarks
// @Accept json
// @Produce json
// @Param request body createBookmarkRequest true "Bookmark payload"
// @Success 201 {object} bookmarkResponse
// @Failure 400 {object} apiErrorResponse
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
		CategoryID: req.CategoryID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBookmarkInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrCategoryNotFound):
			c.JSON(http.StatusUnprocessableEntity, apiErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to create bookmark"})
		}
		return
	}

	c.JSON(http.StatusCreated, bookmarkToResponse(b))
}

// reorderBookmarks godoc
// @Summary Reorder bookmarks
// @Tags bookmarks
// @Accept json
// @Produce json
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
