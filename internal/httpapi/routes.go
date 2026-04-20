package httpapi

import (
	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type handlers struct {
	svc *service.Service
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

// Register registers JSON API routes.
func Register(router gin.IRouter, svc *service.Service) {
	h := handlers{svc: svc}

	router.GET("/openapi.yaml", h.openapiSpec)

	api := router.Group("/api")
	api.POST("/mail/send", h.sendMail)

	api.GET("/categories", h.listCategories)
	api.POST("/categories", h.createCategory)
	api.DELETE("/categories/:id", h.deleteCategory)

	api.GET("/bookmarks", h.listBookmarks)
	api.POST("/bookmarks", h.createBookmark)
	api.PATCH("/bookmarks/reorder", h.reorderBookmarks)
	api.DELETE("/bookmarks/:id", h.deleteBookmark)

	api.POST("/reading-list/items", h.addReadingListItem)
	api.GET("/reading-list/items", h.listReadingListItems)
	api.GET("/reading-list/rss", h.getReadingListRSS)
}

func (h handlers) openapiSpec(c *gin.Context) {
	c.File("docs/swagger.yaml")
}
