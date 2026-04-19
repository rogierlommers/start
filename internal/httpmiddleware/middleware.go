package httpmiddleware

import "github.com/gin-gonic/gin"

// RegisterGlobal registers middleware applied to all HTTP routes.
func RegisterGlobal(router *gin.Engine) {
	router.Use(securityHeaders())
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}
