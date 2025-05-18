package routes

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"server/models"
	"strings"
	"sync"

	"encoding/base64"

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
			rpID = "connect.hsannu.com" // Changed from localhost to production domain
		}

		rpOrigin := os.Getenv("WEBAUTHN_RP_ORIGIN")
		if rpOrigin == "" {
			rpOrigin = "https://connect.hsannu.com" // Changed to HTTPS production URL
		}

		var err error
		config := &webauthn.Config{
			RPDisplayName: rpDisplayName,                                   // Display name for your app
			RPID:          rpID,                                            // Your domain
			RPOrigins:     []string{rpOrigin, "http://connect.hsannu.com"}, // Allow both HTTP and HTTPS
		}

		webAuthnConfig, err = webauthn.New(config)

		if err != nil {
			panic(fmt.Sprintf("Failed to create WebAuthn instance: %v", err))
		}

		fmt.Printf("🔑 [SERVER DEBUG] WebAuthn initialized with RPID: %s, RPOrigins: %v\n",
			webAuthnConfig.Config.RPID, webAuthnConfig.Config.RPOrigins)
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
	fmt.Println("🔑 [SERVER DEBUG] Beginning passkey registration")

	var request struct {
		Username string `json:"username" binding:"required"`
	}

	// Log raw request body
	rawData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to read request body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Print the raw request body
	fmt.Printf("🔑 [SERVER DEBUG] Raw request: %s\n", string(rawData))

	// Restore the body for binding
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(rawData))

	if err := c.BindJSON(&request); err != nil {
		fmt.Printf("❌ [SERVER DEBUG] JSON binding error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	fmt.Printf("🔑 [SERVER DEBUG] Registration requested for username: %s\n", request.Username)

	// Get user for webauthn
	user, err := models.GetUserForWebAuthn(db, request.Username)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] User lookup failed: %v\n", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	fmt.Printf("🔑 [SERVER DEBUG] User found: ID=%s, Username=%s\n", user.UserID, user.Username)

	// Log WebAuthn configuration
	fmt.Printf("🔑 [SERVER DEBUG] WebAuthn config: RPDisplayName=%s, RPID=%s\n",
		webAuthnConfig.Config.RPDisplayName, webAuthnConfig.Config.RPID)

	// Create options for registering a new credential
	fmt.Println("🔑 [SERVER DEBUG] Generating credential creation options")
	options, sessionData, err := webAuthnConfig.BeginRegistration(user)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to begin registration: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to begin registration: %v", err)})
		return
	}

	// Log session data and challenge
	fmt.Printf("🔑 [SERVER DEBUG] Session data generated, challenge length: %d\n", len(sessionData.Challenge))
	fmt.Printf("🔑 [SERVER DEBUG] Challenge (hex): %x\n", sessionData.Challenge)

	// Store session data temporarily
	storeSession(request.Username, *sessionData)
	fmt.Printf("🔑 [SERVER DEBUG] Session stored for user: %s\n", request.Username)

	// Log the response options
	optionsJSON, err := json.Marshal(options)
	if err == nil {
		fmt.Printf("🔑 [SERVER DEBUG] Response options: %s\n", string(optionsJSON))
	}

	// Return options to client
	fmt.Println("🔑 [SERVER DEBUG] Sending registration options to client")
	c.JSON(http.StatusOK, options)
}

