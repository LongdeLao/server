package routes

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"server/models"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

/**
 * RegisterProfileRoutes registers all profile-related routes.
 *
 * Endpoints:
 * 1. POST /profile/upload-picture/:userId
 *    - Uploads a profile picture for a user
 *    - Accepts multipart form data with "profile_picture" field
 *    - Returns URL of uploaded picture
 *
 * 2. GET /profile/:userId
 *    - Retrieves user profile information
 *    - Returns complete user profile including roles and picture
 *
 * 3. PUT /profile/update-email/:userId
 *    - Updates user's email address
 *    - Requires new email in request body
 *
 * 4. PUT /profile/change-password/:userId
 *    - Changes user's password
 *    - Requires current and new password in request body
 */
func RegisterProfileRoutes(router gin.IRouter, db *sql.DB) {
	router.POST("/profile/upload-picture/:userId", handleProfilePictureUpload(db))
	router.GET("/profile/:userId", getProfileInfo(db))
	router.PUT("/profile/update-email/:userId", updateUserEmail(db))
	router.PUT("/profile/change-password/:userId", changePassword(db))
}

/**
 * changePassword handles changing a user's password.
 *
 * Endpoint: PUT /profile/change-password/:userId
 *
 * Request Body:
 * {
 *   "currentPassword": string,  // Required: Current password
 *   "newPassword": string       // Required: New password
 * }
 *
 * Returns:
 *   - 200 OK: Password changed successfully
 *     {
 *       "message": "Password changed successfully"
 *     }
 *   - 400 Bad Request: Invalid request format or missing fields
 *   - 401 Unauthorized: Current password is incorrect
 *   - 404 Not Found: User not found
 *   - 500 Internal Server Error: Database error
 */
func changePassword(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from URL parameter
		userIdStr := c.Param("userId")
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid user ID",
			})
			return
		}

		// Get password data from the request body
		var reqBody struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
			})
			return
		}

		// Validate that both current and new passwords are provided
		if reqBody.CurrentPassword == "" || reqBody.NewPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Both current and new passwords are required",
			})
			return
		}

		// Get the user's current password from the database
		var storedPassword string
		err = db.QueryRow("SELECT password FROM users WHERE id = $1", userId).Scan(&storedPassword)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "User not found",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve user information",
			})
			return
		}

		// Check if the provided current password matches the stored password
		// For plain text comparison (not recommended for production)
		if storedPassword != reqBody.CurrentPassword {
			// Try hashed comparison (if you're using bcrypt or similar)
			err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(reqBody.CurrentPassword))
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Current password is incorrect",
				})
				return
			}
		}

		// Hash the new password if using bcrypt
		var newPasswordToStore string
		// Uncomment and use this for hashed passwords
		/*
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(reqBody.NewPassword), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to process new password",
				})
				return
			}
			newPasswordToStore = string(hashedPassword)
		*/

		// For plaintext storage (not recommended for production)
		newPasswordToStore = reqBody.NewPassword

		// Update the password in the database
		_, err = db.Exec("UPDATE users SET password = $1 WHERE id = $2", newPasswordToStore, userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update password",
			})
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"message": "Password changed successfully",
		})
	}
}

/**
 * updateUserEmail handles updating a user's email address.
 *
 * Endpoint: PUT /profile/update-email/:userId
 *
 * Request Body:
 * {
 *   "email": string  // Required: New email address
 * }
 *
 * Returns:
 *   - 200 OK: Email updated successfully
 *     {
 *       "message": "Email updated successfully",
 *       "email": string
 *     }
 *   - 400 Bad Request: Invalid request format or empty email
 *   - 500 Internal Server Error: Database error
 */
func updateUserEmail(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from URL parameter
		userIdStr := c.Param("userId")
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid user ID",
			})
			return
		}

		// Get new email from the request body
		var reqBody struct {
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
			})
			return
		}

		// Validate email (simple validation)
		if reqBody.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Email cannot be empty",
			})
			return
		}

		// Update the user's email in the database
		_, err = db.Exec("UPDATE users SET email = $1 WHERE id = $2", reqBody.Email, userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update email",
			})
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"message": "Email updated successfully",
			"email":   reqBody.Email,
		})
	}
}

/**
 * handleProfilePictureUpload handles uploading a profile picture.
 *
 * Endpoint: POST /profile/upload-picture/:userId
 *
 * Request:
 * - Content-Type: multipart/form-data
 * - Form field: "profile_picture" (file)
 * - Supported formats: JPG, JPEG, PNG
 *
 * Returns:
 *   - 200 OK: Picture uploaded successfully
 *     {
 *       "message": "Profile picture uploaded successfully",
 *       "profile_picture": string  // URL to the uploaded picture
 *     }
 *   - 400 Bad Request: Invalid file format or no file uploaded
 *   - 500 Internal Server Error: File system or database error
 */
