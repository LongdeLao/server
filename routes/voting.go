package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"server/models"
	"strconv"

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

	// New endpoints
	router.GET("/voting/check-vote", checkUserVote(db))
	router.GET("/voting/results/:event_id", getVoteResults(db))

	// Statistics endpoints
	router.GET("/voting/statistics/:event_id", getVotingStatistics(db))
}

// getVotingEvents returns all voting events with their sub votes and options
func getVotingEvents(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get query parameters
		status := c.Query("status")
		userID := c.Query("user_id")

		// Base query to get voting events
		query := `
            SELECT ve.id, ve.title, ve.description, ve.deadline, ve.status, 
                   ve.organizer_id, u.name as organizer_name, 
                   COALESCE(ar.role_name, 'User') as organizer_role, ve.created_at
            FROM voting_events ve
            LEFT JOIN users u ON ve.organizer_id = u.id
            LEFT JOIN additional_roles ar ON u.id = ar.user_id
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

		// Execute the query
		rows, err := db.Query(query, args...)
		if err != nil {
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning event: " + err.Error()})
				return
			}

			// Get vote count for this event
			var voteCount int
			err := db.QueryRow(`
				SELECT COUNT(DISTINCT uv.user_id) 
				FROM user_votes uv
				JOIN sub_votes sv ON uv.sub_vote_id = sv.id
				WHERE sv.event_id = $1
			`, event.ID).Scan(&voteCount)
			if err != nil && err != sql.ErrNoRows {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting vote count: " + err.Error()})
				return
			}
			event.VoteCount = voteCount

			// Get total eligible users (placeholder - actual logic depends on your requirements)
			// For example, this counts all users
			var totalUsers int
			err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
			if err != nil && err != sql.ErrNoRows {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total users: " + err.Error()})
				return
			}
			event.TotalVotes = totalUsers

			// Get sub-votes for this event
			event.SubVotes = getSubVotesForEvent(db, event.ID, userID)

			events = append(events, event)
		}

		c.JSON(http.StatusOK, events)
	}
}

// getVotingEventByID returns a single voting event with its sub votes and options
func getVotingEventByID(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("id")
		userID := c.Query("user_id")

		// Get event details
		var event models.VotingEvent
		err := db.QueryRow(`
			SELECT ve.id, ve.title, ve.description, ve.deadline, ve.status, 
				   ve.organizer_id, u.name as organizer_name, 
				   COALESCE(ar.role_name, 'User') as organizer_role, ve.created_at
			FROM voting_events ve
			LEFT JOIN users u ON ve.organizer_id = u.id
			LEFT JOIN additional_roles ar ON u.id = ar.user_id
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
				c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			}
			return
		}

		// Get vote count
		var voteCount int
		err = db.QueryRow(`
			SELECT COUNT(DISTINCT uv.user_id) 
			FROM user_votes uv
			JOIN sub_votes sv ON uv.sub_vote_id = sv.id
			WHERE sv.event_id = $1
		`, event.ID).Scan(&voteCount)
		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting vote count: " + err.Error()})
			return
		}
		event.VoteCount = voteCount

		// Get total eligible users
		var totalUsers int
		err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total users: " + err.Error()})
			return
		}
		event.TotalVotes = totalUsers

		// Get sub-votes for this event
		event.SubVotes = getSubVotesForEvent(db, event.ID, userID)

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

		// Extract and validate user ID
		userID, err := ParseUserID(c.GetHeader("User-ID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		// Create voting event
		eventID, err := CreateVotingEvent(tx, request, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create voting event: " + err.Error()})
			return
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":  "Voting event created successfully",
			"event_id": eventID,
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

		// Extract and validate user ID
		userID, err := ParseUserID(c.GetHeader("User-ID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate the voting event (check if active and deadline not passed)
		_, _, _, err = ValidateVotingEvent(db, voteRequest.SubVoteID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Sub-vote not found"})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			}
			return
		}

		// Validate that the option exists and belongs to the sub-vote
		_, err = ValidateVoteOption(db, voteRequest.OptionID, voteRequest.SubVoteID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if user has already voted in this sub-vote
		hasVoted, existingVoteID, err := ValidateUserVote(db, userID, voteRequest.SubVoteID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
			return
		}
		defer tx.Rollback()

		// Submit or update vote
		if hasVoted {
			// User has already voted, update their vote
			err = UpdateExistingVote(tx, existingVoteID, voteRequest.OptionID, voteRequest.CustomInput)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vote: " + err.Error()})
				return
			}
		} else {
			// User has not voted yet, submit new vote
			err = SubmitNewVote(tx, userID, voteRequest.SubVoteID, voteRequest.OptionID, voteRequest.CustomInput)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit vote: " + err.Error()})
				return
			}
		}

		// Update vote counts in options table
		err = UpdateVoteCounts(tx, voteRequest.SubVoteID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vote counts: " + err.Error()})
			return
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Vote submitted successfully",
			"updated": hasVoted,
		})
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
	rows, err := db.Query(`
		SELECT id, event_id, title, description, created_at
		FROM sub_votes
		WHERE event_id = $1
	`, eventID)

	if err != nil {
		log.Printf("Error getting sub-votes: %v", err)
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

		// Get options for this sub-vote
		optRows, err := db.Query(`
			SELECT id, sub_vote_id, text, has_custom_input, vote_count, created_at
			FROM vote_options
			WHERE sub_vote_id = $1
		`, subVote.ID)

		if err != nil {
			log.Printf("Error getting options: %v", err)
			continue
		}
		defer optRows.Close()

		for optRows.Next() {
			var option models.VoteOption
			if err := optRows.Scan(&option.ID, &option.SubVoteID, &option.Text, &option.HasCustomInput, &option.VoteCount, &option.CreatedAt); err != nil {
				log.Printf("Error scanning option: %v", err)
				continue
			}
			subVote.Options = append(subVote.Options, option)
		}

		// If user ID is provided, get the user's vote for this sub-vote
		if userID != "" {
			var userVote models.UserVote
			err = db.QueryRow(`
				SELECT id, user_id, sub_vote_id, option_id, custom_input, created_at
				FROM user_votes
				WHERE user_id = $1 AND sub_vote_id = $2
			`, userID, subVote.ID).Scan(&userVote.ID, &userVote.UserID, &userVote.SubVoteID, &userVote.OptionID, &userVote.CustomInput, &userVote.CreatedAt)

			if err == nil {
				// Maybe add this information to the subVote if needed
				// For now, we don't do anything with it
			}
		}

		subVotes = append(subVotes, subVote)
	}

	return subVotes
}

// checkUserVote checks if a user has already voted in a specific sub-vote
func checkUserVote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Query("user_id")
		subVoteIDStr := c.Query("sub_vote_id")

		if userIDStr == "" || subVoteIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID and sub vote ID are required"})
			return
		}

		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		subVoteID, err := strconv.Atoi(subVoteIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sub vote ID"})
			return
		}

		// Check if the user has already voted
		hasVoted, voteID, err := ValidateUserVote(db, userID, subVoteID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}

		// If the user has voted, get the details
		var voteDetails map[string]interface{}
		if hasVoted {
			var optionID int
			var optionText string
			var customInput sql.NullString

			err := db.QueryRow(`
				SELECT uv.option_id, vo.text, uv.custom_input
				FROM user_votes uv
				JOIN vote_options vo ON uv.option_id = vo.id
				WHERE uv.id = $1
			`, voteID).Scan(&optionID, &optionText, &customInput)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting vote details: " + err.Error()})
				return
			}

			voteDetails = map[string]interface{}{
				"vote_id":      voteID,
				"option_id":    optionID,
				"option_text":  optionText,
				"custom_input": customInput.String,
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"has_voted":    hasVoted,
			"vote_details": voteDetails,
		})
	}
}

