package routes

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	db "server/database"
	"server/models"

	"github.com/gin-gonic/gin"
)

/**
 * RegisterLoginRoute registers the login route.
 *
 * Endpoint: POST /login
 *
 * Request Body:
 * {
 *   "username": string,  // Required: User's username
 *   "password": string,  // Required: User's password
 *   "deviceID": string   // Optional: Device identifier for mobile apps
 * }
 *
 * Returns:
 *   - 200 OK: Successfully logged in
 *     {
 *       "id": number,              // User's ID
 *       "username": string,        // User's username
 *       "password": string,        // User's password (plaintext)
 *       "name": string,           // User's full name
 *       "role": string,           // User's primary role
 *       "status": string,         // User's account status
 *       "additional_roles": string[] // Array of additional roles
 *     }
 *   - 400 Bad Request: Invalid request format
 *   - 401 Unauthorized: Invalid credentials
 *   - 404 Not Found: User not found
 *   - 500 Internal Server Error: Database error
 */
func RegisterLoginRoute(router *gin.Engine) {
	router.POST("/login", loginHandler)
}

/// Example response on successful login:
///
/// ```json
/// {
///   "id": 1,
///   "username": "johndoe",
///   "name": "John Doe",
///   "role": "admin",
///   "additional_roles": ["manager", "developer"]
/// }
/// ```

/**
 * loginHandler processes the login request.
 *
 * This function:
 * 1. Validates the request body
 * 2. Checks user credentials against the database
 * 3. Updates device ID if provided
 * 4. Fetches additional roles
 * 5. Returns user data on success
 *
 * @param c *gin.Context - The Gin context containing the request
 */
func loginHandler(c *gin.Context) {
	// Read the raw request body.
	rawData, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Print the raw payload
	fmt.Printf("DEBUGGER: Incoming Payload: %s\n", string(rawData))

	// Restore the request body so it can be read again by BindJSON
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(rawData))

	// Accept an extra field "deviceID"
	var loginData struct {
		Username string `json:"username"`
		Password string `json:"password"`
		DeviceID string `json:"deviceID"`
	}
	if err := c.BindJSON(&loginData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if loginData.Username == "" || loginData.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return
	}

	conn, err := db.GetConnection()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB connection error"})
		return
	}
	defer conn.Close()

	var user models.User
	query := "SELECT id, username, name, password, role, status FROM users WHERE username = $1"
	err = conn.QueryRow(query, loginData.Username).Scan(&user.ID, &user.Username, &user.Name, &user.Password, &user.Role, &user.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Query error"})
		}
		return
	}

	if user.Password != loginData.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// If a deviceID is provided, update it in the database for this user
	if loginData.DeviceID != "" {
		updateQuery := "UPDATE users SET device_id = $1 WHERE id = $2"
		if _, err := conn.Exec(updateQuery, loginData.DeviceID, user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update deviceID"})
			return
		}
	} else {
		fmt.Printf("NO_DEVICE_ID\n")
	}

	// Fetch additional roles for this user
	rolesQuery := "SELECT role FROM additional_roles WHERE user_id = $1"
	rows, err := conn.Query(rolesQuery, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch additional roles"})
		return
	}
	defer rows.Close()

	// Initialize an empty slice for additional roles
	user.AdditionalRoles = []string{}

	// Iterate through the rows and collect the roles
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process additional roles"})
			return
		}
		user.AdditionalRoles = append(user.AdditionalRoles, role)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error during additional roles fetch"})
		return
	}

	// Get profile picture URL
	var filePath sql.NullString
	var profilePicture string = ""
	err = conn.QueryRow("SELECT file_path FROM profile_pictures WHERE user_id = $1", user.ID).Scan(&filePath)
	if err == nil && filePath.Valid {
		profilePicture = fmt.Sprintf("/%s", filePath.String)
	}

	// Return user data with additional roles and status
	c.JSON(http.StatusOK, gin.H{
		"id":               user.ID,
		"username":         user.Username,
		"password":         user.Password,
		"name":             user.Name,
		"role":             user.Role,
		"status":           user.Status,
		"additional_roles": user.AdditionalRoles,
		"profile_picture":  profilePicture,
	})
}
