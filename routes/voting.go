package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"server/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

/**
 * SetupVotingRoutes registers all the routes related to the voting system.
 *
 * Endpoints:
 * 1. GET /voting/events
 *    - Returns all voting events with their sub votes and options
 *
 * 2. GET /voting/events/:id
 *    - Returns a single voting event with its sub votes and options
 *
 * 3. POST /voting/events
 *    - Creates a new voting event with its sub-votes and options
 *
 * 4. PUT /voting/events/:id
 *    - Updates an existing voting event
 *
 * 5. DELETE /voting/events/:id
 *    - Deletes a voting event and all related data
 *
 * 6. POST /voting/vote
 *    - Handles a user's vote submission
 *
 * 7. GET /voting/user-votes/:user_id
 *    - Returns all votes submitted by a specific user
 *
 * 8. DELETE /voting/user-votes/:id
 *    - Deletes a specific user vote
 *
 * 9. GET /voting/statistics/:event_id
 *    - Returns statistics for a specific voting event
 */
func SetupVotingRoutes(router *gin.RouterGroup, db *sql.DB) {
	router.GET("/voting/events", getVotingEvents(db))
	router.GET("/voting/events/:id", getVotingEventByID(db))
	router.POST("/voting/events", createVotingEvent(db))
	router.PUT("/voting/events/:id", updateVotingEvent(db))
	router.DELETE("/voting/events/:id", deleteVotingEvent(db))

	router.POST("/voting/vote", submitVote(db))
	router.GET("/voting/user-votes/:user_id", getUserVotes(db))
	router.DELETE("/voting/user-votes/:id", deleteUserVote(db))

	router.GET("/voting/statistics/:event_id", getVotingStatistics(db))
}

/**
 * getVotingEvents returns all voting events with their sub votes and options.
 *
 * Endpoint: GET /voting/events
 *
 * Query Parameters:
 *   - status: Optional filter by event status
 *   - user_id: Optional user ID to include user's votes in response
 *
 * Returns:
 *   - 200 OK: Array of voting events
 *     [
 *       {
 *         "id": number,
 *         "title": string,
 *         "description": string,
 *         "deadline": string,
 *         "status": string,
 *         "organizer_id": number,
 *         "organizer_name": string,
 *         "organizer_role": string,
 *         "created_at": string,
 *         "vote_count": number,
 *         "total_votes": number,
 *         "sub_votes": array
 *       }
 *     ]
 *   - 500 Internal Server Error: Database error
 */
func getVotingEvents(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.Query("status")
		userID := c.Query("user_id")

		log.Printf("Getting voting events. Status filter: %s, User ID: %s", status, userID)

		query := `
            SELECT ve.id, ve.title, 
                   COALESCE(ve.description, '') as description, 
                   ve.deadline, 
                   ve.status, 
                   ve.organizer_id, 
                   COALESCE(u.name, 'Unknown') as organizer_name, 
                   COALESCE(u.role, 'User') as organizer_role, 
                   ve.created_at
            FROM voting_events ve
            LEFT JOIN users u ON ve.organizer_id = u.id
            WHERE 1=1
        `

		args := []interface{}{}
		argCount := 1

		if status != "" {
			query += fmt.Sprintf(" AND ve.status = $%d", argCount)
			args = append(args, status)
			argCount++
		}

		query += " ORDER BY ve.created_at DESC"

		log.Printf("SQL Query: %s with args: %v", query, args)

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("Error executing SQL query: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}
		defer rows.Close()

		var events []models.VotingEvent
		for rows.Next() {
			var event models.VotingEvent
			if err := rows.Scan(
				&event.ID,
				&event.Title,
				&event.Description,
				&event.Deadline,
				&event.Status,
				&event.OrganizerID,
				&event.OrganizerName,
				&event.OrganizerRole,
				&event.CreatedAt,
			); err != nil {
				log.Printf("Error scanning event row: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning event: " + err.Error()})
				return
			}

			log.Printf("Successfully scanned event ID %d: %s", event.ID, event.Title)

			var voteCount int
			err := db.QueryRow(`
				SELECT COUNT(DISTINCT uv.user_id) 
				FROM user_votes uv
				JOIN sub_votes sv ON uv.sub_vote_id = sv.id
				WHERE sv.event_id = $1
			`, event.ID).Scan(&voteCount)
			if err != nil && err != sql.ErrNoRows {
				log.Printf("Error getting vote count for event %d: %v", event.ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting vote count: " + err.Error()})
				return
			}
			event.VoteCount = voteCount

			var totalUsers int
			err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
			if err != nil && err != sql.ErrNoRows {
				log.Printf("Error getting total users: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total users: " + err.Error()})
				return
			}
			event.TotalVotes = totalUsers

			subVotes := getSubVotesForEvent(db, event.ID, userID)
			if subVotes == nil {
				event.SubVotes = []models.SubVote{}
			} else {
				event.SubVotes = subVotes
			}

			events = append(events, event)
		}

		log.Printf("Returning %d voting events", len(events))
		c.JSON(http.StatusOK, events)
	}
}

