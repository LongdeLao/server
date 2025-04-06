package routes

import (
	"database/sql"
	"fmt"
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

// UpdateDeviceTokenHandler updates a user's device token for push notifications
func UpdateDeviceTokenHandler(c *gin.Context, db *sql.DB) {
	var request struct {
		UserID      int    `json:"user_id" binding:"required"`
		DeviceToken string `json:"device_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format",
			"error":   err.Error(),
		})
		return
	}

	// Validate request
	if request.UserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	if request.DeviceToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Device token cannot be empty",
		})
		return
	}

	// Check if user exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", request.UserID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Database error",
			"error":   err.Error(),
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("User with ID %d not found", request.UserID),
		})
		return
	}

	// Update the device token
	_, err = db.Exec("UPDATE users SET device_id = $1 WHERE id = $2", request.DeviceToken, request.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update device token",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device token updated successfully",
	})
}

// SetupUserRoutes registers all user management routes
func SetupUserRoutes(router gin.IRouter, db *sql.DB) {
	userGroup := router.Group("/users")
	{
		// Get all users
		userGroup.GET("", func(c *gin.Context) {
			GetAllUsersHandler(c, db)
		})

		// Update device token endpoint
		router.POST("/user/update-device-token", func(c *gin.Context) {
			UpdateDeviceTokenHandler(c, db)
		})

		// Additional user management routes can be added here:
		// - GET /api/users/:id - Get a specific user
		// - POST /api/users - Create a new user
		// - PUT /api/users/:id - Update a user
		// - DELETE /api/users/:id - Delete a user
	}
}
