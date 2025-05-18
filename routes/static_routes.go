package routes

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// SetupStaticRoutes registers routes for serving static files like .well-known
func SetupStaticRoutes(router *gin.Engine) {
	// Serve the .well-known directory for Apple App Site Association
	router.GET("/.well-known/*path", func(c *gin.Context) {
		path := c.Param("path")
		filePath := filepath.Join(".well-known", path)

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}

		// Set proper Content-Type for JSON files
		if filepath.Ext(path) == ".json" || path == "/apple-app-site-association" {
			c.Header("Content-Type", "application/json")
		}

		c.File(filePath)
	})
}