/**
 * getVotingEventByID returns a single voting event with its sub votes and options.
 *
 * Endpoint: GET /voting/events/:id
 *
 * Path Parameters:
 *   - id: Event ID
 *
 * Query Parameters:
 *   - user_id: Optional user ID to include user's votes in response
 *
 * Returns:
 *   - 200 OK: Single voting event object
 *   - 404 Not Found: Event not found
 *   - 500 Internal Server Error: Database error
 */
func getVotingEventByID(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("id")
		userID := c.Query("user_id")

		log.Printf("Getting voting event by ID: %s, User ID: %s", eventID, userID)

		var event models.VotingEvent
		err := db.QueryRow(`
			SELECT ve.id, ve.title, 
				   COALESCE(ve.description, '') as description, 
				   ve.deadline, 
				   ve.status, 
				   ve.organizer_id, 
				   COALESCE(u.name, 'Unknown') as organizer_name, 
				   COALESCE(u.role, 'User') as organizer_role, 
				   ve.created_at
			FROM voting_events ve
			LEFT JOIN users u ON ve.organizer_id = u.id
			WHERE ve.id = $1
		`, eventID).Scan(
			&event.ID,
			&event.Title,
			&event.Description,
			&event.Deadline,
			&event.Status,
			&event.OrganizerID,
			&event.OrganizerName,
			&event.OrganizerRole,
			&event.CreatedAt,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("Event not found: %s", eventID)
				c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			} else {
				log.Printf("Database error getting event %s: %v", eventID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			}
			return
		}

		log.Printf("Successfully retrieved event ID %d: %s", event.ID, event.Title)

		var voteCount int
		err = db.QueryRow(`
			SELECT COUNT(DISTINCT uv.user_id) 
			FROM user_votes uv
			JOIN sub_votes sv ON uv.sub_vote_id = sv.id
			WHERE sv.event_id = $1
		`, event.ID).Scan(&voteCount)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Error getting vote count for event %d: %v", event.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting vote count: " + err.Error()})
			return
		}
		event.VoteCount = voteCount

		var totalUsers int
		err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Error getting total users: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total users: " + err.Error()})
			return
		}
		event.TotalVotes = totalUsers

		subVotes := getSubVotesForEvent(db, event.ID, userID)
		if subVotes == nil {
			event.SubVotes = []models.SubVote{}
		} else {
			event.SubVotes = subVotes
		}

		log.Printf("Returning event with %d sub-votes", len(event.SubVotes))
		c.JSON(http.StatusOK, event)
	}
}

/**
 * createVotingEvent creates a new voting event with its sub-votes and options.
 *
 * Endpoint: POST /voting/events
 *
 * Headers:
 *   - User-ID: Required user ID for organizer
 *
 * Request Body:
 * {
 *   "title": string,           // Required: Event title
 *   "description": string,     // Optional: Event description
 *   "deadline": string,        // Required: Event deadline (ISO format)
 *   "status": string,          // Required: Event status
 *   "sub_votes": array         // Required: Array of sub-votes
 * }
 *
 * Returns:
 *   - 201 Created: Event created successfully
 *     {
 *       "message": string,
 *       "id": number
 *     }
 *   - 400 Bad Request: Invalid request data or missing User-ID
 *   - 500 Internal Server Error: Database error
 */
