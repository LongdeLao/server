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

// SetupVotingRoutes registers all the routes related to the voting system
func SetupVotingRoutes(router *gin.RouterGroup, db *sql.DB) {
	// Voting events endpoints
	router.GET("/voting/events", getVotingEvents(db))
	router.GET("/voting/events/:id", getVotingEventByID(db))
	router.POST("/voting/events", createVotingEvent(db))
	router.PUT("/voting/events/:id", updateVotingEvent(db))
	router.DELETE("/voting/events/:id", deleteVotingEvent(db))

	// User votes endpoints
	router.POST("/voting/vote", submitVote(db))
	router.GET("/voting/user-votes/:user_id", getUserVotes(db))
	router.DELETE("/voting/user-votes/:id", deleteUserVote(db))

	// Statistics endpoints
	router.GET("/voting/statistics/:event_id", getVotingStatistics(db))
}

// getVotingEvents returns all voting events with their sub votes and options
func getVotingEvents(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get query parameters
		status := c.Query("status")
		userID := c.Query("user_id")

		// Add debug logging
		log.Printf("Getting voting events. Status filter: %s, User ID: %s", status, userID)

		// Base query to get voting events - handle NULLs explicitly with COALESCE
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

		// Add status filter if provided
		if status != "" {
			query += fmt.Sprintf(" AND ve.status = $%d", argCount)
			args = append(args, status)
			argCount++
		}

		// Order by created_at
		query += " ORDER BY ve.created_at DESC"

		// Log the final query for debugging
		log.Printf("SQL Query: %s with args: %v", query, args)

		// Execute the query
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

			// Get vote count for this event
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

			// Get total eligible users (placeholder - actual logic depends on your requirements)
			// For example, this counts all users
			var totalUsers int
			err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
			if err != nil && err != sql.ErrNoRows {
				log.Printf("Error getting total users: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total users: " + err.Error()})
				return
			}
			event.TotalVotes = totalUsers

			// Get sub-votes for this event
			subVotes := getSubVotesForEvent(db, event.ID, userID)
			if subVotes == nil {
				// Initialize with empty array instead of nil
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

// getVotingEventByID returns a single voting event with its sub votes and options
func getVotingEventByID(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("id")
		userID := c.Query("user_id")

		log.Printf("Getting voting event by ID: %s, User ID: %s", eventID, userID)

		// Get event details
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

		// Get vote count
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

		// Get total eligible users
		var totalUsers int
		err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Error getting total users: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total users: " + err.Error()})
			return
		}
		event.TotalVotes = totalUsers

		// Get sub-votes for this event
		subVotes := getSubVotesForEvent(db, event.ID, userID)
		if subVotes == nil {
			// Initialize with empty array instead of nil
			event.SubVotes = []models.SubVote{}
		} else {
			event.SubVotes = subVotes
		}

		log.Printf("Returning event with %d sub-votes", len(event.SubVotes))
		c.JSON(http.StatusOK, event)
	}
}

// createVotingEvent creates a new voting event with its sub-votes and options
func createVotingEvent(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request models.VotingEventRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
			return
		}

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		// Extract user ID from the request
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

		// Insert voting event
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

		// Insert sub-votes and options
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

			// Insert options for this sub-vote
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

		// Commit the transaction
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

// updateVotingEvent updates an existing voting event
func updateVotingEvent(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("id")

		var request models.VotingEventRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
			return
		}

		// Verify event exists
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

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		// Update event
		_, err = tx.Exec(`
            UPDATE voting_events
            SET title = $1, description = $2, deadline = $3, status = $4
            WHERE id = $5
        `, request.Title, request.Description, request.Deadline, request.Status, eventID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event: " + err.Error()})
			return
		}

		// For simplicity, we'll delete all sub-votes and recreate them
		// This is not the most efficient approach but works for the demonstration
		// A real implementation might handle partial updates more carefully
		_, err = tx.Exec(`
			DELETE FROM sub_votes WHERE event_id = $1
		`, eventID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update sub-votes: " + err.Error()})
			return
		}

		// Insert new sub-votes and options
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

			// Insert options for this sub-vote
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

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Voting event updated successfully"})
	}
}

