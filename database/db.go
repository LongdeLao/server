package db

import (
	"database/sql"
	"fmt"
	"server/config" // Adjust this if your config package is elsewhere

	_ "github.com/lib/pq" // PostgreSQL driver
)

// GetConnection returns a connection to the PostgreSQL database.
func GetConnection() (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	return sql.Open("postgres", connStr)
}
