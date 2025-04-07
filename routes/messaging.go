package routes

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"server/notifications"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Conversation struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Users     []User    `json:"users"`
}

type Message struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	SenderID       int       `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
	Read           bool      `json:"read"`
}

type User struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Name      string `json:"name"`
	Role      string `json:"role"`
}

// GetUserConversations retrieves all conversations for a specific user
// GET /api/messaging/conversations/:user_id
func GetUserConversations(c *gin.Context, db *sql.DB) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "User ID is required",
		})
		return
	}

	// Convert user ID from string to integer
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid user ID format: %s", userID),
		})
		return
	}

	// First, check if the user exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userIDInt).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking if user exists: %v", err),
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("User not found with ID: %d", userIDInt),
		})
		return
	}

	// Get all conversations for the user with the most recent message
	query := `
		SELECT 
			c.id, 
			c.created_at
		FROM 
			conversations c
		JOIN 
			conversation_participants cp ON c.id = cp.conversation_id
		WHERE 
			cp.user_id = $1
		ORDER BY 
			c.created_at DESC
	`

	rows, err := db.Query(query, userIDInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying conversations: %v", err),
		})
		return
	}
	defer rows.Close()

	var conversations []gin.H

	for rows.Next() {
		var conversation Conversation
		err := rows.Scan(
			&conversation.ID,
			&conversation.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning conversation: %v", err),
			})
			return
		}

		// Get all users in this conversation
		usersQuery := `
			SELECT 
				u.id, 
				u.first_name, 
				u.last_name, 
				u.name, 
				u.role
			FROM 
				users u
			JOIN 
				conversation_participants cp ON u.id = cp.user_id
			WHERE 
				cp.conversation_id = $1 AND u.id != $2
		`

		userRows, err := db.Query(usersQuery, conversation.ID, userIDInt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error querying users for conversation: %v", err),
			})
			return
		}
		defer userRows.Close()

		var users []User
		for userRows.Next() {
			var user User
			err := userRows.Scan(
				&user.ID,
				&user.FirstName,
				&user.LastName,
				&user.Name,
				&user.Role,
			)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("Error scanning user: %v", err),
				})
				return
			}

			users = append(users, user)
		}

		// Get the latest message in this conversation
		var latestMessage Message
		var hasMessages bool

		messageQuery := `
			SELECT 
				m.id, 
				m.sender_id, 
				u.name as sender_name, 
				m.content, 
				m.created_at, 
				m.read
			FROM 
				messages m
			JOIN 
				users u ON m.sender_id = u.id
			WHERE 
				m.conversation_id = $1
			ORDER BY 
				m.created_at DESC
			LIMIT 1
		`

		err = db.QueryRow(messageQuery, conversation.ID).Scan(
			&latestMessage.ID,
			&latestMessage.SenderID,
			&latestMessage.SenderName,
			&latestMessage.Content,
			&latestMessage.CreatedAt,
			&latestMessage.Read,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				hasMessages = false
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("Error querying latest message: %v", err),
				})
				return
			}
		} else {
			hasMessages = true
		}

		// Count unread messages
		var unreadCount int
		unreadQuery := `
			SELECT 
				COUNT(*) 
			FROM 
				messages 
			WHERE 
				conversation_id = $1 AND sender_id != $2 AND read = false
		`

		err = db.QueryRow(unreadQuery, conversation.ID, userIDInt).Scan(&unreadCount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error counting unread messages: %v", err),
			})
			return
		}

		conversationData := gin.H{
			"id":           conversation.ID,
			"created_at":   conversation.CreatedAt,
			"participants": users,
			"unread_count": unreadCount,
		}

		if hasMessages {
			conversationData["latest_message"] = gin.H{
				"id":         latestMessage.ID,
				"sender_id":  latestMessage.SenderID,
				"sender":     latestMessage.SenderName,
				"content":    latestMessage.Content,
				"created_at": latestMessage.CreatedAt,
				"read":       latestMessage.Read,
			}
		} else {
			conversationData["latest_message"] = nil
		}

		conversations = append(conversations, conversationData)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"conversations": conversations,
	})
}

// GetConversationMessages retrieves all messages for a specific conversation
// GET /api/messaging/conversation/:conversation_id/messages
func GetConversationMessages(c *gin.Context, db *sql.DB) {
	conversationID := c.Param("conversation_id")
	userID := c.Query("user_id")

	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Conversation ID is required",
		})
		return
	}

	// Convert conversation ID from string to integer
	conversationIDInt, err := strconv.Atoi(conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid conversation ID format: %s", conversationID),
		})
		return
	}

	// Convert user ID from string to integer if provided
	var userIDInt int
	if userID != "" {
		userIDInt, err = strconv.Atoi(userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid user ID format: %s", userID),
			})
			return
		}

		// Mark messages as read for this user
		if userIDInt > 0 {
			_, err = db.Exec(`
				UPDATE messages
				SET read = true
				WHERE conversation_id = $1 AND sender_id != $2 AND read = false
			`, conversationIDInt, userIDInt)

			if err != nil {
				fmt.Printf("Error marking messages as read: %v\n", err)
				// Continue anyway, this is not a critical error
			}
		}
	}

	// Get all messages for the conversation
	query := `
		SELECT 
			m.id, 
			m.conversation_id,
			m.sender_id, 
			u.name as sender_name, 
			m.content, 
			m.created_at, 
			m.read
		FROM 
			messages m
		JOIN 
			users u ON m.sender_id = u.id
		WHERE 
			m.conversation_id = $1
		ORDER BY 
			m.created_at ASC
	`

	rows, err := db.Query(query, conversationIDInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying messages: %v", err),
		})
		return
	}
	defer rows.Close()

	var messages []gin.H

	for rows.Next() {
		var message Message
		err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.SenderID,
			&message.SenderName,
			&message.Content,
			&message.CreatedAt,
			&message.Read,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning message: %v", err),
			})
			return
		}

		messages = append(messages, gin.H{
			"id":              message.ID,
			"conversation_id": message.ConversationID,
			"sender_id":       message.SenderID,
			"sender":          message.SenderName,
			"content":         message.Content,
			"created_at":      message.CreatedAt,
			"read":            message.Read,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"messages": messages,
	})
}

// SendMessage sends a new message in a conversation
// POST /api/messaging/messages
func SendMessage(c *gin.Context, db *sql.DB) {
	// Log the raw request body for debugging
	body, _ := c.GetRawData()
	fmt.Printf("SendMessage raw request body: %s\n", string(body))

	// Reset the request body so it can be read again
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var request struct {
		ConversationID int    `json:"conversation_id"`
		SenderID       int    `json:"sender_id"`
		Content        string `json:"content"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		fmt.Printf("SendMessage error binding JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request format: %v", err),
		})
		return
	}

	fmt.Printf("SendMessage parsed request: %+v\n", request)

	// Validate the request
	if request.ConversationID <= 0 {
		fmt.Printf("SendMessage error: Invalid conversation ID: %d\n", request.ConversationID)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Conversation ID is required",
		})
		return
	}

	if request.SenderID <= 0 {
		fmt.Printf("SendMessage error: Invalid sender ID: %d\n", request.SenderID)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Sender ID is required",
		})
		return
	}

	if request.Content == "" {
		fmt.Printf("SendMessage error: Empty message content\n")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Message content is required",
		})
		return
	}

	// Check if conversation exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM conversations WHERE id = $1)", request.ConversationID).Scan(&exists)
	if err != nil {
		fmt.Printf("SendMessage error checking if conversation exists: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking if conversation exists: %v", err),
		})
		return
	}

	if !exists {
		fmt.Printf("SendMessage error: Conversation not found with ID: %d\n", request.ConversationID)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("Conversation not found with ID: %d", request.ConversationID),
		})
		return
	}

	// Check if user is a participant in the conversation
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM conversation_participants WHERE conversation_id = $1 AND user_id = $2)",
		request.ConversationID, request.SenderID).Scan(&exists)
	if err != nil {
		fmt.Printf("SendMessage error checking if user is a participant: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking if user is a participant: %v", err),
		})
		return
	}

	if !exists {
		fmt.Printf("SendMessage error: User %d is not a participant in conversation %d\n",
			request.SenderID, request.ConversationID)
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "User is not a participant in this conversation",
		})
		return
	}

	// Insert the message
	var messageID int
	var createdAt time.Time
	err = db.QueryRow(`
		INSERT INTO messages (conversation_id, sender_id, content, created_at, read) 
		VALUES ($1, $2, $3, (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::timestamp, false) 
		RETURNING id, created_at
	`, request.ConversationID, request.SenderID, request.Content).Scan(&messageID, &createdAt)

	if err != nil {
		fmt.Printf("SendMessage error inserting message: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error inserting message: %v", err),
		})
		return
	}

	fmt.Printf("SendMessage successfully inserted message with ID: %d\n", messageID)

	// Get the sent message with sender info
	var message Message
	err = db.QueryRow(`
		SELECT 
			m.id, 
			m.conversation_id,
			m.sender_id, 
			u.name as sender_name, 
			m.content, 
			m.created_at, 
			m.read
		FROM 
			messages m
		JOIN 
			users u ON m.sender_id = u.id
		WHERE 
			m.id = $1
	`, messageID).Scan(
		&message.ID,
		&message.ConversationID,
		&message.SenderID,
		&message.SenderName,
		&message.Content,
		&message.CreatedAt,
		&message.Read,
	)

	if err != nil {
		fmt.Printf("SendMessage error retrieving sent message: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error retrieving sent message: %v", err),
		})
		return
	}

	// Send push notifications to all other participants in the conversation
	go sendPushNotifications(db, request.ConversationID, request.SenderID, message.SenderName, request.Content)

	fmt.Printf("SendMessage successful for message ID: %d\n", messageID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": gin.H{
			"id":              message.ID,
			"conversation_id": message.ConversationID,
			"sender_id":       message.SenderID,
			"sender":          message.SenderName,
			"content":         message.Content,
			"created_at":      message.CreatedAt,
			"read":            message.Read,
		},
	})
}

