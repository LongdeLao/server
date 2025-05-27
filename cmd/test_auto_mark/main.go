// main.go - A simple test script to manually trigger the auto-marking function
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"server/routes"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

func main() {
	// Get the database connection from environment variable or use default
	dbConnectionString := os.Getenv("DB_CONNECTION_STRING")
	if dbConnectionString == "" {
		// Use your default connection string here
		dbConnectionString = "postgres://postgres:password@localhost:5432/hsannu?sslmode=disable"
		fmt.Println("Using default database connection string. Set DB_CONNECTION_STRING env var to override.")
	}

	// Connect to the database
	fmt.Println("Connecting to database...")
	db, err := sql.Open("postgres", dbConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Ping database to verify connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Database connection successful!")

	// Get current time in UTC and display
	now := time.Now().UTC()
	localTime := time.Now()
	fmt.Printf("Current UTC time: %s\n", now.Format(time.RFC3339))
	fmt.Printf("Current local time: %s\n", localTime.Format(time.RFC3339))

	// Display Shanghai time (UTC+8)
	shanghaiLoc, _ := time.LoadLocation("Asia/Shanghai")
	if shanghaiLoc == nil {
		// Fallback if time zone data is not available
		shanghaiOffset := 8 * 60 * 60 // UTC+8 in seconds
		shanghaiTime := localTime.In(time.FixedZone("Asia/Shanghai", shanghaiOffset))
		fmt.Printf("Current Shanghai time (UTC+8): %s\n", shanghaiTime.Format(time.RFC3339))
	} else {
		shanghaiTime := localTime.In(shanghaiLoc)
		fmt.Printf("Current Shanghai time: %s\n", shanghaiTime.Format(time.RFC3339))
	}

	// Display India time (UTC+5:30)
	indiaLoc, _ := time.LoadLocation("Asia/Kolkata")
	if indiaLoc == nil {
		// Fallback if time zone data is not available
		indiaOffset := (5*60 + 30) * 60 // UTC+5:30 in seconds
		indiaTime := localTime.In(time.FixedZone("Asia/Kolkata", indiaOffset))
		fmt.Printf("Current India time (UTC+5:30): %s\n", indiaTime.Format(time.RFC3339))
	} else {
		indiaTime := localTime.In(indiaLoc)
		fmt.Printf("Current India time: %s\n", indiaTime.Format(time.RFC3339))
	}

	// Set target time to 20:12 today in UTC
	currentYear, currentMonth, currentDay := now.Date()
	targetTime := time.Date(currentYear, currentMonth, currentDay, 20, 12, 0, 0, time.UTC)

	fmt.Printf("Target test time in UTC: %s\n", targetTime.Format(time.RFC3339))

	// In production, the target would be 7:40 AM Shanghai time
	// Show what that would be in UTC for reference
	// Shanghai is UTC+8, so 7:40 AM Shanghai = 23:40 UTC (previous day)
	productionYear, productionMonth, productionDay := now.Date()
	productionTime := time.Date(productionYear, productionMonth, productionDay, 23, 40, 0, 0, time.UTC)
	fmt.Printf("Production target time (7:40 AM Shanghai) in UTC: %s\n", productionTime.Format(time.RFC3339))

	// Check if target time is in the future
	if targetTime.After(now) {
		waitDuration := targetTime.Sub(now)
		fmt.Printf("Waiting %.2f seconds until %s to trigger auto-marking...\n",
			waitDuration.Seconds(), targetTime.Format(time.RFC3339))
		time.Sleep(waitDuration)
		now = time.Now().UTC() // Update now after sleep
	}

	fmt.Printf("Triggering auto-marking of late students at %s (UTC)\n", now.Format(time.RFC3339))

	// Call the auto-marking function with the target time
	routes.AutoMarkLateStudents(db, &targetTime)

	fmt.Println("Auto-marking process completed!")
}
