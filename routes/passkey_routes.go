package routes

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// Helper function for string truncation
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const (
	// Domain for the WebAuthn relying party
	rpDomain = "connect.hsannu.com"
	// Display name for the WebAuthn relying party
	rpName = "HSANNU"
)

var (
	// WebAuthn instance
	webAuthnInstance *webauthn.WebAuthn
	// Session storage for WebAuthn challenges
	sessionStore = make(map[string]*webauthn.SessionData)
)

// User represents a user in the WebAuthn system
type PasskeyUser struct {
	ID          int
	Name        string
	Username    string
	Credentials []webauthn.Credential
}

// Implementation of webauthn.User interface for our PasskeyUser
func (u *PasskeyUser) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.ID))
}

func (u *PasskeyUser) WebAuthnName() string {
	return u.Username
}

func (u *PasskeyUser) WebAuthnDisplayName() string {
	return u.Name
}

func (u *PasskeyUser) WebAuthnIcon() string {
	return ""
}

func (u *PasskeyUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// SetupPasskeyRoutes initializes WebAuthn and sets up the routes
func SetupPasskeyRoutes(router *gin.RouterGroup, db *sql.DB) {
	// Initialize WebAuthn
	var err error
	webAuthnInstance, err = webauthn.New(&webauthn.Config{
		RPDisplayName: rpName,
		RPID:          rpDomain,
		RPOrigins:     []string{fmt.Sprintf("https://%s", rpDomain)},
	})
	if err != nil {
		log.Fatalf("Error initializing WebAuthn: %v", err)
	}

	// Create passkey credentials table if not exists
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS passkey_credentials (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		credential_id BYTEA NOT NULL UNIQUE,
		public_key BYTEA NOT NULL,
		attestation_type TEXT NOT NULL,
		aaguid BYTEA,
		sign_count BIGINT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Error creating passkey_credentials table: %v", err)
	}

	// Routes for registration
	router.POST("/register-passkey-begin", func(c *gin.Context) {
		handleBeginRegistration(c, db)
	})
	router.POST("/register-passkey-finish", func(c *gin.Context) {
		handleFinishRegistration(c, db)
	})

	// Routes for authentication
	router.POST("/login-passkey-begin", func(c *gin.Context) {
		handleBeginLogin(c, db)
	})
	router.POST("/login-passkey-finish", func(c *gin.Context) {
		handleFinishLogin(c, db)
	})
}

// Generate a session ID for temporary storage
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// Handle the beginning of passkey registration
func handleBeginRegistration(c *gin.Context, db *sql.DB) {
	// Parse request
	var req struct {
		Username string `json:"username"`
		UserID   int    `json:"user_id"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Check if the user exists in the database
	var userID int
	var userName, displayName string
	err := db.QueryRow("SELECT id, username, name FROM users WHERE id = $1", req.UserID).Scan(&userID, &userName, &displayName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Create a user for WebAuthn
	user := &PasskeyUser{
		ID:       userID,
		Username: userName,
		Name:     displayName,
	}

	// Get existing credentials
	user.Credentials = getCredentialsForUser(db, userID)

	// Create registration options and session data
	_, sessionData, err := webAuthnInstance.BeginRegistration(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to begin registration: %v", err)})
		return
	}

	// Store session data temporarily
	sessionID := generateSessionID()
	sessionStore[sessionID] = sessionData

	// Set cookie with session ID
	c.SetCookie("passkey_session", sessionID, 300, "/", rpDomain, true, true)

	// Return challenge to client
	c.JSON(http.StatusOK, gin.H{
		"challenge":      sessionData.Challenge,
		"relyingPartyId": rpDomain,
		"sessionId":      sessionID,
	})
}

// Handle the completion of passkey registration
func handleFinishRegistration(c *gin.Context, db *sql.DB) {
	// Parse request
	var req struct {
		Username     string `json:"username"`
		UserID       int    `json:"user_id"`
		CredentialID string `json:"credential_id"`
		PublicKey    string `json:"public_key"`
		ClientData   string `json:"client_data"`
		Challenge    string `json:"challenge"`
		SessionID    string `json:"session_id"`
	}

	// Log raw request body for debugging
	rawData, _ := c.GetRawData()
	log.Printf("DEBUG - Raw request body: %s", string(rawData))

	// Reset request body for binding
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawData))

	if err := c.BindJSON(&req); err != nil {
		log.Printf("ERROR - Failed to parse request JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("DEBUG - Received registration data: username=%s, userID=%d, credentialID length=%d, publicKey length=%d, clientData length=%d",
		req.Username, req.UserID, len(req.CredentialID), len(req.PublicKey), len(req.ClientData))

	// Get session ID from cookie or request
	sessionID, _ := c.Cookie("passkey_session")
	if sessionID == "" {
		sessionID = req.SessionID
		log.Printf("DEBUG - Using session ID from request: %s", sessionID)
	} else {
		log.Printf("DEBUG - Using session ID from cookie: %s", sessionID)
	}

	// Get session data
	sessionData, ok := sessionStore[sessionID]
	if !ok {
		log.Printf("ERROR - Session data not found for ID: %s", sessionID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired session"})
		return
	}
	defer delete(sessionStore, sessionID) // Remove session data when done

	// Verify the user exists
	var userID int
	var userName, displayName string
	err := db.QueryRow("SELECT id, username, name FROM users WHERE id = $1", req.UserID).Scan(&userID, &userName, &displayName)
	if err != nil {
		log.Printf("ERROR - User not found for ID: %d, Error: %v", req.UserID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Create a user for WebAuthn
	user := &PasskeyUser{
		ID:       userID,
		Username: userName,
		Name:     displayName,
	}

	// Parse client data JSON
	clientDataBytes, err := base64.StdEncoding.DecodeString(req.ClientData)
	if err != nil {
		log.Printf("ERROR - Failed to decode client data from base64: %v", err)
		log.Printf("DEBUG - Client data (first 100 chars): %s", req.ClientData[:min(len(req.ClientData), 100)])
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client data encoding"})
		return
	}

	log.Printf("DEBUG - Decoded client data: %s", string(clientDataBytes))

	// Create the credential creation response
	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(strings.NewReader(string(clientDataBytes)))
	if err != nil {
		log.Printf("ERROR - Failed to parse credential creation response: %v", err)
		log.Printf("DEBUG - Client data JSON: %s", string(clientDataBytes))
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse response: %v", err)})
		return
	}

	// Finish registration - with detailed logging
	log.Printf("DEBUG - Attempting to create credential for user %s (ID: %d)", userName, userID)
	log.Printf("DEBUG - Session data challenge: %s", sessionData.Challenge)
	log.Printf("DEBUG - Received client data (first 100 chars): %s", string(clientDataBytes)[:min(len(string(clientDataBytes)), 100)])

	credential, err := webAuthnInstance.CreateCredential(user, *sessionData, parsedResponse)
	if err != nil {
		log.Printf("ERROR - Failed to create credential: %v", err)
		log.Printf("ERROR - User ID (hex): %x", user.WebAuthnID())
		log.Printf("ERROR - Parsed response: %+v", parsedResponse)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to create credential: %v", err)})
		return
	}

	log.Printf("DEBUG - Successfully created credential: %+v", credential.ID)

	// Insert credential into database with detailed logging
	userIDStr := fmt.Sprintf("%d", userID)
	attestationType := "none"
	if credential.AttestationType != "" {
		attestationType = credential.AttestationType
	}

	log.Printf("DEBUG - Database insertion - User ID: %s", userIDStr)
	log.Printf("DEBUG - Database insertion - Credential ID (len: %d): %x", len(credential.ID), credential.ID)
	log.Printf("DEBUG - Database insertion - Public Key (len: %d): %x", len(credential.PublicKey), credential.PublicKey)
	log.Printf("DEBUG - Database insertion - Attestation Type: %s", attestationType)
	log.Printf("DEBUG - Database insertion - AAGUID: %x", credential.Authenticator.AAGUID)
	log.Printf("DEBUG - Database insertion - Sign Count: %d", credential.Authenticator.SignCount)

	_, err = db.Exec(`
		INSERT INTO passkey_credentials (user_id, credential_id, public_key, attestation_type, aaguid, sign_count)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userIDStr, credential.ID, credential.PublicKey, attestationType, credential.Authenticator.AAGUID, credential.Authenticator.SignCount)
	if err != nil {
		log.Printf("ERROR - Failed to save credential to database: %v", err)
		dbErr := fmt.Sprintf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": dbErr})
		return
	}

	log.Printf("DEBUG - Successfully saved credential to database")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey registered successfully",
	})
}

