package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"server/config"        // Your configuration package.
	"server/notifications" // Import the notifications package
	"server/routes"        // Adjust the import path based on your module.
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL driver.
)

// CacheMiddleware adds Cache-Control headers for static assets
func CacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/images/") ||
			strings.HasPrefix(c.Request.URL.Path, "/profile_pictures/") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/images/") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/profile_pictures/") ||
			strings.HasPrefix(c.Request.URL.Path, "/document-files/") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/document-files/") ||
			strings.HasPrefix(c.Request.URL.Path, "/student_formal_images/") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/student_formal_images/") {
			c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day (86400 seconds)
		}
		c.Next()
	}
}

// setupAutoMarkScheduler sets up a background task to automatically mark students as late
// at the configured time on weekdays
func setupAutoMarkScheduler(db *sql.DB) {
	go func() {
		log.Println("Starting auto-mark scheduler...")

		// Create a ticker that checks every minute
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				now := time.Now().UTC()

				// Skip if auto-marking is disabled
				if !config.AutoMarkEnabled {
					continue
				}

				// Check if force auto-mark is enabled for testing
				if config.ForceAutoMark {
					log.Printf("Forced auto-marking triggered at %s (UTC)", now.Format(time.RFC3339))
					routes.AutoMarkLateStudents(db, nil)
					config.ForceAutoMark = false // Reset flag after use
					continue
				}

				// Check if it's a weekday (Monday-Friday)
				if now.Weekday() >= time.Monday && now.Weekday() <= time.Friday {
					// Check if current time matches the configured auto-mark time
					if now.Hour() == config.AutoMarkHour && now.Minute() == config.AutoMarkMinute {
						log.Printf("Running scheduled auto-marking at %s (UTC)", now.Format(time.RFC3339))

						// Call the auto-marking function with nil to use current time
						routes.AutoMarkLateStudents(db, nil)

						// Sleep for 70 seconds to avoid running twice if the check happens right at the configured minute
						time.Sleep(70 * time.Second)
					}
				}
			}
		}
	}()
}