func handleProfilePictureUpload(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from URL parameter
		userIdStr := c.Param("userId")
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid user ID",
			})
			return
		}

		// Get file from the request
		file, err := c.FormFile("profile_picture")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No file uploaded or error in upload",
			})
			return
		}

		// Check file type
		extension := filepath.Ext(file.Filename)
		if extension != ".jpg" && extension != ".jpeg" && extension != ".png" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Only JPG, JPEG and PNG files are allowed",
			})
			return
		}

		// Ensure profile_pictures directory exists
		profilePicDir := "profile_pictures"
		if _, err := os.Stat(profilePicDir); os.IsNotExist(err) {
			if err := os.MkdirAll(profilePicDir, 0755); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to create directory",
				})
				return
			}
		}

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to start transaction",
			})
			return
		}

		// Step 1: Check if profile picture exists in database
		var existingFilePath string
		err = tx.QueryRow("SELECT file_path FROM profile_pictures WHERE user_id = $1", userId).Scan(&existingFilePath)

		// Step 2: If exists, delete the current file from folder and remove database entry
		if err == nil {
			// Delete the existing file
			if err := os.Remove(existingFilePath); err != nil && !os.IsNotExist(err) {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to remove existing file",
				})
				return
			}

			// Delete the database entry
			_, err = tx.Exec("DELETE FROM profile_pictures WHERE user_id = $1", userId)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to remove existing database entry",
				})
				return
			}
		} else if err != sql.ErrNoRows {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to check existing profile picture",
			})
			return
		}

		// Step 3: Save new file to folder
		filename := fmt.Sprintf("%d%s", userId, extension)
		filePath := filepath.Join(profilePicDir, filename)

		if err := c.SaveUploadedFile(file, filePath); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to save file",
			})
			return
		}

		// Step 4: Create new database entry
		_, err = tx.Exec("INSERT INTO profile_pictures (user_id, file_path) VALUES ($1, $2)",
			userId, filePath)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create database entry",
			})
			return
		}

		// Commit the transaction
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to commit changes",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":         "Profile picture uploaded successfully",
			"profile_picture": fmt.Sprintf("/profile_pictures/%d%s", userId, extension),
		})
	}
}

/**
 * getProfileInfo retrieves a user's complete profile information.
 *
 * Endpoint: GET /profile/:userId
 *
 * Returns:
 *   - 200 OK: Profile retrieved successfully
 *     {
 *       "id": number,              // User's ID
 *       "username": string,        // User's username
 *       "name": string,           // User's full name
 *       "role": string,           // User's primary role
 *       "email": string,          // User's email address
 *       "status": string,         // User's account status
 *       "profile_picture": string, // URL to profile picture
 *       "additional_roles": string[] // Array of additional roles
 *     }
 *   - 404 Not Found: User not found
 *   - 500 Internal Server Error: Database error
 */
func getProfileInfo(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from URL parameter
		userIdStr := c.Param("userId")
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid user ID",
			})
			return
		}

		// Query the database
		var user models.User
		var status string

		err = db.QueryRow("SELECT id, username, name, role, email, status FROM users WHERE id = $1", userId).Scan(
			&user.ID,
			&user.Username,
			&user.Name,
			&user.Role,
			&user.Email,
			&status,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "User not found",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve user profile",
			})
			return
		}

		// Initialize an empty slice for additional roles
		var additionalRoles []string

		// Get additional roles
		rows, err := db.Query("SELECT role FROM additional_roles WHERE user_id = $1", userId)
		if err == nil {
			defer rows.Close()

			// Iterate through rows and collect roles
			for rows.Next() {
				var role string
				if err := rows.Scan(&role); err == nil {
					additionalRoles = append(additionalRoles, role)
				}
			}
		}

		// Get profile picture from profile_pictures table
		var filePath sql.NullString
		var profilePicture string = ""

		err = db.QueryRow("SELECT file_path FROM profile_pictures WHERE user_id = $1", userId).Scan(&filePath)

		if err == nil && filePath.Valid {
			// Convert file path to URL without adding redundant /api prefix
			// Extract just the filename since we need to reference it correctly
			_, filename := filepath.Split(filePath.String)
			profilePicture = fmt.Sprintf("/profile_pictures/%s", filename)
		}

		// Return the user profile with explicit fields including status
		c.JSON(http.StatusOK, gin.H{
			"id":               user.ID,
			"username":         user.Username,
			"name":             user.Name,
			"role":             user.Role,
			"email":            user.Email,
			"status":           status,
			"profile_picture":  profilePicture,
			"additional_roles": additionalRoles,
		})
	}
}
