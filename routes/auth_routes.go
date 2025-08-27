package routes

import (
	"database/sql"
	"net/http"
	"server/handlers"

	"github.com/gin-gonic/gin"
)

/**
 * Login is a temporary placeholder for the Login handler until the real one is implemented.
 *
 * Endpoint: POST /login
 *
 * Returns:
 *   - 200 OK: Login endpoint message
 *     {
 *       "message": "Login endpoint"
 *     }
 */
func Login(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Login endpoint"})
}

/**
 * RegisterAuthRoutes registers all authentication routes.
 *
 * Endpoints:
 * 1. POST /password/reset-request
 *    - Requests a password reset
 *    - Sets database connection in context
 *    - Calls handlers.RequestPasswordReset
 *
 * 2. POST /password/verify-code
 *    - Verifies reset code
 *    - Calls handlers.VerifyResetCode
 *
 * 3. POST /password/reset
 *    - Resets password
 *    - Calls handlers.ResetPassword
 */
func RegisterAuthRoutes(router *gin.RouterGroup, db *sql.DB) {
	router.POST("/password/reset-request", func(c *gin.Context) {
		c.Set("db", db)
		handlers.RequestPasswordReset(c)
	})

	router.POST("/password/verify-code", handlers.VerifyResetCode)

	router.POST("/password/reset", func(c *gin.Context) {
		handlers.ResetPassword(c, db)
	})
}