func createVotingEvent(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request models.VotingEventRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		userIDStr := c.GetHeader("User-ID")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}
		organizerID, err := strconv.Atoi(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid User ID"})
			return
		}

		var eventID int
		err = tx.QueryRow(`
            INSERT INTO voting_events (title, description, deadline, status, organizer_id)
            VALUES ($1, $2, $3, $4, $5)
            RETURNING id
        `, request.Title, request.Description, request.Deadline, request.Status, organizerID).Scan(&eventID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create voting event: " + err.Error()})
			return
		}

		for _, subVoteReq := range request.SubVotes {
			var subVoteID int
			err = tx.QueryRow(`
                INSERT INTO sub_votes (event_id, title, description)
                VALUES ($1, $2, $3)
                RETURNING id
            `, eventID, subVoteReq.Title, subVoteReq.Description).Scan(&subVoteID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create sub-vote: " + err.Error()})
				return
			}

			for _, optionReq := range subVoteReq.Options {
				_, err = tx.Exec(`
                    INSERT INTO vote_options (sub_vote_id, text, has_custom_input)
                    VALUES ($1, $2, $3)
                `, subVoteID, optionReq.Text, optionReq.HasCustomInput)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vote option: " + err.Error()})
					return
				}
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "Voting event created successfully",
			"id":      eventID,
		})
	}
}

/**
 * updateVotingEvent updates an existing voting event.
 *
 * Endpoint: PUT /voting/events/:id
 *
 * Path Parameters:
 *   - id: Event ID to update
 *
 * Request Body:
 * {
 *   "title": string,           // Required: Event title
 *   "description": string,     // Optional: Event description
 *   "deadline": string,        // Required: Event deadline (ISO format)
 *   "status": string,          // Required: Event status
 *   "sub_votes": array         // Required: Array of sub-votes
 * }
 *
 * Returns:
 *   - 200 OK: Event updated successfully
 *   - 400 Bad Request: Invalid request data
 *   - 404 Not Found: Event not found
 *   - 500 Internal Server Error: Database error
 */