// CreateConversation creates a new conversation between users
// POST /api/messaging/conversations
func CreateConversation(c *gin.Context, db *sql.DB) {
	var request struct {
		UserIDs []int `json:"user_ids"`
	}

	// Log the raw request body for debugging
	body, _ := c.GetRawData()
	fmt.Printf("Raw request body: %s\n", string(body))

	// Reset the request body so it can be read again
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	if err := c.ShouldBindJSON(&request); err != nil {
		fmt.Printf("Error binding JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request format: %v", err),
		})
		return
	}

	fmt.Printf("Parsed user IDs: %v\n", request.UserIDs)

	// Validate the request
	if len(request.UserIDs) < 2 {
		fmt.Printf("Error: Not enough users. Received %d users\n", len(request.UserIDs))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "At least two users are required for a conversation",
		})
		return
	}

	// Check if users exist and collect their roles
	userRoles := make(map[int]string)
	for _, userID := range request.UserIDs {
		var role string
		err := db.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("Error: User ID %d not found\n", userID)
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"message": fmt.Sprintf("User not found with ID: %d", userID),
				})
				return
			}

			fmt.Printf("Error checking user role for ID %d: %v\n", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error checking user role: %v", err),
			})
			return
		}

		userRoles[userID] = role
	}

	fmt.Printf("User roles: %v\n", userRoles)

	// Check if we have a valid conversation pair (student-staff)
	var hasStudent, hasStaff bool
	for _, role := range userRoles {
		if role == "student" {
			hasStudent = true
		} else if role == "staff" {
			hasStaff = true
		}
	}

	if !hasStudent || !hasStaff {
		fmt.Printf("Error: Missing required roles. Student: %v, Staff: %v\n", hasStudent, hasStaff)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Conversations must include at least one student and one staff member",
		})
		return
	}

	// Check if there's already a conversation between these users
	// For a conversation between exactly these users (no more, no fewer)
	if len(request.UserIDs) == 2 {
		query := `
			SELECT c.id
			FROM conversations c
			JOIN conversation_participants cp1 ON c.id = cp1.conversation_id AND cp1.user_id = $1
			JOIN conversation_participants cp2 ON c.id = cp2.conversation_id AND cp2.user_id = $2
			GROUP BY c.id
			HAVING COUNT(DISTINCT cp1.user_id) + COUNT(DISTINCT cp2.user_id) = 2
		`

		var existingConversationID int
		err := db.QueryRow(query, request.UserIDs[0], request.UserIDs[1]).Scan(&existingConversationID)
		if err == nil {
			// Conversation exists, return it
			c.JSON(http.StatusOK, gin.H{
				"success":         true,
				"conversation_id": existingConversationID,
				"message":         "Conversation already exists",
			})
			return
		} else if err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error checking existing conversation: %v", err),
			})
			return
		}
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error starting transaction: %v", err),
		})
		return
	}

	// Create a new conversation
	var conversationID int
	err = tx.QueryRow(`
		INSERT INTO conversations (created_at) 
		VALUES ((CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::timestamp) 
		RETURNING id
	`).Scan(&conversationID)

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error creating conversation: %v", err),
		})
		return
	}

	// Add all users to the conversation
	for _, userID := range request.UserIDs {
		_, err := tx.Exec(`
			INSERT INTO conversation_participants (conversation_id, user_id) 
			VALUES ($1, $2)
		`, conversationID, userID)

		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error adding user to conversation: %v", err),
			})
			return
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error committing transaction: %v", err),
		})
		return
	}

	// Get user details for all participants
	query := `
		SELECT 
			u.id, 
			u.first_name, 
			u.last_name, 
			u.name, 
			u.role
		FROM 
			users u
		JOIN 
			conversation_participants cp ON u.id = cp.user_id
		WHERE 
			cp.conversation_id = $1
	`

	rows, err := db.Query(query, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying users for conversation: %v", err),
		})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&user.Name,
			&user.Role,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning user: %v", err),
			})
			return
		}

		users = append(users, user)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"conversation_id": conversationID,
		"participants":    users,
	})
}

