package routes

import (
	"fmt"
	"net/http"
	"server/config"
	"time"

	"github.com/gin-gonic/gin"
)

/**
 * StatusResponse represents the server status response.
 */
type StatusResponse struct {
	Status          string    `json:"status"`
	Message         string    `json:"message"`
	Timestamp       time.Time `json:"timestamp"`
	Version         string    `json:"version"`
	Uptime          string    `json:"uptime"`
	IsActive        bool      `json:"is_active"`
	Environment     string    `json:"environment"`
	EstimatedFinish string    `json:"estimated_finish,omitempty"`
}

var serverStartTime = time.Now()

/**
 * RegisterStatusRoute registers the server status check route.
 *
 * Endpoint: GET /check-status
 *
 * Returns:
 *   - 200 OK: Server is active
 *     {
 *       "status": string,
 *       "message": string,
 *       "timestamp": string,
 *       "version": string,
 *       "uptime": string,
 *       "is_active": boolean,
 *       "environment": string,
 *       "estimated_finish": string (optional)
 *     }
 *   - 503 Service Unavailable: Server is in maintenance or construction mode
 */
func RegisterStatusRoute(router gin.IRouter) {
	router.GET("/check-status", func(c *gin.Context) {
		uptime := time.Since(serverStartTime)
		uptimeStr := formatDuration(uptime)

		isActive := config.ServerStatus == "active"

		response := StatusResponse{
			Status:          config.ServerStatus,
			Message:         config.StatusMessage,
			Timestamp:       time.Now(),
			Version:         "1.0.0",
			Uptime:          uptimeStr,
			IsActive:        isActive,
			Environment:     getEnvironment(),
			EstimatedFinish: config.EstimatedFinish,
		}

		statusCode := http.StatusOK
		if config.ServerStatus == "maintenance" || config.ServerStatus == "construction" {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, response)
	})
}

/**
 * formatDuration formats a duration into a human-readable string.
 *
 * Parameters:
 *   - d: time.Duration to format
 *
 * Returns:
 *   - string: Formatted duration (e.g., "2d 5h 30m 15s")
 */
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

/**
 * getEnvironment determines the current environment based on server status.
 *
 * Returns:
 *   - string: Environment name ("production", "maintenance", "development", "unknown")
 */
func getEnvironment() string {
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
