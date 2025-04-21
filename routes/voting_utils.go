package routes

import (
	"database/sql"
	"errors"
	"server/models"
	"strconv"
	"time"
)

// Utility functions for the voting system

// ValidateUserVote checks if a user has already voted in a specific sub-vote
func ValidateUserVote(db *sql.DB, userID int, subVoteID int) (bool, int, error) {
	var existingVoteID int
	err := db.QueryRow(`
		SELECT id FROM user_votes WHERE user_id = $1 AND sub_vote_id = $2
	`, userID, subVoteID).Scan(&existingVoteID)

	if err == sql.ErrNoRows {
		// User has not voted yet
		return false, 0, nil
	} else if err != nil {
		// Database error
		return false, 0, err
	}

	// User has already voted
	return true, existingVoteID, nil
}

// ValidateVotingEvent checks if a voting event is active and deadline has not passed
func ValidateVotingEvent(db *sql.DB, subVoteID int) (int, time.Time, string, error) {
	var eventID int
	var deadline time.Time
	var status string

	err := db.QueryRow(`
		SELECT ve.id, ve.deadline, ve.status
		FROM sub_votes sv
		JOIN voting_events ve ON sv.event_id = ve.id
		WHERE sv.id = $1
	`, subVoteID).Scan(&eventID, &deadline, &status)

	if err != nil {
		return 0, time.Time{}, "", err
	}

	// Check if the voting event is active
	if status != "active" {
		return eventID, deadline, status, errors.New("voting event is not active")
	}

	// Check if deadline has passed
	if time.Now().After(deadline) {
		return eventID, deadline, status, errors.New("voting deadline has passed")
	}

	return eventID, deadline, status, nil
}

// ValidateVoteOption checks if a vote option exists and belongs to the specified sub-vote
func ValidateVoteOption(db *sql.DB, optionID int, subVoteID int) (bool, error) {
	var optionExists bool
	err := db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM vote_options WHERE id = $1 AND sub_vote_id = $2)
	`, optionID, subVoteID).Scan(&optionExists)

	if err != nil {
		return false, err
	}

	if !optionExists {
		return false, errors.New("option not found or does not belong to the specified sub-vote")
	}

	return true, nil
}

// SubmitNewVote adds a new vote record
func SubmitNewVote(tx *sql.Tx, userID int, subVoteID int, optionID int, customInput string) error {
	_, err := tx.Exec(`
		INSERT INTO user_votes (user_id, sub_vote_id, option_id, custom_input)
		VALUES ($1, $2, $3, $4)
	`, userID, subVoteID, optionID, customInput)

	return err
}

// UpdateExistingVote updates an existing vote record
func UpdateExistingVote(tx *sql.Tx, voteID int, optionID int, customInput string) error {
	_, err := tx.Exec(`
		UPDATE user_votes
		SET option_id = $1, custom_input = $2
		WHERE id = $3
	`, optionID, customInput, voteID)

	return err
}

// UpdateVoteCounts recalculates vote counts for all options in a sub-vote
func UpdateVoteCounts(tx *sql.Tx, subVoteID int) error {
	_, err := tx.Exec(`
		UPDATE vote_options vo
		SET vote_count = (
			SELECT COUNT(*) 
			FROM user_votes uv 
			WHERE uv.option_id = vo.id
		)
		WHERE vo.sub_vote_id = $1
	`, subVoteID)

	return err
}

// ParseUserID extracts and validates the user ID from the request header
func ParseUserID(userIDStr string) (int, error) {
	if userIDStr == "" {
		return 0, errors.New("user ID is required")
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, errors.New("invalid User ID")
	}

	return userID, nil
}

// CreateVotingEvent creates a new voting event with sub-votes and options
func CreateVotingEvent(tx *sql.Tx, request models.VotingEventRequest, organizerID int) (int, error) {
	var eventID int
	err := tx.QueryRow(`
		INSERT INTO voting_events (title, description, deadline, status, organizer_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, request.Title, request.Description, request.Deadline, request.Status, organizerID).Scan(&eventID)

	if err != nil {
		return 0, err
	}

	// Create sub-votes and options
	for _, subVoteReq := range request.SubVotes {
		var subVoteID int
		err = tx.QueryRow(`
			INSERT INTO sub_votes (event_id, title, description)
			VALUES ($1, $2, $3)
			RETURNING id
		`, eventID, subVoteReq.Title, subVoteReq.Description).Scan(&subVoteID)

		if err != nil {
			return 0, err
		}

		// Insert options for this sub-vote
		for _, optionReq := range subVoteReq.Options {
			_, err = tx.Exec(`
				INSERT INTO vote_options (sub_vote_id, text, has_custom_input)
				VALUES ($1, $2, $3)
			`, subVoteID, optionReq.Text, optionReq.HasCustomInput)

			if err != nil {
				return 0, err
			}
		}
	}

	return eventID, nil
}