// GetAvailableChatUsers returns users that a student can chat with (teachers)
// or users that a teacher can chat with (students)
// GET /api/messaging/chat-users/:user_id
func GetAvailableChatUsers(c *gin.Context, db *sql.DB) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "User ID is required",
		})
		return
	}

	fmt.Printf("GetAvailableChatUsers: Processing request for user ID: %s\n", userID)

	// Convert user ID from string to integer
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		fmt.Printf("GetAvailableChatUsers: Invalid user ID format: %s, error: %v\n", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid user ID format: %s", userID),
		})
		return
	}

	// Check if user exists and get their role
	var userRole string
	err = db.QueryRow("SELECT role FROM users WHERE id = $1", userIDInt).Scan(&userRole)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("GetAvailableChatUsers: User not found with ID: %d\n", userIDInt)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": fmt.Sprintf("User not found with ID: %d", userIDInt),
			})
			return
		}

		fmt.Printf("GetAvailableChatUsers: Error checking user role: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking user role: %v", err),
		})
		return
	}

	fmt.Printf("GetAvailableChatUsers: User %d has role: %s\n", userIDInt, userRole)

	var query string
	var availableRole string

	// If user is a student, they can chat with any staff member
	// If user is a staff member (teacher/admin), they can chat with students
	if userRole == "student" {
		query = `
			SELECT 
				u.id, 
				u.first_name, 
				u.last_name, 
				u.name, 
				u.role,
				u.profile_picture,
				string_agg(DISTINCT ar.role, ', ') as additional_roles
			FROM 
				users u
			LEFT JOIN
				additional_roles ar ON u.id = ar.user_id
			WHERE 
				u.role = 'staff'
			GROUP BY
				u.id, u.first_name, u.last_name, u.name, u.role, u.profile_picture
			ORDER BY 
				u.name
		`
		availableRole = "staff"
		fmt.Printf("GetAvailableChatUsers: Student user, looking for staff members\n")
	} else if userRole == "staff" {
		query = `
			SELECT 
				u.id, 
				u.first_name, 
				u.last_name, 
				u.name, 
				u.role,
				u.profile_picture,
				NULL as additional_roles
			FROM 
				users u
			WHERE 
				u.role = 'student'
			ORDER BY 
				u.name
		`
		availableRole = "students"
		fmt.Printf("GetAvailableChatUsers: Staff user, looking for student members\n")
	} else {
		fmt.Printf("GetAvailableChatUsers: Invalid user role: %s\n", userRole)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid user role: %s", userRole),
		})
		return
	}

	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("GetAvailableChatUsers: Error querying users: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying users: %v", err),
		})
		return
	}
	defer rows.Close()

	var users []gin.H
	for rows.Next() {
		var user struct {
			ID             int            `json:"id"`
			FirstName      string         `json:"first_name"`
			LastName       string         `json:"last_name"`
			Name           string         `json:"name"`
			Role           string         `json:"role"`
			ProfilePicture sql.NullString `json:"profile_picture"`
		}
		var additionalRoles sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&user.Name,
			&user.Role,
			&user.ProfilePicture,
			&additionalRoles,
		)

		if err != nil {
			fmt.Printf("GetAvailableChatUsers: Error scanning user: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning user: %v", err),
			})
			return
		}

		// Create user object with additional roles
		userObj := gin.H{
			"id":         user.ID,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"name":       user.Name,
			"role":       user.Role,
		}

		// Add profile picture if present
		if user.ProfilePicture.Valid {
			userObj["profile_picture"] = user.ProfilePicture.String
		}

		// Add additional roles if present
		if additionalRoles.Valid && additionalRoles.String != "" {
			userObj["additional_roles"] = strings.Split(additionalRoles.String, ", ")
		} else {
			userObj["additional_roles"] = []string{}
		}

		users = append(users, userObj)
	}

	fmt.Printf("GetAvailableChatUsers: Found %d available %s for user %d\n", len(users), availableRole, userIDInt)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"role":    availableRole,
		"users":   users,
	})
}

