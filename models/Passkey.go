package models

import (
	"database/sql"
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebauthnUser represents a user for the webauthn authentication flow
type WebauthnUser struct {
	ID              int
	Username        string
	DisplayName     string
	UserID          string // String representation of user ID for WebAuthn
	Credentials     []webauthn.Credential
	PasskeyVerified bool
}

// WebAuthnID returns the user's ID as a []byte
func (u *WebauthnUser) WebAuthnID() []byte {
	return []byte(u.UserID)
}

// WebAuthnName returns the user's username
func (u *WebauthnUser) WebAuthnName() string {
	return u.Username
}

// WebAuthnDisplayName returns the user's display name
func (u *WebauthnUser) WebAuthnDisplayName() string {
	return u.DisplayName
}

// WebAuthnIcon returns the URL to the user's icon
func (u *WebauthnUser) WebAuthnIcon() string {
	return ""
}

// WebAuthnCredentials returns the user's credentials
func (u *WebauthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// AddCredential adds a credential to the user
func (u *WebauthnUser) AddCredential(cred webauthn.Credential) {
	u.Credentials = append(u.Credentials, cred)
}

// PasskeyCredential represents a stored passkey credential
type PasskeyCredential struct {
	ID           int    `json:"id"`
	UserID       string `json:"user_id"`
	CredentialID []byte `json:"credential_id"`
	PublicKey    []byte `json:"public_key"`
	SignCount    int64  `json:"sign_count"`
	CreatedAt    string `json:"created_at"`
}

// GetUserForWebAuthn retrieves a user by their username and prepares it for WebAuthn
func GetUserForWebAuthn(db *sql.DB, username string) (*WebauthnUser, error) {
	var user User
	query := `
		SELECT id, username, name
		FROM users
		WHERE username = $1
	`
	err := db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Name,
	)
	if err != nil {
		return nil, err
	}

	// Convert user to WebauthnUser
	webAuthnUser := &WebauthnUser{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.Name,
		UserID:      fmt.Sprintf("%d", user.ID),
	}

	// Fetch all credentials for this user
	credQuery := `
		SELECT credential_id, public_key, sign_count
		FROM passkey_credentials
		WHERE user_id = $1
	`
	rows, err := db.Query(credQuery, webAuthnUser.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cred webauthn.Credential
		err := rows.Scan(
			&cred.ID,
			&cred.PublicKey,
			&cred.Authenticator.SignCount,
		)
		if err != nil {
			return nil, err
		}
		webAuthnUser.Credentials = append(webAuthnUser.Credentials, cred)
	}

	return webAuthnUser, nil
}

// SavePasskeyCredential saves a passkey credential to the database
func SavePasskeyCredential(db *sql.DB, userID string, cred webauthn.Credential) error {
	query := `
		INSERT INTO passkey_credentials (user_id, credential_id, public_key, sign_count)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	_, err := db.Exec(query, userID, cred.ID, cred.PublicKey, cred.Authenticator.SignCount)
	return err
}

// UpdatePasskeyCredential updates the sign count for a credential
func UpdatePasskeyCredential(db *sql.DB, credentialID []byte, signCount uint32) error {
	query := `
		UPDATE passkey_credentials
		SET sign_count = $1
		WHERE credential_id = $2
	`
	_, err := db.Exec(query, signCount, credentialID)
	return err
}

// GetCredentialByID retrieves a credential by its ID
func GetCredentialByID(db *sql.DB, credentialID []byte) (*webauthn.Credential, error) {
	var cred webauthn.Credential
	query := `
		SELECT credential_id, public_key, sign_count
		FROM passkey_credentials
		WHERE credential_id = $1
	`
	err := db.QueryRow(query, credentialID).Scan(
		&cred.ID,
		&cred.PublicKey,
		&cred.Authenticator.SignCount,
	)
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// HasPasskey checks if a user has any registered passkeys
func HasPasskey(db *sql.DB, userID string) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) 
		FROM passkey_credentials
		WHERE user_id = $1
	`
	err := db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