func updateVotingEvent(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("id")

		var request models.VotingEventRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
			return
		}

		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM voting_events WHERE id = $1)", eventID).Scan(&exists)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		_, err = tx.Exec(`
            UPDATE voting_events
            SET title = $1, description = $2, deadline = $3, status = $4
            WHERE id = $5
        `, request.Title, request.Description, request.Deadline, request.Status, eventID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event: " + err.Error()})
			return
		}

		_, err = tx.Exec(`
			DELETE FROM sub_votes WHERE event_id = $1
		`, eventID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update sub-votes: " + err.Error()})
			return
		}

		for _, subVoteReq := range request.SubVotes {
			var subVoteID int
			err = tx.QueryRow(`
                INSERT INTO sub_votes (event_id, title, description)
                VALUES ($1, $2, $3)
                RETURNING id
            `, eventID, subVoteReq.Title, subVoteReq.Description).Scan(&subVoteID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create sub-vote: " + err.Error()})
				return
			}

			for _, optionReq := range subVoteReq.Options {
				_, err = tx.Exec(`
                    INSERT INTO vote_options (sub_vote_id, text, has_custom_input)
                    VALUES ($1, $2, $3)
                `, subVoteID, optionReq.Text, optionReq.HasCustomInput)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vote option: " + err.Error()})
					return
				}
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Voting event updated successfully"})
	}
}

/**
 * deleteVotingEvent deletes a voting event and all related data.
 *
 * Endpoint: DELETE /voting/events/:id
 *
 * Path Parameters:
 *   - id: Event ID to delete
 *
 * Returns:
 *   - 200 OK: Event deleted successfully
 *   - 404 Not Found: Event not found
 *   - 500 Internal Server Error: Database error
 */
func deleteVotingEvent(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("id")

		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM voting_events WHERE id = $1)", eventID).Scan(&exists)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}

		_, err = db.Exec("DELETE FROM voting_events WHERE id = $1", eventID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete event: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Voting event deleted successfully"})
	}
}

/**
 * submitVote handles a user's vote submission.
 *
 * Endpoint: POST /voting/vote
 *
 * Headers:
 *   - User-ID: Required user ID
 *
 * Request Body:
 * {
 *   "sub_vote_id": number,     // Required: Sub-vote ID
 *   "option_id": number,       // Required: Option ID
 *   "custom_input": string     // Optional: Custom input text
 * }
 *
 * Returns:
 *   - 200 OK: Vote submitted successfully
 *   - 400 Bad Request: Invalid request data, event not active, or deadline passed
 *   - 404 Not Found: Sub-vote or option not found
 *   - 500 Internal Server Error: Database error
 */
func submitVote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var voteRequest models.UserVoteRequest
		if err := c.ShouldBindJSON(&voteRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
			return
		}

		userIDStr := c.GetHeader("User-ID")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}
		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid User ID"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		var eventID int
		var deadline time.Time
		var status string
		err = tx.QueryRow(`
			SELECT ve.id, ve.deadline, ve.status
			FROM sub_votes sv
			JOIN voting_events ve ON sv.event_id = ve.id
			WHERE sv.id = $1
		`, voteRequest.SubVoteID).Scan(&eventID, &deadline, &status)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sub-vote not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			}
			return
		}

		if status != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This voting event is not active"})
			return
		}

		if time.Now().After(deadline) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "The deadline for this voting event has passed"})
			return
		}

		var optionExists bool
		err = tx.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM vote_options WHERE id = $1 AND sub_vote_id = $2)
		`, voteRequest.OptionID, voteRequest.SubVoteID).Scan(&optionExists)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}

		if !optionExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Option not found or does not belong to the specified sub-vote"})
			return
		}

		var existingVoteID int
		err = tx.QueryRow(`
			SELECT id FROM user_votes WHERE user_id = $1 AND sub_vote_id = $2
		`, userID, voteRequest.SubVoteID).Scan(&existingVoteID)

		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}

		if err == nil {
			_, err = tx.Exec(`
				UPDATE user_votes
				SET option_id = $1, custom_input = $2
				WHERE id = $3
			`, voteRequest.OptionID, voteRequest.CustomInput, existingVoteID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vote: " + err.Error()})
				return
			}
		} else {
			_, err = tx.Exec(`
				INSERT INTO user_votes (user_id, sub_vote_id, option_id, custom_input)
				VALUES ($1, $2, $3, $4)
			`, userID, voteRequest.SubVoteID, voteRequest.OptionID, voteRequest.CustomInput)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit vote: " + err.Error()})
				return
			}
		}

		_, err = tx.Exec(`
			UPDATE vote_options vo
			SET vote_count = (
				SELECT COUNT(*) 
				FROM user_votes uv 
				WHERE uv.option_id = vo.id
			)
			WHERE vo.sub_vote_id = $1
		`, voteRequest.SubVoteID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vote counts: " + err.Error()})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Vote submitted successfully"})
	}
}

/**
 * getUserVotes returns all votes submitted by a specific user.
 *
 * Endpoint: GET /voting/user-votes/:user_id
 *
 * Path Parameters:
 *   - user_id: User ID to get votes for
 *
 * Returns:
 *   - 200 OK: Array of user votes with details
 *     [
 *       {
 *         "id": number,
 *         "user_id": number,
 *         "sub_vote_id": number,
 *         "option_id": number,
 *         "custom_input": string,
 *         "created_at": string,
 *         "sub_vote_title": string,
 *         "option_text": string
 *       }
 *     ]
 *   - 500 Internal Server Error: Database error
 */
func getUserVotes(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")

		rows, err := db.Query(`
			SELECT uv.id, uv.user_id, uv.sub_vote_id, uv.option_id, 
				   uv.custom_input, uv.created_at,
				   sv.title as sub_vote_title, 
				   vo.text as option_text
			FROM user_votes uv
			JOIN sub_votes sv ON uv.sub_vote_id = sv.id
			JOIN vote_options vo ON uv.option_id = vo.id
			WHERE uv.user_id = $1
			ORDER BY uv.created_at DESC
		`, userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}
		defer rows.Close()

		type UserVoteWithDetails struct {
			models.UserVote
			SubVoteTitle string `json:"sub_vote_title"`
			OptionText   string `json:"option_text"`
		}

		var votes []UserVoteWithDetails
		for rows.Next() {
			var vote UserVoteWithDetails
			err := rows.Scan(
				&vote.ID,
				&vote.UserID,
				&vote.SubVoteID,
				&vote.OptionID,
				&vote.CustomInput,
				&vote.CreatedAt,
				&vote.SubVoteTitle,
				&vote.OptionText,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning vote: " + err.Error()})
				return
			}
			votes = append(votes, vote)
		}

		c.JSON(http.StatusOK, votes)
	}
}