// sendPushNotifications sends push notifications to all participants in a conversation
// except the sender of the message
func sendPushNotifications(db *sql.DB, conversationID int, senderID int, senderName string, content string) {
	// Find all participants in the conversation except the sender
	query := `
		SELECT u.id, u.device_id
		FROM users u
		JOIN conversation_participants cp ON u.id = cp.user_id
		WHERE cp.conversation_id = $1 AND u.id != $2 AND u.device_id IS NOT NULL AND u.device_id != ''
	`

	rows, err := db.Query(query, conversationID, senderID)
	if err != nil {
		fmt.Printf("Error querying participants for notifications: %v\n", err)
		return
	}
	defer rows.Close()

	// Send a push notification to each participant
	for rows.Next() {
		var userID int
		var deviceID string

		if err := rows.Scan(&userID, &deviceID); err != nil {
			fmt.Printf("Error scanning participant data: %v\n", err)
			continue
		}

		// Skip if device ID is missing or invalid
		if deviceID == "" {
			fmt.Printf("Skipping notification for user %d: No device ID\n", userID)
			continue
		}

		// Truncate content if too long for notification
		messagePreview := content
		if len(messagePreview) > 100 {
			messagePreview = messagePreview[:97] + "..."
		}

		// Send the notification
		err := notifications.SendMessageNotification(deviceID, conversationID, senderName, messagePreview)
		if err != nil {
			fmt.Printf("Error sending notification to user %d: %v\n", userID, err)
		} else {
			fmt.Printf("Successfully sent notification to user %d\n", userID)
		}
	}

	if err := rows.Err(); err != nil {
		fmt.Printf("Error iterating participants: %v\n", err)
	}
}

// SetupMessagingRoutes sets up the messaging routes
func SetupMessagingRoutes(router gin.IRouter, db *sql.DB) {
	messagingGroup := router.Group("/messaging")
	{
		messagingGroup.GET("/conversations/:user_id", func(c *gin.Context) {
			GetUserConversations(c, db)
		})
		messagingGroup.GET("/conversation/:conversation_id/messages", func(c *gin.Context) {
			GetConversationMessages(c, db)
		})
		messagingGroup.POST("/messages", func(c *gin.Context) {
			SendMessage(c, db)
		})
		messagingGroup.POST("/conversations", func(c *gin.Context) {
			CreateConversation(c, db)
		})
		messagingGroup.GET("/chat-users/:user_id", func(c *gin.Context) {
			GetAvailableChatUsers(c, db)
		})
	}
}
