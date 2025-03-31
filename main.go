package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"server/config" // Your configuration package.
	"server/routes" // Adjust the import path based on your module.

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

	// Register your routes.
	routes.RegisterLoginRoute(router)
	routes.RegisterEventRoutes(router, db)
	routes.RegisterGetAllEvents(router,db)
    routes.RegisterGetEventByID(router,db)
	routes.RegisterGetSubjectsRoute(router, db)
	routes.RegisterGetSubjectsTeacherRoute(router,db)
	
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
	if err := router.Run(":2000"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