// deleteVotingEvent deletes a voting event and all related data
func deleteVotingEvent(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("id")

		// Verify event exists
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

		// Delete the event - cascade deletion will handle sub-votes, options, and user votes
		_, err = db.Exec("DELETE FROM voting_events WHERE id = $1", eventID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete event: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Voting event deleted successfully"})
	}
}

// submitVote handles a user's vote submission
func submitVote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var voteRequest models.UserVoteRequest
		if err := c.ShouldBindJSON(&voteRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data: " + err.Error()})
			return
		}

		// Extract user ID from the request
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

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		// Check if the sub-vote exists and get the event ID
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

		// Check if the voting event is active and deadline has not passed
		if status != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This voting event is not active"})
			return
		}

		if time.Now().After(deadline) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "The deadline for this voting event has passed"})
			return
		}

		// Check if the option exists and belongs to the specified sub-vote
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

		// Check if the user has already voted for this sub-vote
		var existingVoteID int
		err = tx.QueryRow(`
			SELECT id FROM user_votes WHERE user_id = $1 AND sub_vote_id = $2
		`, userID, voteRequest.SubVoteID).Scan(&existingVoteID)

		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}

		// If user has already voted, update their vote
		if err == nil { // User has voted before
			_, err = tx.Exec(`
				UPDATE user_votes
				SET option_id = $1, custom_input = $2
				WHERE id = $3
			`, voteRequest.OptionID, voteRequest.CustomInput, existingVoteID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vote: " + err.Error()})
				return
			}
		} else { // User hasn't voted before, insert new vote
			_, err = tx.Exec(`
				INSERT INTO user_votes (user_id, sub_vote_id, option_id, custom_input)
				VALUES ($1, $2, $3, $4)
			`, userID, voteRequest.SubVoteID, voteRequest.OptionID, voteRequest.CustomInput)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit vote: " + err.Error()})
				return
			}
		}

		// Update vote count in options table
		// First, recalculate counts for all options in this sub-vote
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

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Vote submitted successfully"})
	}
}

// getUserVotes returns all votes submitted by a specific user
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

// deleteUserVote deletes a specific user vote
func deleteUserVote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		voteID := c.Param("id")
		userIDStr := c.GetHeader("User-ID")

		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		// Check if the vote exists and belongs to the user
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

		// Delete the vote
		_, err = tx.Exec("DELETE FROM user_votes WHERE id = $1", voteID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete vote: " + err.Error()})
			return
		}

		// Update vote counts
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

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Vote deleted successfully"})
	}
}

// getVotingStatistics returns statistics for a specific voting event
func getVotingStatistics(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("event_id")

		// Check if the event exists
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

		// Get sub-votes for this event
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

			// Get options for this sub-vote
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

			// Count total votes for this sub-vote
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

				// Calculate percentage
				if stat.TotalVotes > 0 {
					option.Percentage = int(float64(option.VoteCount) / float64(stat.TotalVotes) * 100)
				}

				// Get custom inputs if this option has them
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

// Helper function to get sub-votes for an event
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

		// Get options for this sub-vote
		optRows, err := db.Query(`
			SELECT id, sub_vote_id, text, has_custom_input, vote_count, created_at
			FROM vote_options
			WHERE sub_vote_id = $1
		`, subVote.ID)

		if err != nil {
			log.Printf("Error getting options for sub-vote %d: %v", subVote.ID, err)
			// Continue with empty options rather than skipping the sub-vote entirely
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

		// If user ID is provided, get the user's vote for this sub-vote
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
					// If found user vote, attach it to the subVote as user_vote field
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
