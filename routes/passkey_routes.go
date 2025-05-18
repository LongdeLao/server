package routes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"server/models"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
)

// Global WebAuthn instance
var (
	webAuthnConfig *webauthn.WebAuthn
	setupOnce      sync.Once
	sessionStore   = make(map[string]webauthn.SessionData)
	sessionMutex   sync.RWMutex
)

// initWebAuthn initializes the WebAuthn configuration
func initWebAuthn() {
	setupOnce.Do(func() {
		// Get the RPDisplayName from environment or default to "HSANNU"
		rpDisplayName := os.Getenv("WEBAUTHN_RP_DISPLAY_NAME")
		if rpDisplayName == "" {
			rpDisplayName = "HSANNU"
		}

		// Get the RPID from environment or default to hostname
		rpID := os.Getenv("WEBAUTHN_RP_ID")
		if rpID == "" {
			rpID = "localhost" // Use appropriate domain in production
		}

		rpOrigin := os.Getenv("WEBAUTHN_RP_ORIGIN")
		if rpOrigin == "" {
			rpOrigin = "http://localhost:2000" // Default to HTTP localhost
		}

		var err error
		webAuthnConfig, err = webauthn.New(&webauthn.Config{
			RPDisplayName: rpDisplayName,      // Display name for your app
			RPID:          rpID,               // Your domain
			RPOrigins:     []string{rpOrigin}, // Origins that are allowed to use the WebAuthn
		})

		if err != nil {
			panic(fmt.Sprintf("Failed to create WebAuthn instance: %v", err))
		}
	})
}

// SetupPasskeyRoutes registers all passkey related routes
func SetupPasskeyRoutes(router *gin.RouterGroup, db *sql.DB) {
	// Initialize WebAuthn
	initWebAuthn()

	// Register passkey routes
	router.POST("/begin-register-passkey", func(c *gin.Context) {
		beginRegisterPasskey(c, db)
	})

	router.POST("/finish-register-passkey", func(c *gin.Context) {
		finishRegisterPasskey(c, db)
	})

	router.POST("/begin-login-passkey", func(c *gin.Context) {
		beginLoginPasskey(c, db)
	})

	router.POST("/finish-login-passkey", func(c *gin.Context) {
		finishLoginPasskey(c, db)
	})

	// Route to check if a user has a passkey
	router.GET("/has-passkey/:username", func(c *gin.Context) {
		hasPasskey(c, db)
	})
}

// storeSession stores session data in memory
func storeSession(username string, data webauthn.SessionData) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	sessionStore[username] = data
}

// getSession retrieves session data from memory
func getSession(username string) (webauthn.SessionData, bool) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	session, ok := sessionStore[username]
	return session, ok
}

// removeSession removes session data from memory
func removeSession(username string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	delete(sessionStore, username)
}

// beginRegisterPasskey initiates passkey registration
func beginRegisterPasskey(c *gin.Context, db *sql.DB) {
	var request struct {
		Username string `json:"username" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user for webauthn
	user, err := models.GetUserForWebAuthn(db, request.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Create options for registering a new credential
	options, sessionData, err := webAuthnConfig.BeginRegistration(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin registration"})
		return
	}

	// Store session data temporarily
	storeSession(request.Username, *sessionData)

	// Return options to client
	c.JSON(http.StatusOK, options)
}

// finishRegisterPasskey completes passkey registration
func finishRegisterPasskey(c *gin.Context, db *sql.DB) {
	var request struct {
		Username string          `json:"username" binding:"required"`
		Response json.RawMessage `json:"response" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user for webauthn
	user, err := models.GetUserForWebAuthn(db, request.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get session data
	sessionData, ok := getSession(request.Username)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Registration session not found"})
		return
	}

	// Parse and validate the attestation response
	credential, err := webAuthnConfig.FinishRegistration(user, sessionData, c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to verify attestation: %v", err)})
		return
	}

	// Remove session data
	removeSession(request.Username)

	// Save credential to database
	err = models.SavePasskeyCredential(db, user.UserID, *credential)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save credential"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey registered successfully",
	})
}

// beginLoginPasskey initiates passkey authentication
func beginLoginPasskey(c *gin.Context, db *sql.DB) {
	var request struct {
		Username string `json:"username" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user for webauthn
	user, err := models.GetUserForWebAuthn(db, request.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user has any passkeys registered
	if len(user.Credentials) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No passkeys registered for this user"})
		return
	}

	// Create options for authentication
	options, sessionData, err := webAuthnConfig.BeginLogin(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin authentication"})
		return
	}

	// Store session data temporarily
	storeSession(request.Username, *sessionData)

	// Return options to client
	c.JSON(http.StatusOK, options)
}

// finishLoginPasskey completes passkey authentication
func finishLoginPasskey(c *gin.Context, db *sql.DB) {
	var request struct {
		Username string          `json:"username" binding:"required"`
		Response json.RawMessage `json:"response" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user for webauthn
	user, err := models.GetUserForWebAuthn(db, request.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get session data
	sessionData, ok := getSession(request.Username)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authentication session not found"})
		return
	}

	// Parse and validate the assertion
	credential, err := webAuthnConfig.FinishLogin(user, sessionData, c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Authentication failed: %v", err)})
		return
	}

	// Remove session data
	removeSession(request.Username)

	// Update the credential's sign count in database
	err = models.UpdatePasskeyCredential(db, credential.ID, credential.Authenticator.SignCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update credential"})
		return
	}

	// Get full user for login response
	var fullUser models.User
	userQuery := "SELECT id, username, name, password, role, status FROM users WHERE username = $1"
	err = db.QueryRow(userQuery, request.Username).Scan(
		&fullUser.ID,
		&fullUser.Username,
		&fullUser.Name,
		&fullUser.Password,
		&fullUser.Role,
		&fullUser.Status,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user data"})
		return
	}

	// Fetch additional roles for this user
	rolesQuery := "SELECT role FROM additional_roles WHERE user_id = $1"
	rows, err := db.Query(rolesQuery, fullUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch additional roles"})
		return
	}
	defer rows.Close()

	// Initialize an empty slice for additional roles
	fullUser.AdditionalRoles = []string{}

	// Iterate through the rows and collect the roles
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process additional roles"})
			return
		}
		fullUser.AdditionalRoles = append(fullUser.AdditionalRoles, role)
	}

	// Get profile picture URL
	var filePath sql.NullString
	var profilePicture string = ""
	err = db.QueryRow("SELECT file_path FROM profile_pictures WHERE user_id = $1", fullUser.ID).Scan(&filePath)
	if err == nil && filePath.Valid {
		profilePicture = fmt.Sprintf("/%s", filePath.String)
	}

	// Return user data
	c.JSON(http.StatusOK, gin.H{
		"id":               fullUser.ID,
		"username":         fullUser.Username,
		"name":             fullUser.Name,
		"role":             fullUser.Role,
		"status":           fullUser.Status,
		"additional_roles": fullUser.AdditionalRoles,
		"profile_picture":  profilePicture,
		"passkey_verified": true,
	})
}

// hasPasskey checks if a user has a passkey
func hasPasskey(c *gin.Context, db *sql.DB) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	// Get user for webauthn
	user, err := models.GetUserForWebAuthn(db, username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user has any passkeys
	hasPasskey, err := models.HasPasskey(db, user.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check passkeys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_passkey": hasPasskey,
	})
}