// finishRegisterPasskey completes passkey registration
func finishRegisterPasskey(c *gin.Context, db *sql.DB) {
	fmt.Println("🔑 [SERVER DEBUG] Finishing passkey registration")

	// Log raw request body
	rawData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to read request body: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Print the raw request body
	fmt.Printf("🔑 [SERVER DEBUG] Raw finish request: %s\n", string(rawData))

	// Restore the body for binding
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(rawData))

	var request struct {
		Username string          `json:"username" binding:"required"`
		Response json.RawMessage `json:"response" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		fmt.Printf("❌ [SERVER DEBUG] JSON binding error for finish: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	fmt.Printf("🔑 [SERVER DEBUG] Registration completion for username: %s\n", request.Username)
	fmt.Printf("🔑 [SERVER DEBUG] Response data length: %d\n", len(request.Response))

	// Get user for webauthn
	user, err := models.GetUserForWebAuthn(db, request.Username)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] User lookup failed: %v\n", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	fmt.Printf("🔑 [SERVER DEBUG] User found for completion: ID=%s, Username=%s\n", user.UserID, user.Username)

	// Get session data
	sessionData, ok := getSession(request.Username)
	if !ok {
		fmt.Printf("❌ [SERVER DEBUG] Session data not found for user: %s\n", request.Username)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Registration session not found"})
		return
	}
	fmt.Printf("🔑 [SERVER DEBUG] Session data retrieved for user: %s\n", request.Username)
	fmt.Printf("🔑 [SERVER DEBUG] Challenge (hex): %x\n", sessionData.Challenge)

	// Log headers
	fmt.Println("🔑 [SERVER DEBUG] Request headers:")
	for name, values := range c.Request.Header {
		for _, value := range values {
			fmt.Printf("🔑 [SERVER DEBUG] Header %s: %s\n", name, value)
		}
	}

	// Parse the response data to extract attestation object and client data JSON
	var attestationResponse struct {
		AttestationObject string `json:"attestationObject"`
		ClientDataJSON    string `json:"clientDataJSON"`
	}

	if err := json.Unmarshal(request.Response, &attestationResponse); err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to parse attestation response: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attestation response format"})
		return
	}

	// Debug attestation data
	fmt.Printf("🔑 [SERVER DEBUG] Attestation Object: %s\n", attestationResponse.AttestationObject)
	fmt.Printf("🔑 [SERVER DEBUG] Client Data JSON: %s\n", attestationResponse.ClientDataJSON)

	// Decode and debug client data JSON contents
	clientDataBytes, err := base64.StdEncoding.DecodeString(attestationResponse.ClientDataJSON)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to decode client data JSON: %v\n", err)
	} else {
		fmt.Printf("🔑 [SERVER DEBUG] Decoded Client Data: %s\n", string(clientDataBytes))

		// Parse to get challenge and origin
		var clientData struct {
			Type      string `json:"type"`
			Challenge string `json:"challenge"`
			Origin    string `json:"origin"`
		}

		if err := json.Unmarshal(clientDataBytes, &clientData); err != nil {
			fmt.Printf("❌ [SERVER DEBUG] Failed to parse client data: %v\n", err)
		} else {
			fmt.Printf("🔑 [SERVER DEBUG] Client Data Type: %s\n", clientData.Type)
			fmt.Printf("🔑 [SERVER DEBUG] Client Data Challenge: %s\n", clientData.Challenge)
			fmt.Printf("🔑 [SERVER DEBUG] Client Data Origin: %s\n", clientData.Origin)

			// Compare challenge
			fmt.Printf("🔑 [SERVER DEBUG] Expected Challenge (hex): %x\n", sessionData.Challenge)
		}
	}

	// Try base64url decoding if standard decoding fails
	if _, err := base64.StdEncoding.DecodeString(attestationResponse.AttestationObject); err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to decode attestation object with standard base64: %v\n", err)

		// Replace characters for base64url decoding
		attestationResponse.AttestationObject = strings.ReplaceAll(attestationResponse.AttestationObject, "-", "+")
		attestationResponse.AttestationObject = strings.ReplaceAll(attestationResponse.AttestationObject, "_", "/")
		// Add padding if necessary
		if len(attestationResponse.AttestationObject)%4 != 0 {
			padding := 4 - len(attestationResponse.AttestationObject)%4
			attestationResponse.AttestationObject += strings.Repeat("=", padding)
		}

		if _, err := base64.StdEncoding.DecodeString(attestationResponse.AttestationObject); err != nil {
			fmt.Printf("❌ [SERVER DEBUG] Failed to decode attestation object with base64url: %v\n", err)
		} else {
			fmt.Println("🔑 [SERVER DEBUG] Base64url decoding of attestation object successful")
		}
	} else {
		fmt.Println("🔑 [SERVER DEBUG] Standard base64 decoding of attestation object successful")
	}

	// Display WebAuthn configuration for verification
	fmt.Printf("🔑 [SERVER DEBUG] WebAuthn config for verification - RPID: %s, Origins: %v\n",
		webAuthnConfig.Config.RPID, webAuthnConfig.Config.RPOrigins)

	// Parse and validate the attestation response
	fmt.Println("🔑 [SERVER DEBUG] Validating attestation with WebAuthn")
	credential, err := webAuthnConfig.FinishRegistration(user, sessionData, c.Request)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to verify attestation: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to verify attestation: %v", err)})
		return
	}

	// Log successful credential details
	fmt.Printf("🔑 [SERVER DEBUG] Attestation verified successfully for user: %s\n", request.Username)
	fmt.Printf("🔑 [SERVER DEBUG] Credential ID: %x\n", credential.ID)
	fmt.Printf("🔑 [SERVER DEBUG] Public Key: %x\n", credential.PublicKey)
	fmt.Printf("🔑 [SERVER DEBUG] Sign Count: %d\n", credential.Authenticator.SignCount)

	// Remove session data
	removeSession(request.Username)
	fmt.Printf("🔑 [SERVER DEBUG] Session data removed for user: %s\n", request.Username)

	// Save credential to database
	fmt.Println("🔑 [SERVER DEBUG] Saving credential to database")
	err = models.SavePasskeyCredential(db, user.UserID, *credential)
	if err != nil {
		fmt.Printf("❌ [SERVER DEBUG] Failed to save credential: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save credential: %v", err)})
		return
	}
	fmt.Printf("✅ [SERVER DEBUG] Credential saved successfully for user: %s\n", request.Username)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey registered successfully",
	})
	fmt.Println("🔑 [SERVER DEBUG] Registration process completed successfully")
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
