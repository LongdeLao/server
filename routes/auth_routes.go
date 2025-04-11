package routes

import (
	"database/sql"
	"net/http"
	"server/handlers"

	"github.com/gin-gonic/gin"
)

// This is a temporary placeholder for the Login handler until the real one is implemented
func Login(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Login endpoint"})
}

// RegisterAuthRoutes registers all authentication routes
func RegisterAuthRoutes(router *gin.RouterGroup, db *sql.DB) {
	// Login route is already registered via RegisterLoginRoute in main.go
	// router.POST("/login", Login)

	// Register password reset routes
	router.POST("/password/reset-request", func(c *gin.Context) {
		// Set database connection in context
		c.Set("db", db)
		handlers.RequestPasswordReset(c)
	})

	router.POST("/password/verify-code", handlers.VerifyResetCode)

	router.POST("/password/reset", func(c *gin.Context) {
		handlers.ResetPassword(c, db)
	})
}
