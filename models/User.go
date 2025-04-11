package models

import (
	"database/sql"
)

// User represents a user in the system, including their roles and additional roles.
type User struct {
	ID              int      `json:"id"`
	Username        string   `json:"username"`
	Name            string   `json:"name"`
	Password        string   `json:"password,omitempty"` // omit from JSON responses for security
	Role            string   `json:"role"`
	AdditionalRoles []string `json:"additional_roles"` // New field to hold additional roles
	ProfilePicture  string   `json:"profile_picture"`  // URL or path to the user's profile picture
	Email           string   `json:"email"`            // User's email address
	FirstName       string   `json:"first_name"`
	LastName        string   `json:"last_name"`
	DeviceID        string   `json:"device_id"`
	Status          string   `json:"status"` // User account status (active, inactive, suspended, etc.)
}

// GetAllUsers retrieves all users from the database
func GetAllUsers(db *sql.DB) ([]User, error) {
	query := `
		SELECT id, first_name, last_name, name, username, 
		       password, role, device_id, email, status
		FROM users
		ORDER BY id
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
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
			&user.Username,
			&user.Password,
			&user.Role,
			&user.DeviceID,
			&user.Email,
			&user.Status,
		)
		if err != nil {
			return nil, err
		}

		// For security, clear the password before sending to client
		user.Password = ""

		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// UserExistsByEmail checks if a user with the given email exists
func UserExistsByEmail(db *sql.DB, email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	err := db.QueryRow(query, email).Scan(&exists)
	return exists, err
}

// GetUserByEmail retrieves a user by their email address
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	var user User
	query := `
		SELECT id, first_name, last_name, name, username, 
		       password, role, device_id, email, status
		FROM users
		WHERE email = $1
	`
	err := db.QueryRow(query, email).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Name,
		&user.Username,
		&user.Password,
		&user.Role,
		&user.DeviceID,
		&user.Email,
		&user.Status,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
