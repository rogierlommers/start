package httpapi

import (
	"start/internal/config"
	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type handlers struct {
	svc *service.Service
	cfg config.Config
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

// RegisterPublic registers public JSON API routes.
func RegisterPublic(router gin.IRouter, svc *service.Service, cfg config.Config) {
	h := handlers{svc: svc, cfg: cfg}

	router.GET("/api/reading-list/rss", h.getReadingListRSS)
}

// Register registers JSON API routes.
func Register(router gin.IRouter, svc *service.Service, cfg config.Config) {
	h := handlers{svc: svc, cfg: cfg}

	router.GET("/openapi.yaml", h.openapiSpec)

	api := router.Group("/api")
	api.POST("/mail/send", h.sendMail)
	api.POST("/storage/upload", h.uploadStorageFile)
	api.POST("/storage/uploads", h.uploadStorageFiles)
	api.GET("/storage/files", h.listStorageFiles)
	api.GET("/storage/files/:filename", h.downloadStorageFile)

	api.GET("/categories", h.listCategories)
	api.POST("/categories", h.createCategory)

	api.GET("/bookmarks", h.listBookmarks)
	api.POST("/bookmarks", h.createBookmark)
	api.PATCH("/bookmarks/:id", h.updateBookmark)
	api.PATCH("/bookmarks/:id/hidden", h.toggleBookmarkHidden)
	api.PATCH("/bookmarks/reorder", h.reorderBookmarks)
	api.DELETE("/bookmarks/:id", h.deleteBookmark)

	api.POST("/reading-list/items", h.addReadingListItem)
	api.GET("/reading-list/bookmarklet-input", h.addReadingListItemFromBookmarklet)
	api.GET("/reading-list/items", h.listReadingListItems)
}

func (h handlers) openapiSpec(c *gin.Context) {
	c.File("docs/swagger.yaml")
}
