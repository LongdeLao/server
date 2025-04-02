package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterTestRoute registers a simple test route
func RegisterTestRoute(router gin.IRouter) {
	router.GET("/dev/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello, this is a test route!",
		})
	})
}
