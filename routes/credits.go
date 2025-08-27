package routes

import (
	"database/sql"
	"net/http"
	"server/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

/**
 * SetupCreditsRoutes registers all credit-related routes.
 *
 * Endpoints:
 * 1. GET /credits/user/:userID
 *    - Retrieves credit information for a specific user
 *
 * 2. POST /credits/deduct
 *    - Deducts credits from a user's account (used for AI usage)
 *
 * 3. POST /credits/add
 *    - Adds credits to a user's account (admin function)
 *
 * 4. PUT /credits/update
 *    - Updates a user's credit amount (admin function)
 *
 * 5. GET /credits/all
 *    - Retrieves all users' credit information (admin endpoint)
 */
func SetupCreditsRoutes(router *gin.RouterGroup, db *sql.DB) {
	creditsGroup := router.Group("/credits")
	{
		creditsGroup.GET("/user/:userID", getUserCredits(db))
		creditsGroup.POST("/deduct", deductCredits(db))
		creditsGroup.POST("/add", addCredits(db))
		creditsGroup.PUT("/update", updateCredits(db))
		creditsGroup.GET("/all", getAllUserCredits(db))
	}
}

/**
 * getUserCredits retrieves the credit information for a specific user.
 *
 * Endpoint: GET /credits/user/:userID
 *
 * Parameters:
 *   - userID: The ID of the user (integer)
 *
 * Returns:
 *   - 200 OK: User credits retrieved successfully
 *     {
 *       "success": boolean,
 *       "data": object
 *     }
 *   - 400 Bad Request: Invalid user ID format
 *   - 500 Internal Server Error: Database error
 */
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

/**
 * deductCredits deducts credits from a user's account (used for AI usage).
 *
 * Endpoint: POST /credits/deduct
 *
 * Request Body:
 * {
 *   "user_id": number,  // Required: User ID
 *   "amount": number    // Required: Amount to deduct (minimum 1)
 * }
 *
 * Returns:
 *   - 200 OK: Credits deducted successfully
 *     {
 *       "success": boolean,
 *       "message": string,
 *       "data": object
 *     }
 *   - 400 Bad Request: Invalid request data
 *   - 402 Payment Required: Insufficient credits
 *   - 500 Internal Server Error: Database error
 */
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

/**
 * addCredits adds credits to a user's account (admin function).
 *
 * Endpoint: POST /credits/add
 *
 * Request Body:
 * {
 *   "user_id": number,  // Required: User ID
 *   "amount": number    // Required: Amount to add (minimum 1)
 * }
 *
 * Returns:
 *   - 200 OK: Credits added successfully
 *     {
 *       "success": boolean,
 *       "message": string,
 *       "data": object
 *     }
 *   - 400 Bad Request: Invalid request data
 *   - 500 Internal Server Error: Database error
 */
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

/**
 * updateCredits updates a user's credit amount (admin function).
 *
 * Endpoint: PUT /credits/update
 *
 * Request Body:
 * {
 *   "user_id": number,     // Required: User ID
 *   "new_credits": number  // Required: New credit amount (minimum 0)
 * }
 *
 * Returns:
 *   - 200 OK: Credits updated successfully
 *     {
 *       "success": boolean,
 *       "message": string,
 *       "data": object
 *     }
 *   - 400 Bad Request: Invalid request data
 *   - 500 Internal Server Error: Database error
 */
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

/**
 * getAllUserCredits retrieves all users' credit information (admin function).
 *
 * Endpoint: GET /credits/all
 *
 * Returns:
 *   - 200 OK: All user credits retrieved successfully
 *     {
 *       "success": boolean,
 *       "data": array,
 *       "count": number
 *     }
 *   - 500 Internal Server Error: Database error
 */
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
