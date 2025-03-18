package routes

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"github.com/gin-gonic/gin"
	"server/models"
	"server/db" // Ensure your DB package is imported correctly (note: package name is lowercase "db")
)

// RegisterLoginRoute registers the login route.
func RegisterLoginRoute(router *gin.Engine) {
	router.POST("/login", loginHandler)
}

// loginHandler processes the login request.
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
	query := "SELECT id, username, name, password, role FROM users WHERE username = $1"
	err = conn.QueryRow(query, loginData.Username).Scan(&user.ID, &user.Username, &user.Name, &user.Password, &user.Role)
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

	// Return user data (including password as requested)
	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"password": user.Password,
		"name":     user.Name,
		"role":     user.Role,
	})
}
