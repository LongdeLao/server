package routes

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"server/models"
	"server/notifications"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// SetupLeaveRequestRoutes registers all the routes for leave requests
func SetupLeaveRequestRoutes(router *gin.RouterGroup, db *sql.DB) {
	// Create a new leave request
	router.POST("/leave-requests", func(c *gin.Context) {
		var requestData struct {
			StudentID   int     `json:"student_id" binding:"required"`
			StudentName string  `json:"student_name" binding:"required"`
			RequestType string  `json:"request_type" binding:"required"`
			Reason      *string `json:"reason"`
		}

		if err := c.BindJSON(&requestData); err != nil {
			log.Printf("Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request data: " + err.Error(),
			})
			return
		}

		// Insert the new leave request
		var leaveRequest models.LeaveRequest
		err := db.QueryRow(`
			INSERT INTO leave_requests 
			(student_id, student_name, request_type, reason, status) 
			VALUES ($1, $2, $3, $4, 'pending') 
			RETURNING id, student_id, student_name, request_type, reason, status, created_at, updated_at`,
			requestData.StudentID, requestData.StudentName, requestData.RequestType, requestData.Reason).Scan(
			&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
			&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
			&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt)

		if err != nil {
			log.Printf("Error creating leave request: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to create leave request: " + err.Error(),
			})
			return
		}

		// Return the created leave request
		c.JSON(http.StatusCreated, models.LeaveRequestResponse{
			Success: true,
			Request: &leaveRequest,
		})
	})

	// Get a list of all pending leave requests (for staff members)
	router.GET("/leave-requests/pending", func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, student_id, student_name, request_type, reason, status, 
			       created_at, updated_at, responded_by, response_time, 
			       live_activity_id, live_activity_token
			FROM leave_requests 
			WHERE status = 'pending'
			ORDER BY created_at DESC`)

		if err != nil {
			log.Printf("Error getting pending leave requests: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestsResponse{
				Success: false,
				Message: "Failed to get pending leave requests: " + err.Error(),
			})
			return
		}
		defer rows.Close()

		var requests []models.LeaveRequest
		for rows.Next() {
			var req models.LeaveRequest
			if err := rows.Scan(
				&req.ID, &req.StudentID, &req.StudentName, &req.RequestType,
				&req.Reason, &req.Status, &req.CreatedAt, &req.UpdatedAt,
				&req.RespondedBy, &req.ResponseTime, &req.LiveActivityID, &req.LiveActivityToken); err != nil {
				log.Printf("Error scanning leave request: %v", err)
				continue
			}
			requests = append(requests, req)
		}

		c.JSON(http.StatusOK, models.LeaveRequestsResponse{
			Success:  true,
			Requests: requests,
		})
	})

	// Get all leave requests for a specific student
	router.GET("/leave-requests/student/:studentId", func(c *gin.Context) {
		studentIdStr := c.Param("studentId")
		studentId, err := strconv.Atoi(studentIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestsResponse{
				Success: false,
				Message: "Invalid student ID",
			})
			return
		}

		rows, err := db.Query(`
			SELECT id, student_id, student_name, request_type, reason, status, 
			       created_at, updated_at, responded_by, response_time, 
			       live_activity_id, live_activity_token
			FROM leave_requests 
			WHERE student_id = $1
			ORDER BY created_at DESC`, studentId)

		if err != nil {
			log.Printf("Error getting student leave requests: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestsResponse{
				Success: false,
				Message: "Failed to get student leave requests: " + err.Error(),
			})
			return
		}
		defer rows.Close()

		var requests []models.LeaveRequest
		for rows.Next() {
			var req models.LeaveRequest
			if err := rows.Scan(
				&req.ID, &req.StudentID, &req.StudentName, &req.RequestType,
				&req.Reason, &req.Status, &req.CreatedAt, &req.UpdatedAt,
				&req.RespondedBy, &req.ResponseTime, &req.LiveActivityID, &req.LiveActivityToken); err != nil {
				log.Printf("Error scanning leave request: %v", err)
				continue
			}
			requests = append(requests, req)
		}

		c.JSON(http.StatusOK, models.LeaveRequestsResponse{
			Success:  true,
			Requests: requests,
		})
	})

	// Update the status of a leave request (approve/reject)
	router.PUT("/leave-requests/:requestId/status", func(c *gin.Context) {
		requestIdStr := c.Param("requestId")
		requestId, err := strconv.Atoi(requestIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request ID",
			})
			return
		}

		var updateData struct {
			Status    string `json:"status" binding:"required"`
			StaffID   int    `json:"staff_id" binding:"required"`
			StaffName string `json:"staff_name"`
		}

		if err := c.BindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request data: " + err.Error(),
			})
			return
		}

		// Validate status
		if updateData.Status != "approved" && updateData.Status != "rejected" && updateData.Status != "finished" {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid status value. Must be 'approved', 'rejected', or 'finished'",
			})
			return
		}

		// Get current time for response_time
		responseTime := time.Now()

		// First get the existing request to check if it has live activity info
		var existingRequest models.LeaveRequest
		err = db.QueryRow(`
			SELECT id, live_activity_id, live_activity_token
			FROM leave_requests
			WHERE id = $1`, requestId).Scan(
			&existingRequest.ID, &existingRequest.LiveActivityID, &existingRequest.LiveActivityToken)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, models.LeaveRequestResponse{
					Success: false,
					Message: "Leave request not found",
				})
				return
			}
			log.Printf("Error getting existing leave request: %v", err)
		}

		// Update the leave request status
		var leaveRequest models.LeaveRequest
		err = db.QueryRow(`
			UPDATE leave_requests 
			SET status = $1, responded_by = $2, response_time = $3, updated_at = $3
			WHERE id = $4
			RETURNING id, student_id, student_name, request_type, reason, status, 
			          created_at, updated_at, responded_by, response_time, 
			          live_activity_id, live_activity_token`,
			updateData.Status, updateData.StaffID, responseTime, requestId).Scan(
			&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
			&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
			&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt, &leaveRequest.RespondedBy,
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityID, &leaveRequest.LiveActivityToken)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, models.LeaveRequestResponse{
					Success: false,
					Message: "Leave request not found",
				})
				return
			}

			log.Printf("Error updating leave request: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to update leave request: " + err.Error(),
			})
			return
		}

		// If we have live activity info, send push notification
		if leaveRequest.LiveActivityID != nil && leaveRequest.LiveActivityToken != nil {
			// Send a push notification to update the Live Activity
			go sendLiveActivityUpdate(leaveRequest, updateData.StaffName, responseTime)
		}

		// Return the updated leave request
		c.JSON(http.StatusOK, models.LeaveRequestResponse{
			Success: true,
			Request: &leaveRequest,
		})
	})

	// Get a specific leave request by ID
	router.GET("/leave-requests/:requestId", func(c *gin.Context) {
		requestIdStr := c.Param("requestId")
		requestId, err := strconv.Atoi(requestIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request ID",
			})
			return
		}

		var leaveRequest models.LeaveRequest
		err = db.QueryRow(`
			SELECT id, student_id, student_name, request_type, reason, status, 
			       created_at, updated_at, responded_by, response_time, 
			       live_activity_id, live_activity_token
			FROM leave_requests 
			WHERE id = $1`, requestId).Scan(
			&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
			&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
			&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt, &leaveRequest.RespondedBy,
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityID, &leaveRequest.LiveActivityToken)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, models.LeaveRequestResponse{
					Success: false,
					Message: "Leave request not found",
				})
				return
			}

			log.Printf("Error getting leave request: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to get leave request: " + err.Error(),
			})
			return
		}

		// Return the leave request
		c.JSON(http.StatusOK, models.LeaveRequestResponse{
			Success: true,
			Request: &leaveRequest,
		})
	})

	// Update live activity information for a leave request
	router.PUT("/leave-requests/:requestId/live-activity", func(c *gin.Context) {
		requestIdStr := c.Param("requestId")
		requestId, err := strconv.Atoi(requestIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request ID",
			})
			return
		}

		var updateData struct {
			LiveActivityID    string `json:"live_activity_id" binding:"required"`
			LiveActivityToken string `json:"live_activity_token" binding:"required"`
		}

		if err := c.BindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request data: " + err.Error(),
			})
			return
		}

		// Log the received Live Activity token for debugging
		log.Printf("🎯 Received Live Activity token for request ID %d:", requestId)
		log.Printf("Activity ID: %s", updateData.LiveActivityID)
		log.Printf("Token: %s", updateData.LiveActivityToken)

		// Update the live activity information
		var leaveRequest models.LeaveRequest
		err = db.QueryRow(`
			UPDATE leave_requests 
			SET live_activity_id = $1, live_activity_token = $2, updated_at = NOW()
			WHERE id = $3
			RETURNING id, student_id, student_name, request_type, reason, status, 
			          created_at, updated_at, responded_by, response_time, 
			          live_activity_id, live_activity_token`,
			updateData.LiveActivityID, updateData.LiveActivityToken, requestId).Scan(
			&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
			&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
			&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt, &leaveRequest.RespondedBy,
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityID, &leaveRequest.LiveActivityToken)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, models.LeaveRequestResponse{
					Success: false,
					Message: "Leave request not found",
				})
				return
			}

			log.Printf("Error updating live activity info: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to update live activity info: " + err.Error(),
			})
			return
		}

		// Return the updated leave request
		c.JSON(http.StatusOK, models.LeaveRequestResponse{
			Success: true,
			Request: &leaveRequest,
		})
	})
}

// Struct for Live Activity update payload
type LiveActivityPayload struct {
	APS struct {
		Event        string `json:"event"`
		Timestamp    int64  `json:"timestamp"`
		ContentState struct {
			Status       string    `json:"status"`
			ResponseTime time.Time `json:"responseTime"`
			RespondedBy  string    `json:"respondedBy"`
		} `json:"content-state"`
	} `json:"aps"`
	ActivityID string `json:"activity-id"`
}

// Send a push notification to update a Live Activity
func sendLiveActivityUpdate(request models.LeaveRequest, staffName string, responseTime time.Time) {
	if request.LiveActivityID == nil || request.LiveActivityToken == nil {
		log.Println("⚠️ Missing Live Activity info for leave request:", request.ID)
		return
	}

	// Create the push notification payload
	payload := LiveActivityPayload{}
	payload.APS.Event = "update"
	payload.APS.Timestamp = time.Now().Unix()
	payload.APS.ContentState.Status = request.Status
	payload.APS.ContentState.ResponseTime = responseTime
	payload.APS.ContentState.RespondedBy = staffName
	payload.ActivityID = *request.LiveActivityID

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling Live Activity payload: %v", err)
		return
	}

	log.Printf("📱 Sending Live Activity update for request %d with status %s", request.ID, request.Status)
	log.Printf("Payload: %s", jsonPayload)

	// The bundle ID for Live Activities needs .push-type.liveactivity appended
	bundleID := "com.leo.hsannu.push-type.liveactivity"
	deviceToken := *request.LiveActivityToken

	// Send the push notification
	resp, err := notifications.SendAPNsNotification(deviceToken, bundleID, string(jsonPayload), true)
	if err != nil {
		log.Printf("❌ Error sending Live Activity update: %v", err)
		return
	}

	log.Printf("✅ Live Activity update sent successfully: %s", resp)
}