/**
 * deleteUserVote deletes a specific user vote.
 *
 * Endpoint: DELETE /voting/user-votes/:id
 *
 * Path Parameters:
 *   - id: Vote ID to delete
 *
 * Headers:
 *   - User-ID: Required user ID for authorization
 *
 * Returns:
 *   - 200 OK: Vote deleted successfully
 *   - 400 Bad Request: Missing User-ID
 *   - 404 Not Found: Vote not found or does not belong to user
 *   - 500 Internal Server Error: Database error
 */
func deleteUserVote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		voteID := c.Param("id")
		userIDStr := c.GetHeader("User-ID")

		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		var subVoteID int
		err = tx.QueryRow(`
			SELECT sub_vote_id FROM user_votes 
			WHERE id = $1 AND user_id = $2
		`, voteID, userIDStr).Scan(&subVoteID)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Vote not found or does not belong to the user"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			}
			return
		}

		_, err = tx.Exec("DELETE FROM user_votes WHERE id = $1", voteID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete vote: " + err.Error()})
			return
		}

		_, err = tx.Exec(`
			UPDATE vote_options vo
			SET vote_count = (
				SELECT COUNT(*) 
				FROM user_votes uv 
				WHERE uv.option_id = vo.id
			)
			WHERE vo.sub_vote_id = $1
		`, subVoteID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vote counts: " + err.Error()})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Vote deleted successfully"})
	}
}

/**
 * getVotingStatistics returns statistics for a specific voting event.
 *
 * Endpoint: GET /voting/statistics/:event_id
 *
 * Path Parameters:
 *   - event_id: Event ID to get statistics for
 *
 * Returns:
 *   - 200 OK: Array of sub-vote statistics
 *     [
 *       {
 *         "sub_vote_id": number,
 *         "title": string,
 *         "description": string,
 *         "total_votes": number,
 *         "option_stats": array
 *       }
 *     ]
 *   - 404 Not Found: Event not found
 *   - 500 Internal Server Error: Database error
 */
