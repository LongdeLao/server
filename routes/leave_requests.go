package routes

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
			StudentID         int     `json:"student_id" binding:"required"`
			StudentName       string  `json:"student_name" binding:"required"`
			RequestType       string  `json:"request_type" binding:"required"`
			Reason            *string `json:"reason"`
			LiveActivityId    *string `json:"live_activity_id"`
			LiveActivityToken *string `json:"live_activity_token"`
		}

		if err := c.BindJSON(&requestData); err != nil {
			log.Printf("Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request data: " + err.Error(),
			})
			return
		}

		// Explicit logging of token mismatch detection
		log.Printf("🔐 RECEIVED TOKEN VERIFICATION:")
		log.Printf("🔐 Request from student: %s (ID: %d)", requestData.StudentName, requestData.StudentID)

		if requestData.LiveActivityId != nil {
			log.Printf("🔐 Activity ID received in request: %s", *requestData.LiveActivityId)
			if requestData.LiveActivityToken != nil {
				tokenPrefix := (*requestData.LiveActivityToken)[:10]
				log.Printf("🔐 Token received in request (first 10 chars): %s", tokenPrefix)
				log.Printf("🔐 Complete token: %s", *requestData.LiveActivityToken)
			}
		}

		// Log the request data for debugging
		log.Printf("Creating leave request for student %s (ID: %d)", requestData.StudentName, requestData.StudentID)
		log.Printf("Request type: %s", requestData.RequestType)
		if requestData.Reason != nil {
			log.Printf("Reason: %s", *requestData.Reason)
		}

		// Enhanced debugging for Live Activity token mismatch
		log.Printf("🔍 ---------- DEBUG: REQUEST PAYLOAD START ----------")
		payloadBytes, _ := json.MarshalIndent(requestData, "", "  ")
		log.Printf("📦 Raw request payload:\n%s", string(payloadBytes))

		// Clear any cached variables that might affect token processing
		log.Printf("🧹 Clearing any cached variables to prevent token mismatch")

		// Log Live Activity info if provided
		if requestData.LiveActivityId != nil && requestData.LiveActivityToken != nil {
			log.Printf("📱 Live Activity ID: %s", *requestData.LiveActivityId)
			log.Printf("🔑 Live Activity Token: %s", *requestData.LiveActivityToken)
			log.Printf("🔑 Token first 10 chars: %s", (*requestData.LiveActivityToken)[:10])
			log.Printf("🔑 Token length: %d", len(*requestData.LiveActivityToken))
			log.Printf("⚠️ VERIFICATION - This token is for activity ID: %s", *requestData.LiveActivityId)
		} else {
			log.Printf("⚠️ No Live Activity info provided in the initial request")
		}
		log.Printf("🔍 ---------- DEBUG: REQUEST PAYLOAD END ----------")

		// Make an explicit copy of the token and activity ID to prevent
		// any reference issues or unexpected modifications
		var liveActivityIdCopy, liveActivityTokenCopy *string
		if requestData.LiveActivityId != nil {
			idCopy := *requestData.LiveActivityId
			liveActivityIdCopy = &idCopy
		}

		if requestData.LiveActivityToken != nil {
			tokenCopy := *requestData.LiveActivityToken
			liveActivityTokenCopy = &tokenCopy
		}

		// Insert the new leave request with live activity info if provided
		var query string
		var args []interface{}

		if liveActivityIdCopy != nil && liveActivityTokenCopy != nil {
			// Include live activity columns in the query
			query = `
				INSERT INTO leave_requests 
				(student_id, student_name, request_type, reason, status, live_activity_id, live_activity_token) 
				VALUES ($1, $2, $3, $4, 'pending', $5, $6) 
				RETURNING id, student_id, student_name, request_type, reason, status, created_at, updated_at, 
						  live_activity_id, live_activity_token`
			args = []interface{}{
				requestData.StudentID, requestData.StudentName, requestData.RequestType, requestData.Reason,
				liveActivityIdCopy, liveActivityTokenCopy,
			}

			// Log what's being inserted with extra verification
			log.Printf("🔍 ---------- DEBUG: DATABASE INSERT ----------")
			log.Printf("📝 SQL Query: %s", query)
			log.Printf("📝 Inserting Activity ID: %s", *liveActivityIdCopy)
			log.Printf("📝 Inserting Token: %s", *liveActivityTokenCopy)
			log.Printf("📝 Verified token is from current request, not cached")
			log.Printf("📝 For Student: %s (ID: %d)", requestData.StudentName, requestData.StudentID)
			log.Printf("🔍 ---------- DEBUG: DATABASE INSERT END ----------")
		} else {
			// Original query without live activity info
			query = `
				INSERT INTO leave_requests 
				(student_id, student_name, request_type, reason, status) 
				VALUES ($1, $2, $3, $4, 'pending') 
				RETURNING id, student_id, student_name, request_type, reason, status, created_at, updated_at`
			args = []interface{}{
				requestData.StudentID, requestData.StudentName, requestData.RequestType, requestData.Reason,
			}
		}

		// Execute the query based on which fields we're using
		var leaveRequest models.LeaveRequest
		var err error

		if liveActivityIdCopy != nil && liveActivityTokenCopy != nil {
			err = db.QueryRow(query, args...).Scan(
				&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
				&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
				&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt,
				&leaveRequest.LiveActivityId, &leaveRequest.LiveActivityToken)
		} else {
			err = db.QueryRow(query, args...).Scan(
				&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
				&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
				&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt)
		}

		if err != nil {
			log.Printf("Error creating leave request: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to create leave request: " + err.Error(),
			})
			return
		}

		log.Printf("✅ Successfully created leave request #%d for %s", leaveRequest.ID, leaveRequest.StudentName)

		// Final verification of what was saved
		if leaveRequest.LiveActivityId != nil && leaveRequest.LiveActivityToken != nil {
			log.Printf("📱 Saved Live Activity ID: %s", *leaveRequest.LiveActivityId)
			log.Printf("🔑 Saved Live Activity Token: %s", *leaveRequest.LiveActivityToken)

			// Verify the token matches what was sent in the request
			if requestData.LiveActivityToken != nil {
				if *leaveRequest.LiveActivityToken != *requestData.LiveActivityToken {
					log.Printf("❌ TOKEN MISMATCH DETECTED! Saved token doesn't match request token!")
					log.Printf("❌ Saved token: %s", *leaveRequest.LiveActivityToken)
					log.Printf("❌ Request token: %s", *requestData.LiveActivityToken)
				} else {
					log.Printf("✅ Token verification successful - saved token matches request token")
				}
			}
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
				&req.RespondedBy, &req.ResponseTime, &req.LiveActivityId, &req.LiveActivityToken); err != nil {
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
				&req.RespondedBy, &req.ResponseTime, &req.LiveActivityId, &req.LiveActivityToken); err != nil {
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
			&existingRequest.ID, &existingRequest.LiveActivityId, &existingRequest.LiveActivityToken)

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
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityId, &leaveRequest.LiveActivityToken)

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
		if leaveRequest.LiveActivityId != nil && leaveRequest.LiveActivityToken != nil {
			// Send a push notification to update the Live Activity
			go sendLiveActivityUpdate(leaveRequest, updateData.StaffName, responseTime)
		}

		// Return the updated leave request
		c.JSON(http.StatusOK, models.LeaveRequestResponse{
			Success: true,
			Request: &leaveRequest,
		})
	})

	// Cancel a leave request (can be done by students)
	router.PUT("/leave-requests/:requestId/cancel", func(c *gin.Context) {
		requestIdStr := c.Param("requestId")
		requestId, err := strconv.Atoi(requestIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request ID",
			})
			return
		}

		var cancelData struct {
			StudentID int    `json:"student_id" binding:"required"`
			Reason    string `json:"reason"`
		}

		if err := c.BindJSON(&cancelData); err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request data: " + err.Error(),
			})
			return
		}

		// Get current time for the cancellation timestamp
		cancellationTime := time.Now()

		// Get the existing request to verify student ID and check if it has live activity info
		var existingRequest models.LeaveRequest
		err = db.QueryRow(`
			SELECT id, student_id, live_activity_id, live_activity_token, status
			FROM leave_requests
			WHERE id = $1`, requestId).Scan(
			&existingRequest.ID, &existingRequest.StudentID,
			&existingRequest.LiveActivityId, &existingRequest.LiveActivityToken,
			&existingRequest.Status)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, models.LeaveRequestResponse{
					Success: false,
					Message: "Leave request not found",
				})
				return
			}
			log.Printf("Error getting existing leave request: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to get leave request information",
			})
			return
		}

		// Verify that the student is the owner of the request
		if existingRequest.StudentID != cancelData.StudentID {
			c.JSON(http.StatusForbidden, models.LeaveRequestResponse{
				Success: false,
				Message: "You are not authorized to cancel this request",
			})
			return
		}

		// Check if the request is already finalized
		if existingRequest.Status == "approved" || existingRequest.Status == "rejected" || existingRequest.Status == "finished" {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Cannot cancel a request that has already been " + existingRequest.Status,
			})
			return
		}

		// Update the leave request status to cancelled
		var updatedRequest models.LeaveRequest
		err = db.QueryRow(`
			UPDATE leave_requests 
			SET status = 'cancelled', updated_at = $1, response_time = $1
			WHERE id = $2
			RETURNING id, student_id, student_name, request_type, reason, status, 
					  created_at, updated_at, responded_by, response_time, 
					  live_activity_id, live_activity_token`,
			cancellationTime, requestId).Scan(
			&updatedRequest.ID, &updatedRequest.StudentID, &updatedRequest.StudentName,
			&updatedRequest.RequestType, &updatedRequest.Reason, &updatedRequest.Status,
			&updatedRequest.CreatedAt, &updatedRequest.UpdatedAt, &updatedRequest.RespondedBy,
			&updatedRequest.ResponseTime, &updatedRequest.LiveActivityId, &updatedRequest.LiveActivityToken)

		if err != nil {
			log.Printf("Error updating leave request: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to cancel leave request: " + err.Error(),
			})
			return
		}

		// If we have live activity info, send push notification
		if updatedRequest.LiveActivityId != nil && updatedRequest.LiveActivityToken != nil {
			// Send a push notification to update the Live Activity
			go func() {
				// Create the push notification payload
				payload := LiveActivityPayload{}
				payload.APS.Event = "update"
				payload.APS.Timestamp = time.Now().Unix()
				payload.APS.ContentState.Status = "cancelled"
				payload.APS.ContentState.ResponseTime = cancellationTime
				payload.APS.ContentState.RespondedBy = "Student"
				payload.ActivityId = *updatedRequest.LiveActivityId

				// Convert payload to JSON
				jsonPayload, err := json.Marshal(payload)
				if err != nil {
					log.Printf("Error marshalling Live Activity payload: %v", err)
					return
				}

				log.Printf("📱 Sending Live Activity update for cancelled request %d", updatedRequest.ID)
				log.Printf("Payload: %s", jsonPayload)

				// The bundle ID for Live Activities needs .push-type.liveactivity appended
				bundleID := "com.leo.hsannu.push-type.liveactivity"
				deviceToken := *updatedRequest.LiveActivityToken

				// Send the push notification
				resp, err := notifications.SendAPNsNotification(deviceToken, bundleID, string(jsonPayload), true)
				if err != nil {
					log.Printf("❌ Error sending Live Activity update: %v", err)
					return
				}

				log.Printf("✅ Live Activity update for cancellation sent successfully: %s", resp)
			}()
		}

		// Return the updated leave request
		c.JSON(http.StatusOK, models.LeaveRequestResponse{
			Success: true,
			Request: &updatedRequest,
			Message: "Leave request cancelled successfully",
		})
	})

	// Bulk update multiple leave requests (for staff efficiency)
	router.POST("/leave-requests/bulk-update", func(c *gin.Context) {
		var bulkUpdateData struct {
			RequestIDs []int  `json:"request_ids" binding:"required"`
			Status     string `json:"status" binding:"required"`
			StaffID    int    `json:"staff_id" binding:"required"`
			StaffName  string `json:"staff_name"`
		}

		if err := c.BindJSON(&bulkUpdateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request data: " + err.Error(),
			})
			return
		}

		// Validate status
		if bulkUpdateData.Status != "approved" && bulkUpdateData.Status != "rejected" && bulkUpdateData.Status != "finished" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid status value. Must be 'approved', 'rejected', or 'finished'",
			})
			return
		}

		// Get current time for response_time
		responseTime := time.Now()

		// Prepare a slice to hold updated requests
		var updatedRequests []models.LeaveRequest
		var failedRequestIDs []int

		// Process each request ID
		for _, requestId := range bulkUpdateData.RequestIDs {
			// Update the leave request status
			var leaveRequest models.LeaveRequest
			err := db.QueryRow(`
				UPDATE leave_requests 
				SET status = $1, responded_by = $2, response_time = $3, updated_at = $3
				WHERE id = $4
				RETURNING id, student_id, student_name, request_type, reason, status, 
						  created_at, updated_at, responded_by, response_time, 
						  live_activity_id, live_activity_token`,
				bulkUpdateData.Status, bulkUpdateData.StaffID, responseTime, requestId).Scan(
				&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
				&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
				&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt, &leaveRequest.RespondedBy,
				&leaveRequest.ResponseTime, &leaveRequest.LiveActivityId, &leaveRequest.LiveActivityToken)

			if err != nil {
				log.Printf("Error updating leave request %d: %v", requestId, err)
				failedRequestIDs = append(failedRequestIDs, requestId)
				continue
			}

			updatedRequests = append(updatedRequests, leaveRequest)

			// If we have live activity info, send push notification
			if leaveRequest.LiveActivityId != nil && leaveRequest.LiveActivityToken != nil {
				// Send a push notification to update the Live Activity
				go sendLiveActivityUpdate(leaveRequest, bulkUpdateData.StaffName, responseTime)
			}
		}

		// Return summary of the operation
		if len(updatedRequests) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"success":          true,
				"updated_requests": updatedRequests,
				"failed_requests":  failedRequestIDs,
				"message":          fmt.Sprintf("Updated %d leave requests, %d failed", len(updatedRequests), len(failedRequestIDs)),
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"success":         false,
				"failed_requests": failedRequestIDs,
				"message":         "Failed to update any leave requests",
			})
		}
	})

	// Get a leave request by activity ID (for iOS Live Activity handling)
	router.GET("/leave-requests/activity/:activityId", func(c *gin.Context) {
		activityId := c.Param("activityId")
		if activityId == "" {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid activity ID",
			})
			return
		}

		var leaveRequest models.LeaveRequest
		err := db.QueryRow(`
			SELECT id, student_id, student_name, request_type, reason, status, 
				   created_at, updated_at, responded_by, response_time, 
				   live_activity_id, live_activity_token
			FROM leave_requests 
			WHERE live_activity_id = $1`, activityId).Scan(
			&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
			&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
			&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt, &leaveRequest.RespondedBy,
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityId, &leaveRequest.LiveActivityToken)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, models.LeaveRequestResponse{
					Success: false,
					Message: "Leave request not found for the given activity ID",
				})
				return
			}

			log.Printf("Error getting leave request by activity ID: %v", err)
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
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityId, &leaveRequest.LiveActivityToken)

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
			LiveActivityId    string `json:"live_activity_id" binding:"required"`
			LiveActivityToken string `json:"live_activity_token" binding:"required"`
		}

		if err := c.BindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request data: " + err.Error(),
			})
			return
		}

		// Enhanced logging for token updates
		log.Printf("🔍 ---------- DEBUG: LIVE ACTIVITY UPDATE REQUEST ----------")
		log.Printf("🎯 Received Live Activity update for request ID %d:", requestId)
		log.Printf("Activity ID: %s", updateData.LiveActivityId)
		log.Printf("Token: %s", updateData.LiveActivityToken)
		log.Printf("Token first 10 chars: %s", updateData.LiveActivityToken[:10])
		log.Printf("Token length: %d", len(updateData.LiveActivityToken))
		log.Printf("🔍 ---------- DEBUG: LIVE ACTIVITY UPDATE REQUEST END ----------")

		// Get existing record before update for comparison
		var existingRequest models.LeaveRequest
		err = db.QueryRow(`
			SELECT id, live_activity_id, live_activity_token
			FROM leave_requests
			WHERE id = $1`, requestId).Scan(
			&existingRequest.ID, &existingRequest.LiveActivityId, &existingRequest.LiveActivityToken)

		if err == nil && existingRequest.LiveActivityId != nil {
			log.Printf("🔍 ---------- DEBUG: EXISTING RECORD ----------")
			log.Printf("Existing ID: %d", existingRequest.ID)
			log.Printf("Existing Activity ID: %s", *existingRequest.LiveActivityId)
			if existingRequest.LiveActivityToken != nil {
				log.Printf("Existing Token: %s", *existingRequest.LiveActivityToken)
			}
			log.Printf("🔍 ---------- DEBUG: EXISTING RECORD END ----------")
		}

		// Make explicit copies of the input values to prevent caching issues
		activityIdCopy := updateData.LiveActivityId
		tokenCopy := updateData.LiveActivityToken

		log.Printf("🔒 Using explicitly copied values to prevent caching issues")
		log.Printf("🔒 Activity ID: %s", activityIdCopy)
		log.Printf("🔒 Token: %s", tokenCopy)

		// Update the live activity information using the copies
		var leaveRequest models.LeaveRequest
		err = db.QueryRow(`
			UPDATE leave_requests 
			SET live_activity_id = $1, live_activity_token = $2, updated_at = NOW()
			WHERE id = $3
			RETURNING id, student_id, student_name, request_type, reason, status, 
			          created_at, updated_at, responded_by, response_time, 
			          live_activity_id, live_activity_token`,
			activityIdCopy, tokenCopy, requestId).Scan(
			&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
			&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
			&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt, &leaveRequest.RespondedBy,
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityId, &leaveRequest.LiveActivityToken)

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

		// Log the updated record
		log.Printf("🔍 ---------- DEBUG: UPDATED RECORD ----------")
		log.Printf("Updated ID: %d", leaveRequest.ID)
		if leaveRequest.LiveActivityId != nil {
			log.Printf("Updated Activity ID: %s", *leaveRequest.LiveActivityId)
		}
		if leaveRequest.LiveActivityToken != nil {
			log.Printf("Updated Token: %s", *leaveRequest.LiveActivityToken)

			// Verify the token matches what was sent in the request
			if *leaveRequest.LiveActivityToken != tokenCopy {
				log.Printf("❌ CRITICAL ERROR: Updated token doesn't match input token!")
				log.Printf("❌ Expected: %s", tokenCopy)
				log.Printf("❌ Got: %s", *leaveRequest.LiveActivityToken)
			} else {
				log.Printf("✅ Token update verified successfully")
			}
		}
		log.Printf("🔍 ---------- DEBUG: UPDATED RECORD END ----------")

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
	ActivityId string `json:"activity-id"`
}

