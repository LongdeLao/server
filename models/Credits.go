package models

import (
	"database/sql"
	"fmt"
	"time"
)

// UserCredits represents a user's credit information
type UserCredits struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Credits   int       `json:"credits"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetUserCredits retrieves the credit information for a specific user
func GetUserCredits(db *sql.DB, userID int) (*UserCredits, error) {
	var credits UserCredits
	query := `
		SELECT id, user_id, credits, created_at, updated_at
		FROM user_credits
		WHERE user_id = $1
	`
	err := db.QueryRow(query, userID).Scan(
		&credits.ID,
		&credits.UserID,
		&credits.Credits,
		&credits.CreatedAt,
		&credits.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// If no credits record exists, create one with default 10 credits
			return CreateUserCredits(db, userID, 10)
		}
		return nil, err
	}

	return &credits, nil
}

// CreateUserCredits creates a new credit record for a user
func CreateUserCredits(db *sql.DB, userID int, initialCredits int) (*UserCredits, error) {
	var credits UserCredits
	query := `
		INSERT INTO user_credits (user_id, credits, created_at, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, user_id, credits, created_at, updated_at
	`
	err := db.QueryRow(query, userID, initialCredits).Scan(
		&credits.ID,
		&credits.UserID,
		&credits.Credits,
		&credits.CreatedAt,
		&credits.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &credits, nil
}

// UpdateUserCredits updates the credit amount for a user
func UpdateUserCredits(db *sql.DB, userID int, newCredits int) error {
	query := `
		UPDATE user_credits 
		SET credits = $1, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
	`
	result, err := db.Exec(query, newCredits, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no credits record found for user ID %d", userID)
	}

	return nil
}

// DeductUserCredits deducts a specified amount from user's credits
func DeductUserCredits(db *sql.DB, userID int, amount int) (*UserCredits, error) {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get current credits
	var currentCredits int
	query := `SELECT credits FROM user_credits WHERE user_id = $1 FOR UPDATE`
	err = tx.QueryRow(query, userID).Scan(&currentCredits)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no credits record found for user ID %d", userID)
		}
		return nil, err
	}

	// Check if user has enough credits
	if currentCredits < amount {
		return nil, fmt.Errorf("insufficient credits: has %d, needs %d", currentCredits, amount)
	}

	// Deduct credits
	newCredits := currentCredits - amount
	updateQuery := `
		UPDATE user_credits 
		SET credits = $1, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
		RETURNING id, user_id, credits, created_at, updated_at
	`

	var credits UserCredits
	err = tx.QueryRow(updateQuery, newCredits, userID).Scan(
		&credits.ID,
		&credits.UserID,
		&credits.Credits,
		&credits.CreatedAt,
		&credits.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &credits, nil
}

// AddUserCredits adds credits to a user's account
func AddUserCredits(db *sql.DB, userID int, amount int) (*UserCredits, error) {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get current credits
	var currentCredits int
	query := `SELECT credits FROM user_credits WHERE user_id = $1 FOR UPDATE`
	err = tx.QueryRow(query, userID).Scan(&currentCredits)
	if err != nil {
		if err == sql.ErrNoRows {
			// Create new record if doesn't exist
			return CreateUserCredits(db, userID, amount)
		}
		return nil, err
	}

	// Add credits
	newCredits := currentCredits + amount
	updateQuery := `
		UPDATE user_credits 
		SET credits = $1, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
		RETURNING id, user_id, credits, created_at, updated_at
	`

	var credits UserCredits
	err = tx.QueryRow(updateQuery, newCredits, userID).Scan(
		&credits.ID,
		&credits.UserID,
		&credits.Credits,
		&credits.CreatedAt,
		&credits.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &credits, nil
}

// GetAllUserCredits retrieves all users' credit information (admin function)
func GetAllUserCredits(db *sql.DB) ([]UserCredits, error) {
	query := `
		SELECT uc.id, uc.user_id, uc.credits, uc.created_at, uc.updated_at
		FROM user_credits uc
		ORDER BY uc.user_id
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allCredits []UserCredits
	for rows.Next() {
		var credits UserCredits
		err := rows.Scan(
			&credits.ID,
			&credits.UserID,
			&credits.Credits,
			&credits.CreatedAt,
			&credits.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		allCredits = append(allCredits, credits)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return allCredits, nil
}
