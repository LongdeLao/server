package models

import (
	"time"
)

// LeaveRequest represents a student's leave request
type LeaveRequest struct {
	ID                int        `json:"id"`
	StudentID         int        `json:"student_id"`
	StudentName       string     `json:"student_name"`
	RequestType       string     `json:"request_type"`
	Reason            *string    `json:"reason"`
	Status            string     `json:"status"` // Values: "sent", "pending", "approved", "rejected", "cancelled", "finished"
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	RespondedBy       *int       `json:"responded_by"`
	ResponseTime      *time.Time `json:"response_time"`
	LiveActivityId    *string    `json:"live_activity_id"`
	LiveActivityToken *string    `json:"live_activity_token"`
}

// StatusDisplayInfo contains UI display data for each status
type StatusDisplayInfo struct {
	Color      string `json:"color"`       // Hex color code
	SystemIcon string `json:"system_icon"` // SF Symbol name for iOS/macOS
}

// GetStatusInfo returns display info for a given status
func GetStatusInfo(status string) StatusDisplayInfo {
	switch status {
	case "sent":
		return StatusDisplayInfo{
			Color:      "#3478F6", // Blue
			SystemIcon: "paperplane.fill",
		}
	case "pending":
		return StatusDisplayInfo{
			Color:      "#F5A623", // Orange
			SystemIcon: "clock.fill",
		}
	case "approved":
		return StatusDisplayInfo{
			Color:      "#34C759", // Green
			SystemIcon: "checkmark.circle.fill",
		}
	case "rejected":
		return StatusDisplayInfo{
			Color:      "#FF3B30", // Red
			SystemIcon: "xmark.circle.fill",
		}
	case "cancelled":
		return StatusDisplayInfo{
			Color:      "#8E8E93", // Gray
			SystemIcon: "xmark.bin.fill",
		}
	case "finished":
		return StatusDisplayInfo{
			Color:      "#8E8E93", // Gray
			SystemIcon: "flag.checkered",
		}
	default:
		return StatusDisplayInfo{
			Color:      "#8E8E93", // Gray
			SystemIcon: "questionmark.circle",
		}
	}
}

// LeaveRequestResponse is the API response format for leave requests
type LeaveRequestResponse struct {
	Success bool          `json:"success"`
	Request *LeaveRequest `json:"request,omitempty"`
	Message string        `json:"message,omitempty"`
}

// LeaveRequestsResponse is the API response format for multiple leave requests
type LeaveRequestsResponse struct {
	Success  bool           `json:"success"`
	Requests []LeaveRequest `json:"requests"`
	Message  string         `json:"message,omitempty"`
}