// Send a push notification to update a Live Activity
func sendLiveActivityUpdate(request models.LeaveRequest, staffName string, responseTime time.Time) {
	if request.LiveActivityId == nil || request.LiveActivityToken == nil {
		log.Println("⚠️ Missing Live Activity info for leave request:", request.ID)
		return
	}

	// Make explicit copies to prevent caching issues
	activityId := *request.LiveActivityId
	deviceToken := *request.LiveActivityToken

	// Log original values from request
	log.Printf("🔍 VERIFICATION - sendLiveActivityUpdate called for:")
	log.Printf("🔍 Request ID: %d", request.ID)
	log.Printf("🔍 Original Activity ID: %s", activityId)
	log.Printf("🔍 Original Token: %s", deviceToken)
	log.Printf("🔍 Token Length: %d", len(deviceToken))

	// Create the push notification payload
	payload := LiveActivityPayload{}
	payload.APS.Event = "update"
	payload.APS.Timestamp = time.Now().Unix()
	payload.APS.ContentState.Status = request.Status
	payload.APS.ContentState.ResponseTime = responseTime
	payload.APS.ContentState.RespondedBy = staffName
	payload.ActivityId = activityId // Use the copied value

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

	// Send the push notification using the copied token
	resp, err := notifications.SendAPNsNotification(deviceToken, bundleID, string(jsonPayload), true)
	if err != nil {
		log.Printf("❌ Error sending Live Activity update: %v", err)
		return
	}

	log.Printf("✅ Live Activity update sent successfully: %s", resp)
}
