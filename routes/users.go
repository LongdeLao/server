package routes

import (
	"database/sql"
	"fmt"
	"net/http"

	"server/models"

	"github.com/gin-gonic/gin"
)

/**
 * GetAllUsersHandler handles the request to get all users.
 *
 * Endpoint: GET /users
 *
 * Returns:
 *   - 200 OK: List of all users
 *     {
 *       "success": boolean,
 *       "users": array,
 *       "count": number
 *     }
 *   - 500 Internal Server Error: Database error
 */
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

/**
 * UpdateDeviceTokenHandler updates a user's device token for push notifications.
 *
 * Endpoint: POST /user/update-device-token
 *
 * Request Body:
 * {
 *   "user_id": number,      // Required: User ID
 *   "device_token": string  // Required: Device token for push notifications
 * }
 *
 * Returns:
 *   - 200 OK: Device token updated successfully
 *     {
 *       "success": boolean,
 *       "message": string
 *     }
 *   - 400 Bad Request: Invalid request format or validation error
 *   - 404 Not Found: User not found
 *   - 500 Internal Server Error: Database error
 */
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

/**
 * SetupUserRoutes registers all user management routes.
 *
 * Endpoints:
 * 1. GET /users
 *    - Retrieves all users
 *
 * 2. POST /user/update-device-token
 *    - Updates user's device token for push notifications
 */
func SetupUserRoutes(router gin.IRouter, db *sql.DB) {
	userGroup := router.Group("/users")
	{
		userGroup.GET("", func(c *gin.Context) {
			GetAllUsersHandler(c, db)
		})

		router.POST("/user/update-device-token", func(c *gin.Context) {
			UpdateDeviceTokenHandler(c, db)
		})
	}
}
