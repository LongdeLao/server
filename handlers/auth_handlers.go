package handlers

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	stdrand "math/rand"
	"net/http"
	"net/smtp"
	"server/config"
	"server/models"
	"server/utils"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Initialize the package
func init() {
	// Seed the random number generator with a truly random seed
	seed, err := rand.Int(rand.Reader, big.NewInt(1<<63-1))
	if err != nil {
		// Fallback to time-based seed if crypto random fails
		stdrand.Seed(time.Now().UnixNano())
		log.Printf("Warning: Using time-based random seed: %v", err)
	} else {
		stdrand.Seed(seed.Int64())
		log.Printf("Random number generator initialized with crypto-secure seed")
	}

	// Start the cleanup routine for rate limiting and reset codes
	go cleanupRoutine()
}

// In-memory storage for verification codes
var resetCodes = make(map[string]ResetCodeInfo)

// Rate limiting data
var (
	// Track reset attempts by email
	resetAttempts = make(map[string][]time.Time)
	// Track reset attempts by IP
	ipAttempts = make(map[string][]time.Time)
	// Mutex for thread safety
	rateLimitMutex sync.Mutex
)

// Rate limiting constants
const (
	// Maximum attempts per email in the time window
	maxAttemptsPerEmail = 3
	// Maximum attempts per IP in the time window
	maxAttemptsPerIP = 10
	// Time window for rate limiting (in minutes)
	rateLimitWindow = 30
	// Cooldown between attempts for the same email (in minutes)
	emailCooldown = 1
)

// ResetCodeInfo stores password reset information
type ResetCodeInfo struct {
	Email     string
	Code      string
	CreatedAt time.Time
	SessionID string
}

// PasswordResetRequest represents the request to reset a password
type PasswordResetRequest struct {
	Email string `json:"email" binding:"required"`
}

// VerifyCodeRequest represents the request to verify a reset code
type VerifyCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
}

