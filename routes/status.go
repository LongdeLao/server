package routes

import (
	"fmt"
	"net/http"
	"server/config"
	"time"

	"github.com/gin-gonic/gin"
)

// StatusResponse represents the server status response
type StatusResponse struct {
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Version     string    `json:"version"`
	Uptime      string    `json:"uptime"`
	IsActive    bool      `json:"is_active"`
	Environment string    `json:"environment"`
}

var serverStartTime = time.Now()

// RegisterStatusRoute registers the server status check route
func RegisterStatusRoute(router gin.IRouter) {
	router.GET("/check-status", func(c *gin.Context) {
		// Calculate uptime
		uptime := time.Since(serverStartTime)
		uptimeStr := formatDuration(uptime)

		// Determine if server is active based on status
		isActive := config.ServerStatus == "active"

		// Create response
		response := StatusResponse{
			Status:      config.ServerStatus,
			Message:     config.StatusMessage,
			Timestamp:   time.Now(),
			Version:     "1.0.0", // You can make this configurable too
			Uptime:      uptimeStr,
			IsActive:    isActive,
			Environment: getEnvironment(),
		}

		// Set appropriate HTTP status code
		statusCode := http.StatusOK
		if config.ServerStatus == "maintenance" || config.ServerStatus == "construction" {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, response)
	})
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// getEnvironment determines the current environment
func getEnvironment() string {
	// You can make this more sophisticated by checking environment variables
	// For now, we'll determine based on the server status
	switch config.ServerStatus {
	case "active":
		return "production"
	case "maintenance":
		return "maintenance"
	case "construction":
		return "development"
	default:
		return "unknown"
	}
}
