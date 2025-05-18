package routes

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// SetupStaticRoutes registers routes for serving static files like .well-known
func SetupStaticRoutes(router *gin.Engine) {
	// Serve the apple-app-site-association file directly at the root level
	router.GET("/apple-app-site-association", func(c *gin.Context) {
		serveAppSiteAssociation(c, ".well-known/apple-app-site-association")
	})

	// Serve the apple-app-site-association file in the .well-known directory
	router.GET("/.well-known/apple-app-site-association", func(c *gin.Context) {
		serveAppSiteAssociation(c, ".well-known/apple-app-site-association")
	})

	// Serve other files in the .well-known directory
	router.GET("/.well-known/*path", func(c *gin.Context) {
		path := c.Param("path")
		if path == "/apple-app-site-association" {
			// This is handled by the specific route above
			return
		}

		filePath := filepath.Join(".well-known", path)

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}

		// Set proper Content-Type for JSON files
		if filepath.Ext(path) == ".json" {
			c.Header("Content-Type", "application/json")
		}

		c.File(filePath)
	})
}

// Helper function to serve the Apple App Site Association file
func serveAppSiteAssociation(c *gin.Context, filePath string) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("Error: Apple App Site Association file not found at %s\n", filePath)
		c.JSON(http.StatusNotFound, gin.H{"error": "Apple App Site Association file not found"})
		return
	}

	// Always set the correct Content-Type for AASA files
	c.Header("Content-Type", "application/json")

	c.File(filePath)
}