// ResetPasswordRequest represents the request to set a new password
type ResetPasswordRequest struct {
	SessionID   string `json:"session_id" binding:"required"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// cleanupRoutine periodically cleans up expired entries in rate limiting maps and reset codes
func cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// Clean up rate limiting maps
		rateLimitMutex.Lock()

		// Clean email attempts
		for email, attempts := range resetAttempts {
			var validAttempts []time.Time
			for _, t := range attempts {
				if now.Sub(t).Minutes() < rateLimitWindow {
					validAttempts = append(validAttempts, t)
				}
			}
			if len(validAttempts) == 0 {
				delete(resetAttempts, email)
			} else {
				resetAttempts[email] = validAttempts
			}
		}

		// Clean IP attempts
		for ip, attempts := range ipAttempts {
			var validAttempts []time.Time
			for _, t := range attempts {
				if now.Sub(t).Minutes() < rateLimitWindow {
					validAttempts = append(validAttempts, t)
				}
			}
			if len(validAttempts) == 0 {
				delete(ipAttempts, ip)
			} else {
				ipAttempts[ip] = validAttempts
			}
		}
		rateLimitMutex.Unlock()

		// Clean up expired reset codes
		for sessionID, info := range resetCodes {
			if now.Sub(info.CreatedAt).Minutes() > 15 {
				delete(resetCodes, sessionID)
			}
		}

		log.Printf("Cleanup routine executed: removed expired rate limits and reset codes")
	}
}

// isRateLimited checks if the email or IP is currently rate limited
func isRateLimited(email, ip string) (bool, time.Time, error) {
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()

	now := time.Now()

	// Check email cooldown - must wait at least emailCooldown minutes between attempts
	emailTimes := resetAttempts[email]
	if len(emailTimes) > 0 {
		lastAttempt := emailTimes[len(emailTimes)-1]
		timeSinceLastAttempt := now.Sub(lastAttempt).Minutes()
		if timeSinceLastAttempt < emailCooldown {
			nextAllowedTime := lastAttempt.Add(time.Duration(emailCooldown) * time.Minute)
			return true, nextAllowedTime, fmt.Errorf("too many attempts for this email, please wait %d seconds",
				int(nextAllowedTime.Sub(now).Seconds()))
		}
	}

	// Filter attempts within the time window for email
	var recentEmailAttempts []time.Time
	for _, t := range emailTimes {
		if now.Sub(t).Minutes() < rateLimitWindow {
			recentEmailAttempts = append(recentEmailAttempts, t)
		}
	}

	// Check if email has exceeded maximum attempts
	if len(recentEmailAttempts) >= maxAttemptsPerEmail {
		oldestAttempt := recentEmailAttempts[0]
		nextAllowedTime := oldestAttempt.Add(time.Duration(rateLimitWindow) * time.Minute)
		return true, nextAllowedTime, fmt.Errorf("maximum password reset attempts reached for this email, please try again later")
	}

	// Filter attempts within the time window for IP
	var recentIPAttempts []time.Time
	for _, t := range ipAttempts[ip] {
		if now.Sub(t).Minutes() < rateLimitWindow {
			recentIPAttempts = append(recentIPAttempts, t)
		}
	}

	// Check if IP has exceeded maximum attempts
	if len(recentIPAttempts) >= maxAttemptsPerIP {
		oldestAttempt := recentIPAttempts[0]
		nextAllowedTime := oldestAttempt.Add(time.Duration(rateLimitWindow) * time.Minute)
		return true, nextAllowedTime, fmt.Errorf("too many password reset attempts from your location, please try again later")
	}

	// Not rate limited
	return false, time.Time{}, nil
}

// recordAttempt records an attempt for rate limiting purposes
func recordAttempt(email, ip string) {
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()

	now := time.Now()

	// Record email attempt
	resetAttempts[email] = append(resetAttempts[email], now)

	// Record IP attempt
	ipAttempts[ip] = append(ipAttempts[ip], now)
}

// RequestPasswordReset handles the request to reset a password
func RequestPasswordReset(c *gin.Context) {
	var req PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Normalize email to lowercase
	email := strings.ToLower(req.Email)
	log.Printf("Password reset requested for email: %s", email)

	// Get client IP for rate limiting
	clientIP := c.ClientIP()

	// Check rate limiting
	limited, nextAllowedTime, err := isRateLimited(email, clientIP)
	if limited {
		log.Printf("Rate limited password reset attempt for email %s from IP %s: %v", email, clientIP, err)

		// Calculate wait time
		waitSeconds := int(nextAllowedTime.Sub(time.Now()).Seconds())
		if waitSeconds < 0 {
			waitSeconds = 60
		}

		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":        err.Error(),
			"wait_seconds": waitSeconds,
		})
		return
	}

	// Record this attempt regardless of success to prevent email enumeration attacks
	recordAttempt(email, clientIP)

	// Connect to the database to check if user exists
	db, err := getDB(c)
	if err != nil {
		log.Printf("Failed to get database connection: %v", err)
		return
	}

	// Check if user exists with this email
	exists, err := models.UserExistsByEmail(db, email)
	if err != nil {
		log.Printf("Database error checking email existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !exists {
		log.Printf("Password reset attempted for non-existent email: %s", email)
		// Return an explicit error that the user doesn't exist
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No account exists with this email address",
		})
		return
	}

	// User exists, proceed with the reset process
	log.Printf("Email %s is registered, generating reset code", email)

	// Generate a cryptographically secure random 6-digit code
	code := generateSecureCode(6)
	log.Printf("Generated secure 6-digit code: %s", code)

	// Generate a session ID
	sessionID := generateSessionID()

	// Store the code with a timestamp
	resetCodes[sessionID] = ResetCodeInfo{
		Email:     email,
		Code:      code,
		CreatedAt: time.Now(),
		SessionID: sessionID,
	}

	// In a real application, send the code via email
	emailSent := sendResetCodeEmail(email, code)

	if emailSent {
		log.Printf("Password reset code sent successfully to %s. Code: %s, Session ID: %s", email, code, sessionID)
	} else {
		log.Printf("Failed to send password reset code to %s", email)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to send reset code email. Please try again later.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Reset code sent to email",
		"session_id": sessionID,
	})
}

// VerifyResetCode verifies if the provided code is valid
func VerifyResetCode(c *gin.Context) {
	var req VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid verification code request format: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("Verification attempt for session ID: %s", req.SessionID)

	// Check if the session ID exists
	info, exists := resetCodes[req.SessionID]
	if !exists {
		log.Printf("Invalid or expired session ID: %s", req.SessionID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired session"})
		return
	}

	// Check if the code is valid
	if info.Code != req.Code {
		log.Printf("Invalid verification code provided for email %s. Expected: %s, Got: %s",
			info.Email, info.Code, req.Code)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid verification code"})
		return
	}

	// Check if the code has expired (15 minutes)
	timeElapsed := time.Since(info.CreatedAt).Minutes()
	if timeElapsed > 15 {
		log.Printf("Expired verification code for email %s. Code age: %.2f minutes",
			info.Email, timeElapsed)
		delete(resetCodes, req.SessionID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code has expired"})
		return
	}

	log.Printf("Verification code successfully validated for email %s", info.Email)
	c.JSON(http.StatusOK, gin.H{
		"message": "Code verified successfully",
		"email":   info.Email,
	})
}

// ResetPassword resets the user's password after code verification
func ResetPassword(c *gin.Context, db *sql.DB) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid reset password request format: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("Password reset attempt for session ID: %s", req.SessionID)

	// Password strength validation
	if len(req.NewPassword) < 8 {
		log.Printf("Password reset failed: Password too short (length: %d)", len(req.NewPassword))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters long"})
		return
	}

	// Check if the session ID exists
	info, exists := resetCodes[req.SessionID]
	if !exists {
		log.Printf("Password reset failed: Invalid or expired session ID: %s", req.SessionID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired session"})
		return
	}

	// Check if the code is valid
	if info.Code != req.Code {
		log.Printf("Password reset failed: Invalid code for email %s", info.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid verification code"})
		return
	}

	// Check if the code has expired (15 minutes)
	timeElapsed := time.Since(info.CreatedAt).Minutes()
	if timeElapsed > 15 {
		log.Printf("Password reset failed: Code expired for email %s. Age: %.2f minutes",
			info.Email, timeElapsed)
		delete(resetCodes, req.SessionID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code has expired"})
		return
	}

	// Hash the new password
	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		log.Printf("Password reset failed: Error hashing password for email %s: %v", info.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Update the password in the database
	if err := updateUserPassword(db, info.Email, hashedPassword); err != nil {
		log.Printf("Password reset failed: Database error updating password for email %s: %v", info.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Delete the reset code from memory
	delete(resetCodes, req.SessionID)

	log.Printf("Password reset successful for email %s", info.Email)
	c.JSON(http.StatusOK, gin.H{
		"message": "Password has been reset successfully",
	})
}

// Helper functions

// getDB gets the database connection from the gin context
func getDB(c *gin.Context) (*sql.DB, error) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection not available"})
		return nil, fmt.Errorf("database connection not available")
	}
	return db.(*sql.DB), nil
}

// generateSessionID generates a cryptographically secure random session ID
func generateSessionID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 32
	b := make([]byte, length)

	// Use crypto/rand for secure random number generation
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to math/rand if crypto/rand fails
		log.Printf("Warning: Using less secure random for session ID: %v", err)
		for i := range b {
			b[i] = charset[stdrand.Intn(len(charset))]
		}
		return string(b)
	}

	// Map random bytes to charset
	for i := 0; i < length; i++ {
		b[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return string(b)
}

// sendResetCodeEmail sends an email with the reset code
// Returns true if the email was sent successfully, false otherwise
func sendResetCodeEmail(email, code string) bool {
	log.Printf("Preparing to send reset code %s to %s", code, email)

	// Setup the email message
	to := []string{email}
	subject := "HSANNU Connect - Password Reset Code"

	// Construct the message body (text version)
	body := fmt.Sprintf(`
Dear User,

You have requested to reset your password for your HSANNU Connect account.
Please use the following verification code to complete the process:

%s

This code will expire in 15 minutes.

If you did not request a password reset, please ignore this email.

Best regards,
HSANNU Connect Support Team
`, code)

	// Build the email with correct headers
	mime := "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n"
	message := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"%s\r\n"+
		"%s",
		config.SMTPSender,
		email,
		subject,
		mime,
		body)

	// Set up authentication information.
	auth := smtp.PlainAuth("",
		config.SMTPUsername,
		config.SMTPPassword,
		config.SMTPHost)

	// Connect to the server, authenticate, and send the email
	smtpAddr := fmt.Sprintf("%s:%s", config.SMTPHost, config.SMTPPort)

	log.Printf("Attempting to send email to %s via SMTP server %s", email, smtpAddr)

	// Send the email
	err := smtp.SendMail(
		smtpAddr,
		auth,
		config.SMTPUsername,
		to,
		[]byte(message),
	)

	if err != nil {
		log.Printf("Error sending email to %s: %v", email, err)
		return false
	}

	log.Printf("Email sent successfully to %s", email)
	return true
}

// updateUserPassword updates a user's password in the database
func updateUserPassword(db *sql.DB, email string, hashedPassword string) error {
	log.Printf("Attempting to update password for user with email: %s", email)

	query := `UPDATE users SET password = $1 WHERE email = $2`
	result, err := db.Exec(query, hashedPassword, email)
	if err != nil {
		log.Printf("Database error during password update for %s: %v", email, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected for password update: %v", err)
		return err
	}

	if rowsAffected == 0 {
		log.Printf("No rows were updated for email %s. User may not exist anymore.", email)
		return fmt.Errorf("no user found with email %s", email)
	}

	log.Printf("Successfully updated password for user with email: %s. Rows affected: %d", email, rowsAffected)
	return nil
}

// generateSecureCode generates a cryptographically secure random numeric code of specified length
func generateSecureCode(length int) string {
	const digits = "0123456789"
	result := make([]byte, length)

	// Generate random bytes
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to math/rand if crypto/rand fails
		log.Printf("Warning: Using less secure random for code generation: %v", err)
		for i := range result {
			result[i] = digits[stdrand.Intn(len(digits))]
		}
		return string(result)
	}

	// Map random bytes to digits
	for i := 0; i < length; i++ {
		result[i] = digits[int(randomBytes[i])%len(digits)]
	}

	return string(result)
}
