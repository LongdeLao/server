package routes

import (
	"database/sql"
	"net/http"
	"server/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SetupCreditsRoutes registers all credit-related routes
func SetupCreditsRoutes(router *gin.RouterGroup, db *sql.DB) {
	creditsGroup := router.Group("/credits")
	{
		creditsGroup.GET("/user/:userID", getUserCredits(db))
		creditsGroup.POST("/deduct", deductCredits(db))
		creditsGroup.POST("/add", addCredits(db))
		creditsGroup.PUT("/update", updateCredits(db))
		creditsGroup.GET("/all", getAllUserCredits(db)) // Admin endpoint
	}
}

// getUserCredits retrieves the credit information for a specific user
func getUserCredits(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userID")
		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid user ID",
				"message": "User ID must be a valid integer",
			})
			return
		}

		credits, err := models.GetUserCredits(db, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to retrieve user credits",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    credits,
		})
	}
}

// deductCredits deducts credits from a user's account (used for AI usage)
func deductCredits(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			UserID int `json:"user_id" binding:"required"`
			Amount int `json:"amount" binding:"required,min=1"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request data",
				"message": err.Error(),
			})
			return
		}

		credits, err := models.DeductUserCredits(db, request.UserID, request.Amount)
		if err != nil {
			// Check if it's an insufficient credits error
			if err.Error() == "insufficient credits" ||
				(len(err.Error()) > 20 && err.Error()[:20] == "insufficient credits") {
				c.JSON(http.StatusPaymentRequired, gin.H{
					"error":   "Insufficient credits",
					"message": err.Error(),
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to deduct credits",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Credits deducted successfully",
			"data":    credits,
		})
	}
}

// addCredits adds credits to a user's account (admin function)
func addCredits(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			UserID int `json:"user_id" binding:"required"`
			Amount int `json:"amount" binding:"required,min=1"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request data",
				"message": err.Error(),
			})
			return
		}

		credits, err := models.AddUserCredits(db, request.UserID, request.Amount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to add credits",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Credits added successfully",
			"data":    credits,
		})
	}
}

// updateCredits updates a user's credit amount (admin function)
func updateCredits(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			UserID     int `json:"user_id" binding:"required"`
			NewCredits int `json:"new_credits" binding:"required,min=0"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request data",
				"message": err.Error(),
			})
			return
		}

		err := models.UpdateUserCredits(db, request.UserID, request.NewCredits)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update credits",
				"message": err.Error(),
			})
			return
		}

		// Get updated credits to return
		credits, err := models.GetUserCredits(db, request.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Credits updated but failed to retrieve updated data",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Credits updated successfully",
			"data":    credits,
		})
	}
}

// getAllUserCredits retrieves all users' credit information (admin function)
func getAllUserCredits(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		allCredits, err := models.GetAllUserCredits(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to retrieve all user credits",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    allCredits,
			"count":   len(allCredits),
		})
	}
}