// getVoteResults gets detailed results for a voting event
func getVoteResults(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID := c.Param("event_id")

		// First, check if the event exists
		var eventExists bool
		err := db.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM voting_events WHERE id = $1)
		`, eventID).Scan(&eventExists)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
			return
		}

		if !eventExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Voting event not found"})
			return
		}

		// Get event details
		var event models.VotingEvent
		err = db.QueryRow(`
			SELECT ve.id, ve.title, ve.description, ve.deadline, ve.status, 
				   ve.organizer_id, u.name as organizer_name,
				   COALESCE(ar.role_name, 'User') as organizer_role, ve.created_at
			FROM voting_events ve
			LEFT JOIN users u ON ve.organizer_id = u.id
			LEFT JOIN additional_roles ar ON u.id = ar.user_id
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting event details: " + err.Error()})
			return
		}

		// Get sub-votes and results
		rows, err := db.Query(`
			SELECT sv.id, sv.title, sv.description
			FROM sub_votes sv
			WHERE sv.event_id = $1
			ORDER BY sv.id
		`, eventID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting sub-votes: " + err.Error()})
			return
		}
		defer rows.Close()

		type OptionResult struct {
			OptionID       int      `json:"option_id"`
			Text           string   `json:"text"`
			HasCustomInput bool     `json:"has_custom_input"`
			VoteCount      int      `json:"vote_count"`
			Percentage     float64  `json:"percentage"`
			CustomInputs   []string `json:"custom_inputs,omitempty"`
		}

		type SubVoteResult struct {
			SubVoteID   int            `json:"sub_vote_id"`
			Title       string         `json:"title"`
			Description string         `json:"description"`
			TotalVotes  int            `json:"total_votes"`
			Options     []OptionResult `json:"options"`
		}

		var results []SubVoteResult

		for rows.Next() {
			var subVote SubVoteResult
			if err := rows.Scan(&subVote.SubVoteID, &subVote.Title, &subVote.Description); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning sub-vote: " + err.Error()})
				return
			}

			// Get total votes for this sub-vote
			err := db.QueryRow(`
				SELECT COUNT(*) FROM user_votes WHERE sub_vote_id = $1
			`, subVote.SubVoteID).Scan(&subVote.TotalVotes)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting vote count: " + err.Error()})
				return
			}

			// Get options and results
			optionRows, err := db.Query(`
				SELECT vo.id, vo.text, vo.has_custom_input, vo.vote_count
				FROM vote_options vo
				WHERE vo.sub_vote_id = $1
				ORDER BY vo.id
			`, subVote.SubVoteID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting options: " + err.Error()})
				return
			}
			defer optionRows.Close()

			for optionRows.Next() {
				var option OptionResult
				if err := optionRows.Scan(&option.OptionID, &option.Text, &option.HasCustomInput, &option.VoteCount); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning option: " + err.Error()})
					return
				}

				// Calculate percentage
				if subVote.TotalVotes > 0 {
					option.Percentage = float64(option.VoteCount) / float64(subVote.TotalVotes) * 100
				}

				// Get custom inputs if applicable
				if option.HasCustomInput && option.VoteCount > 0 {
					customRows, err := db.Query(`
						SELECT custom_input
						FROM user_votes
						WHERE sub_vote_id = $1 AND option_id = $2 AND custom_input IS NOT NULL AND custom_input != ''
					`, subVote.SubVoteID, option.OptionID)

					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting custom inputs: " + err.Error()})
						return
					}
					defer customRows.Close()

					for customRows.Next() {
						var customInput string
						if err := customRows.Scan(&customInput); err != nil {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning custom input: " + err.Error()})
							return
						}
						option.CustomInputs = append(option.CustomInputs, customInput)
					}
				}

				subVote.Options = append(subVote.Options, option)
			}

			results = append(results, subVote)
		}

		// Get vote counts
		var totalVotes int
		err = db.QueryRow(`
			SELECT COUNT(DISTINCT user_id)
			FROM user_votes uv
			JOIN sub_votes sv ON uv.sub_vote_id = sv.id
			WHERE sv.event_id = $1
		`, eventID).Scan(&totalVotes)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total votes: " + err.Error()})
			return
		}

		// Get total eligible users
		var totalUsers int
		err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total users: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"event": gin.H{
				"id":             event.ID,
				"title":          event.Title,
				"description":    event.Description,
				"deadline":       event.Deadline,
				"status":         event.Status,
				"organizer_id":   event.OrganizerID,
				"organizer_name": event.OrganizerName,
				"organizer_role": event.OrganizerRole,
				"created_at":     event.CreatedAt,
				"vote_count":     totalVotes,
				"total_votes":    totalUsers,
			},
			"results": results,
		})
	}
}
