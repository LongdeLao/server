package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"server/config"        // Your configuration package.
	"server/notifications" // Import the notifications package
	"server/routes"        // Adjust the import path based on your module.

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL driver.
)

func main() {
	// Initialize the Gin router.
	router := gin.Default()

	// Connect to your PostgreSQL database.
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost,
		config.DBPort,
		config.DBUser,
		config.DBPassword,
		config.DBName,
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	// Initialize the APNs client
	if err := notifications.InitAPNS(); err != nil {
		log.Printf("Warning: Failed to initialize APNs: %v", err)
		// We continue anyway, as APNs may not be crucial for the app to function
	}

	// Set up static file serving for images
	router.Static("/images", "./images")
	router.Static("/profile_pictures", "./profile_pictures")

	// Also serve static files under /api prefix to maintain compatibility
	router.Static("/api/profile_pictures", "./profile_pictures")
	router.Static("/api/images", "./images")

	// Create an API router group
	apiRouter := router.Group("/api")

	// Register your routes under the API router group
	routes.RegisterLoginRoute(apiRouter)
	routes.RegisterEventRoutes(apiRouter, db)
	routes.RegisterGetAllEvents(apiRouter, db)
	routes.RegisterGetEventByID(apiRouter, db)
	routes.RegisterGetSubjectsRoute(apiRouter, db)
	routes.RegisterGetSubjectsTeacherRoute(apiRouter, db)
	routes.RegisterProfileRoutes(apiRouter, db)
	routes.SetupAttendanceRoutes(apiRouter, db)
	routes.SetupUserRoutes(apiRouter, db)
	routes.SetupMessagingRoutes(apiRouter, db)
	routes.RegisterTestRoute(apiRouter)

	// Print local non-loopback IPv4 addresses.
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Error getting IP addresses: %v", err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				fmt.Printf("Running on IP: %s\n", ip4.String())
			}
		}
	}

	// Start the server on port 2000.
	log.Printf("Starting server on port %s...", config.ServerPort)
	if err := router.Run(fmt.Sprintf(":%s", config.ServerPort)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
