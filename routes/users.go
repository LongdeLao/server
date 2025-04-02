package routes

import (
	"database/sql"
	"net/http"

	"server/models"

	"github.com/gin-gonic/gin"
)

// GetAllUsersHandler handles the request to get all users
func GetAllUsersHandler(c *gin.Context, db *sql.DB) {
	users, err := models.GetAllUsers(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve users",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"users":   users,
		"count":   len(users),
	})
}

// SetupUserRoutes registers all user management routes
func SetupUserRoutes(router *gin.Engine, db *sql.DB) {
	userGroup := router.Group("/api/users")
	{
		// Get all users
		userGroup.GET("", func(c *gin.Context) {
			GetAllUsersHandler(c, db)
		})

		// Additional user management routes can be added here:
		// - GET /api/users/:id - Get a specific user
		// - POST /api/users - Create a new user
		// - PUT /api/users/:id - Update a user
		// - DELETE /api/users/:id - Delete a user
	}
}