func getVotingStatistics(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("event_id")

		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM voting_events WHERE id = $1)", eventID).Scan(&exists)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}

		rows, err := db.Query(`
			SELECT id, title, description 
			FROM sub_votes 
			WHERE event_id = $1
		`, eventID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}
		defer rows.Close()

		type SubVoteStats struct {
			SubVoteID   int    `json:"sub_vote_id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			TotalVotes  int    `json:"total_votes"`
			OptionStats []struct {
				OptionID       int      `json:"option_id"`
				Text           string   `json:"text"`
				VoteCount      int      `json:"vote_count"`
				Percentage     int      `json:"percentage"`
				HasCustomInput bool     `json:"has_custom_input"`
				CustomInputs   []string `json:"custom_inputs,omitempty"`
			} `json:"option_stats"`
		}

		var stats []SubVoteStats
		for rows.Next() {
			var stat SubVoteStats
			err := rows.Scan(&stat.SubVoteID, &stat.Title, &stat.Description)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning sub-vote: " + err.Error()})
				return
			}

			optRows, err := db.Query(`
				SELECT vo.id, vo.text, vo.vote_count, vo.has_custom_input
				FROM vote_options vo
				WHERE vo.sub_vote_id = $1
			`, stat.SubVoteID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
				return
			}
			defer optRows.Close()

			err = db.QueryRow(`
				SELECT COUNT(*) FROM user_votes WHERE sub_vote_id = $1
			`, stat.SubVoteID).Scan(&stat.TotalVotes)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
				return
			}

			for optRows.Next() {
				var option struct {
					OptionID       int      `json:"option_id"`
					Text           string   `json:"text"`
					VoteCount      int      `json:"vote_count"`
					Percentage     int      `json:"percentage"`
					HasCustomInput bool     `json:"has_custom_input"`
					CustomInputs   []string `json:"custom_inputs,omitempty"`
				}

				err := optRows.Scan(&option.OptionID, &option.Text, &option.VoteCount, &option.HasCustomInput)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning option: " + err.Error()})
					return
				}

				if stat.TotalVotes > 0 {
					option.Percentage = int(float64(option.VoteCount) / float64(stat.TotalVotes) * 100)
				}

				if option.HasCustomInput && option.VoteCount > 0 {
					customRows, err := db.Query(`
						SELECT custom_input FROM user_votes 
						WHERE sub_vote_id = $1 AND option_id = $2 AND custom_input IS NOT NULL AND custom_input != ''
					`, stat.SubVoteID, option.OptionID)

					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
						return
					}
					defer customRows.Close()

					for customRows.Next() {
						var customInput string
						err := customRows.Scan(&customInput)
						if err != nil {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning custom input: " + err.Error()})
							return
						}
						option.CustomInputs = append(option.CustomInputs, customInput)
					}
				}

				stat.OptionStats = append(stat.OptionStats, option)
			}

			stats = append(stats, stat)
		}

		c.JSON(http.StatusOK, stats)
	}
}

/**
 * getSubVotesForEvent retrieves all sub-votes for a specific event.
 *
 * Parameters:
 *   - db: Database connection
 *   - eventID: Event ID to get sub-votes for
 *   - userID: Optional user ID to include user's votes
 *
 * Returns:
 *   - []models.SubVote: Array of sub-votes with options and user votes
 */
func getSubVotesForEvent(db *sql.DB, eventID int, userID string) []models.SubVote {
	log.Printf("Getting sub-votes for event ID: %d, User ID: %s", eventID, userID)

	rows, err := db.Query(`
		SELECT id, event_id, title, COALESCE(description, '') as description, created_at
		FROM sub_votes
		WHERE event_id = $1
	`, eventID)

	if err != nil {
		log.Printf("Error getting sub-votes for event %d: %v", eventID, err)
		return nil
	}
	defer rows.Close()

	var subVotes []models.SubVote
	for rows.Next() {
		var subVote models.SubVote
		if err := rows.Scan(&subVote.ID, &subVote.EventID, &subVote.Title, &subVote.Description, &subVote.CreatedAt); err != nil {
			log.Printf("Error scanning sub-vote: %v", err)
			continue
		}

		log.Printf("Retrieved sub-vote ID %d: %s", subVote.ID, subVote.Title)

		optRows, err := db.Query(`
			SELECT id, sub_vote_id, text, has_custom_input, vote_count, created_at
			FROM vote_options
			WHERE sub_vote_id = $1
		`, subVote.ID)

		if err != nil {
			log.Printf("Error getting options for sub-vote %d: %v", subVote.ID, err)
			subVote.Options = []models.VoteOption{}
		} else {
			defer optRows.Close()

			var options []models.VoteOption
			for optRows.Next() {
				var option models.VoteOption
				if err := optRows.Scan(&option.ID, &option.SubVoteID, &option.Text, &option.HasCustomInput, &option.VoteCount, &option.CreatedAt); err != nil {
					log.Printf("Error scanning option: %v", err)
					continue
				}
				log.Printf("Retrieved option ID %d: %s for sub-vote %d", option.ID, option.Text, subVote.ID)
				options = append(options, option)
			}

			if options == nil {
				subVote.Options = []models.VoteOption{}
			} else {
				subVote.Options = options
			}
		}

		if userID != "" {
			userIDInt, err := strconv.Atoi(userID)
			if err != nil {
				log.Printf("Invalid user ID format: %s - %v", userID, err)
			} else {
				var userVote models.UserVote
				err = db.QueryRow(`
					SELECT id, user_id, sub_vote_id, option_id, COALESCE(custom_input, '') as custom_input, created_at
					FROM user_votes
					WHERE user_id = $1 AND sub_vote_id = $2
				`, userIDInt, subVote.ID).Scan(&userVote.ID, &userVote.UserID, &userVote.SubVoteID, &userVote.OptionID, &userVote.CustomInput, &userVote.CreatedAt)

				if err == nil {
					log.Printf("Found user vote for sub-vote %d: option %d", subVote.ID, userVote.OptionID)
					subVote.UserVote = &userVote
				} else if err != sql.ErrNoRows {
					log.Printf("Error retrieving user vote: %v", err)
				}
			}
		}

		subVotes = append(subVotes, subVote)
	}

	log.Printf("Returning %d sub-votes for event %d", len(subVotes), eventID)

	if subVotes == nil {
		return []models.SubVote{}
	}
	return subVotes
}
