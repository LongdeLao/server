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
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	RespondedBy       *int       `json:"responded_by"`
	ResponseTime      *time.Time `json:"response_time"`
	LiveActivityID    *string    `json:"live_activity_id"`
	LiveActivityToken *string    `json:"live_activity_token"`
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