// promptForServerStatus asks the user to select the server status interactively
func promptForServerStatus() {
	fmt.Println("🚀 HSANNU Server Configuration")
	fmt.Println("==============================")
	fmt.Println()
	fmt.Println("Please select the server status:")
	fmt.Println("  (a) Active - Server is running normally")
	fmt.Println("  (m) Maintenance - Server is under maintenance")
	fmt.Println("  (c) Construction - Server is under construction")
	fmt.Println()
	fmt.Print("Enter your choice [a/m/c] (default: a): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	// Set status based on user input
	switch input {
	case "a", "active", "":
		config.ServerStatus = "active"
		config.StatusMessage = "Server is running normally"
		config.EstimatedFinish = ""
		fmt.Println("✅ Server status set to: ACTIVE")
	case "m", "maintenance":
		config.ServerStatus = "maintenance"
		config.StatusMessage = "Server is under maintenance. Please try again later."
		fmt.Println("🔧 Server status set to: MAINTENANCE")

		// Ask for custom maintenance message
		fmt.Print("Enter custom maintenance message (optional): ")
		customMessage, _ := reader.ReadString('\n')
		customMessage = strings.TrimSpace(customMessage)
		if customMessage != "" {
			config.StatusMessage = customMessage
		}

		// Ask for estimated finish time
		fmt.Print("Enter estimated finish time (e.g., '2 hours', 'tomorrow 3pm', '30 minutes'): ")
		estimatedFinish, _ := reader.ReadString('\n')
		estimatedFinish = strings.TrimSpace(estimatedFinish)
		if estimatedFinish != "" {
			config.EstimatedFinish = estimatedFinish
		}
	case "c", "construction":
		config.ServerStatus = "construction"
		config.StatusMessage = "Server is under construction. New features are being added."
		fmt.Println("🚧 Server status set to: CONSTRUCTION")

		// Ask for custom construction message
		fmt.Print("Enter custom construction message (optional): ")
		customMessage, _ := reader.ReadString('\n')
		customMessage = strings.TrimSpace(customMessage)
		if customMessage != "" {
			config.StatusMessage = customMessage
		}

		// Ask for estimated finish time
		fmt.Print("Enter estimated completion time (e.g., '1 week', 'next Monday', '3 days'): ")
		estimatedFinish, _ := reader.ReadString('\n')
		estimatedFinish = strings.TrimSpace(estimatedFinish)
		if estimatedFinish != "" {
			config.EstimatedFinish = estimatedFinish
		}
	default:
		fmt.Printf("❌ Invalid choice '%s'. Defaulting to ACTIVE.\n", input)
		config.ServerStatus = "active"
		config.StatusMessage = "Server is running normally"
		config.EstimatedFinish = ""
	}

	fmt.Println()
	fmt.Printf("📝 Status Message: %s\n", config.StatusMessage)
	if config.EstimatedFinish != "" {
		fmt.Printf("⏰ Estimated Finish: %s\n", config.EstimatedFinish)
	}
	fmt.Println()
}

func main() {
	// Prompt for server status configuration
	promptForServerStatus()

	// Set Gin to production mode
	gin.SetMode(gin.ReleaseMode)

	// Initialize random number generator
	rand.Seed(time.Now().UnixNano())

	// Initialize the Gin router.
	router := gin.Default()

	// Apply caching middleware globally or to specific routes
	router.Use(CacheMiddleware())

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

	// Set up auto-marking scheduler
	setupAutoMarkScheduler(db)
	log.Printf("Auto-marking scheduler started - will run at %02d:%02d UTC (%02d:%02d Shanghai time) on weekdays",
		config.AutoMarkHour, config.AutoMarkMinute,
		(config.AutoMarkHour+8)%24, config.AutoMarkMinute)

	// Initialize the APNs client
	if err := notifications.InitAPNS(); err != nil {
		log.Printf("Warning: Failed to initialize APNs: %v", err)
		// We continue anyway, as APNs may not be crucial for the app to function
	}

	// Set up static file serving for images
	router.Static("/images", "./images")
	router.Static("/profile_pictures", "./profile_pictures")
	router.Static("/document-files", "./documents")
	router.Static("/student_formal_images", "./student_formal_images") // Add student formal images

	// Also serve static files under /api prefix to maintain compatibility
	router.Static("/api/profile_pictures", "./profile_pictures")
	router.Static("/api/images", "./images")
	router.Static("/api/document-files", "./documents")
	router.Static("/api/student_formal_images", "./student_formal_images") // Add student formal images under API prefix

	// Create an API router group
	apiRouter := router.Group("/api")

	// Register your routes under the API router group
	routes.RegisterLoginRoute(apiRouter)
	routes.RegisterAuthRoutes(apiRouter, db)
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

	// Register the new leave request routes
	routes.SetupLeaveRequestRoutes(apiRouter, db)

	// Register voting system routes
	routes.SetupVotingRoutes(apiRouter, db)

	// Register passkey authentication routes
	routes.SetupPasskeyRoutes(apiRouter, db)

	// Register student routes
	routes.SetupStudentRoutes(apiRouter, db)

	// Register document hub routes
	routes.SetupDocumentRoutes(apiRouter, db)

	// Register AI configuration routes
	routes.RegisterAIConfigRoutes(apiRouter)

	// Register server status route
	routes.RegisterStatusRoute(apiRouter)

	// Register credits routes
	routes.SetupCreditsRoutes(apiRouter, db)

	// Add a test route to trigger auto-marking immediately
	registerTestAutoMarkRoute(apiRouter, db)

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
	log.Printf("Server Status: %s", config.ServerStatus)
	log.Printf("Status Message: %s", config.StatusMessage)
	log.Printf("IS IT RUNNING?")
	if err := router.Run(fmt.Sprintf(":%s", config.ServerPort)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Add a test route to trigger auto-marking immediately
func registerTestAutoMarkRoute(apiRouter *gin.RouterGroup, db *sql.DB) {
	apiRouter.GET("/test/auto-mark", func(c *gin.Context) {
		// Get the time parameter if provided
		timeStr := c.Query("time")
		var targetTime *time.Time

		if timeStr != "" {
			// Try to parse the time string (format: HH:MM)
			parts := strings.Split(timeStr, ":")
			if len(parts) == 2 {
				hour, errH := strconv.Atoi(parts[0])
				min, errM := strconv.Atoi(parts[1])
				if errH == nil && errM == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 {
					now := time.Now().UTC()
					t := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, time.UTC)
					targetTime = &t
				}
			}
		}

		// Run the auto-marking function
		go func() {
			if targetTime != nil {
				log.Printf("Running auto-marking test with time: %s (UTC)", targetTime.Format(time.RFC3339))
				routes.AutoMarkLateStudents(db, targetTime)
			} else {
				log.Printf("Running auto-marking test with current time")
				routes.AutoMarkLateStudents(db, nil)
			}
		}()

		c.JSON(200, gin.H{
			"success": true,
			"message": "Auto-marking test triggered successfully",
		})
	})

	// Add a route to update auto-marking configuration
	apiRouter.POST("/settings/auto-mark", func(c *gin.Context) {
		var request struct {
			Hour    *int  `json:"hour"`
			Minute  *int  `json:"minute"`
			Enabled *bool `json:"enabled"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request format",
			})
			return
		}

		// Update the configuration if values are provided
		if request.Hour != nil && *request.Hour >= 0 && *request.Hour < 24 {
			config.AutoMarkHour = *request.Hour
		}

		if request.Minute != nil && *request.Minute >= 0 && *request.Minute < 60 {
			config.AutoMarkMinute = *request.Minute
		}

		if request.Enabled != nil {
			config.AutoMarkEnabled = *request.Enabled
		}

		// Force auto-marking to run on next check if hour:minute is in the past for today
		if c.Query("run_now") == "true" {
			config.ForceAutoMark = true
		}

		c.JSON(200, gin.H{
			"success": true,
			"message": "Auto-marking settings updated successfully",
			"settings": gin.H{
				"hour":    config.AutoMarkHour,
				"minute":  config.AutoMarkMinute,
				"enabled": config.AutoMarkEnabled,
			},
		})
	})
}