// Handle the beginning of passkey login
func handleBeginLogin(c *gin.Context, db *sql.DB) {
	// Parse request
	var req struct {
		Username string `json:"username"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Check if the user exists
	var userID int
	var userName, displayName string
	err := db.QueryRow("SELECT id, username, name FROM users WHERE username = $1", req.Username).Scan(&userID, &userName, &displayName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Create a user for WebAuthn
	user := &PasskeyUser{
		ID:       userID,
		Username: userName,
		Name:     displayName,
	}

	// Get credentials for the user
	user.Credentials = getCredentialsForUser(db, userID)
	if len(user.Credentials) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No passkeys registered for this user"})
		return
	}

	// Begin authentication - we don't need the options, just the session data
	_, sessionData, err := webAuthnInstance.BeginLogin(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to begin login: %v", err)})
		return
	}

	// Store session data temporarily
	sessionID := generateSessionID()
	sessionStore[sessionID] = sessionData

	// Set cookie with session ID
	c.SetCookie("passkey_session", sessionID, 300, "/", rpDomain, true, true)

	// Return challenge and credential IDs to the client
	credentialIDs := make([]string, len(user.Credentials))
	for i, cred := range user.Credentials {
		credentialIDs[i] = base64.StdEncoding.EncodeToString(cred.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"challenge":     sessionData.Challenge,
		"credentialIds": credentialIDs,
		"sessionId":     sessionID,
	})
}

// Handle the completion of passkey login
func handleFinishLogin(c *gin.Context, db *sql.DB) {
	// Parse request
	var req struct {
		Username          string `json:"username"`
		CredentialID      string `json:"credential_id"`
		ClientData        string `json:"client_data"`
		AuthenticatorData string `json:"authenticator_data"`
		Signature         string `json:"signature"`
		Challenge         string `json:"challenge"`
		SessionID         string `json:"session_id"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get session ID from cookie or request
	sessionID, _ := c.Cookie("passkey_session")
	if sessionID == "" {
		sessionID = req.SessionID
	}

	// Get session data
	sessionData, ok := sessionStore[sessionID]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired session"})
		return
	}
	defer delete(sessionStore, sessionID) // Remove session data when done

	// Check if the user exists
	var userID int
	var userName, displayName, role string
	err := db.QueryRow("SELECT id, username, name, role FROM users WHERE username = $1", req.Username).Scan(&userID, &userName, &displayName, &role)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Create a user for WebAuthn
	user := &PasskeyUser{
		ID:       userID,
		Username: userName,
		Name:     displayName,
	}

	// Get credentials for the user
	user.Credentials = getCredentialsForUser(db, userID)
	if len(user.Credentials) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No passkeys registered for this user"})
		return
	}

	// Decode credential ID
	credentialIDBytes, err := base64.StdEncoding.DecodeString(req.CredentialID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	// Parse client data - we only need this for the assertion
	clientDataBytes, err := base64.StdEncoding.DecodeString(req.ClientData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid client data"})
		return
	}

	// Parse the assertion response
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(strings.NewReader(string(clientDataBytes)))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse response: %v", err)})
		return
	}

	// Validate the assertion
	_, err = webAuthnInstance.ValidateLogin(user, *sessionData, parsedResponse)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Invalid assertion: %v", err)})
		return
	}

	// Get additional roles for the user
	var additionalRoles []string
	rows, err := db.Query("SELECT role FROM user_roles WHERE user_id = $1", userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var additionalRole string
			if err := rows.Scan(&additionalRole); err == nil {
				additionalRoles = append(additionalRoles, additionalRole)
			}
		}
	}

	// Update sign count in the database
	userIDStr := fmt.Sprintf("%d", userID)
	_, err = db.Exec("UPDATE passkey_credentials SET sign_count = sign_count + 1 WHERE user_id = $1 AND credential_id = $2", userIDStr, credentialIDBytes)
	if err != nil {
		log.Printf("Failed to update sign count: %v", err)
	}

	// Successful login
	c.JSON(http.StatusOK, gin.H{
		"id":               userID,
		"username":         userName,
		"name":             displayName,
		"role":             role,
		"additional_roles": additionalRoles,
	})
}

// Get passkey credentials for a user from the database
func getCredentialsForUser(db *sql.DB, userID int) []webauthn.Credential {
	userIDStr := fmt.Sprintf("%d", userID)
	rows, err := db.Query(`
		SELECT credential_id, public_key, attestation_type, aaguid, sign_count
		FROM passkey_credentials
		WHERE user_id = $1
	`, userIDStr)
	if err != nil {
		log.Printf("Error getting credentials: %v", err)
		return nil
	}
	defer rows.Close()

	credentials := []webauthn.Credential{}
	for rows.Next() {
		var credentialID, publicKey, aaguid []byte
		var attestationType string
		var signCount int64
		err := rows.Scan(&credentialID, &publicKey, &attestationType, &aaguid, &signCount)
		if err != nil {
			log.Printf("Error scanning credential: %v", err)
			continue
		}

		// Create a credential
		credential := webauthn.Credential{
			ID:              credentialID,
			PublicKey:       publicKey,
			AttestationType: attestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:    aaguid,
				SignCount: uint32(signCount),
			},
		}
		credentials = append(credentials, credential)
	}

	return credentials
}
