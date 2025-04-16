package routes

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
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
		// Reset everything at the beginning of the function to prevent cross-request caching
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

		// ANTI-CACHE: Print the raw request body for validation
		rawData, _ := c.GetRawData()
		log.Printf("📋 RAW REQUEST BODY: %s", string(rawData))

		log.Printf("🚫 CACHE RESET: Ensuring fresh values for this request")
		log.Printf("Creating leave request for student %s (ID: %d)", requestData.StudentName, requestData.StudentID)
		log.Printf("Request type: %s", requestData.RequestType)
		if requestData.Reason != nil {
			log.Printf("Reason: %s", *requestData.Reason)
		}

		// Direct database insertion with no intermediate variables
		var leaveRequest models.LeaveRequest
		var err error

		if requestData.LiveActivityId != nil && requestData.LiveActivityToken != nil {
			// Log incoming token for verification
			log.Printf("🛑 DIRECT INSERT: Using token directly from request")
			log.Printf("📱 Activity ID from request: %s", *requestData.LiveActivityId)
			log.Printf("🔑 Token from request: %s", *requestData.LiveActivityToken)

			// Simplified insertion with direct parameter passing
			err = db.QueryRow(`
				INSERT INTO leave_requests 
				(student_id, student_name, request_type, reason, status, live_activity_id, live_activity_token) 
				VALUES ($1, $2, $3, $4, 'pending', $5, $6) 
				RETURNING id, student_id, student_name, request_type, reason, status, created_at, updated_at, 
						  live_activity_id, live_activity_token`,
				requestData.StudentID,
				requestData.StudentName,
				requestData.RequestType,
				requestData.Reason,
				*requestData.LiveActivityId,    // Direct use
				*requestData.LiveActivityToken, // Direct use - added comma here
			).Scan(
				&leaveRequest.ID,
				&leaveRequest.StudentID,
				&leaveRequest.StudentName,
				&leaveRequest.RequestType,
				&leaveRequest.Reason,
				&leaveRequest.Status,
				&leaveRequest.CreatedAt,
				&leaveRequest.UpdatedAt,
				&leaveRequest.LiveActivityId,
				&leaveRequest.LiveActivityToken,
			)
		} else {
			// Handle request without live activity info
			log.Printf("ℹ️ No live activity info in request")
			err = db.QueryRow(`
				INSERT INTO leave_requests 
				(student_id, student_name, request_type, reason, status) 
				VALUES ($1, $2, $3, $4, 'pending') 
				RETURNING id, student_id, student_name, request_type, reason, status, created_at, updated_at`,
				requestData.StudentID,
				requestData.StudentName,
				requestData.RequestType,
				requestData.Reason,
			).Scan(
				&leaveRequest.ID,
				&leaveRequest.StudentID,
				&leaveRequest.StudentName,
				&leaveRequest.RequestType,
				&leaveRequest.Reason,
				&leaveRequest.Status,
				&leaveRequest.CreatedAt,
				&leaveRequest.UpdatedAt,
			)
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

		// Verify what was actually saved
		if leaveRequest.LiveActivityId != nil && leaveRequest.LiveActivityToken != nil {
			log.Printf("🔍 VERIFICATION - SAVED VALUES:")
			log.Printf("📱 Saved Activity ID: %s", *leaveRequest.LiveActivityId)
			log.Printf("🔑 Saved Token: %s", *leaveRequest.LiveActivityToken)

			// Double-check against what was in the request
			if requestData.LiveActivityToken != nil {
				if *leaveRequest.LiveActivityToken != *requestData.LiveActivityToken {
					log.Printf("❌ CRITICAL ERROR: Token mismatch between request and database!")
					log.Printf("❌ Request token: %s", *requestData.LiveActivityToken)
					log.Printf("❌ Saved token: %s", *leaveRequest.LiveActivityToken)

					// Immediately fix with a direct update
					log.Printf("🔄 Attempting immediate fix with direct update...")
					_, updateErr := db.Exec(`
						UPDATE leave_requests 
						SET live_activity_token = $1
						WHERE id = $2`,
						*requestData.LiveActivityToken,
						leaveRequest.ID)

					if updateErr != nil {
						log.Printf("❌ Fix failed: %v", updateErr)
					} else {
						log.Printf("✅ Token directly corrected in database")
						// Replace the token in returned object too
						tokenCopy := *requestData.LiveActivityToken
						leaveRequest.LiveActivityToken = &tokenCopy
					}
				} else {
					log.Printf("✅ Token verification successful - saved token matches request token")
				}
			}
		}

		// Return the leave request
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
				activityId := *updatedRequest.LiveActivityId
				deviceToken := *updatedRequest.LiveActivityToken

				log.Printf("🔄 Sending cancellation notification for request %d", updatedRequest.ID)
				log.Printf("🔄 Activity ID: %s", activityId)
				log.Printf("🔄 Token: %s", deviceToken)

				// Use our exact shell script implementation
				resp, err := notifications.SendAPNsNotificationExact(
					deviceToken,
					activityId,
					"cancelled", // Always "cancelled" for this endpoint
					"Student",   // Always "Student" for cancellations
				)

				if err != nil {
					log.Printf("❌ Error sending cancellation notification: %v", err)

					// Try fallback approach
					log.Printf("⚠️ Trying fallback cancellation approach...")

					// Create the exact payload structure
					timestamp := time.Now().Unix()

					// Create content state with just status for cancelled
					contentState := map[string]interface{}{
						"status":       "cancelled",
						"responseTime": cancellationTime,
						"respondedBy":  "Student",
					}

					// Build the exact same structure as the shell script
					payload := map[string]interface{}{
						"aps": map[string]interface{}{
							"event":         "update",
							"timestamp":     timestamp,
							"content-state": contentState,
						},
						"activity-id": activityId,
					}

					// Convert payload to JSON
					jsonPayload, err := json.Marshal(payload)
					if err != nil {
						log.Printf("Error marshalling Live Activity payload: %v", err)
						return
					}

					log.Printf("📱 Fallback - Sending Live Activity update for cancelled request %d", updatedRequest.ID)
					log.Printf("Payload: %s", jsonPayload)

					// The bundle ID for Live Activities needs .push-type.liveactivity appended
					bundleID := "com.leo.hsannu.push-type.liveactivity"

					// ENHANCED LOGGING: Log all details about the notification
					log.Printf("📲 APNS CANCELLATION DETAILS:")
					log.Printf("Token: %s", deviceToken)
					log.Printf("Bundle ID: %s", bundleID)
					log.Printf("Push Type: liveactivity")
					log.Printf("Activity ID: %s", activityId)

					// Send the push notification
					resp, err = notifications.SendAPNsNotification(deviceToken, bundleID, string(jsonPayload), true)
					if err != nil {
						log.Printf("❌ Error sending Live Activity update (fallback): %v", err)
						return
					}
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

		// ANTI-CACHE: Print the raw request body for validation
		rawData, _ := c.GetRawData()
		log.Printf("📋 RAW UPDATE REQUEST BODY: %s", string(rawData))

		// Need to re-bind after reading raw data
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawData))

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

		log.Printf("🚫 CACHE RESET: Ensuring fresh values for live activity update")
		log.Printf("🎯 Updating Live Activity for request ID %d:", requestId)
		log.Printf("📱 Activity ID from request: %s", updateData.LiveActivityId)
		log.Printf("🔑 Token from request: %s", updateData.LiveActivityToken)

		// Direct update with no intermediate variables
		var leaveRequest models.LeaveRequest
		err = db.QueryRow(`
			UPDATE leave_requests 
			SET live_activity_id = $1, live_activity_token = $2, updated_at = NOW()
			WHERE id = $3
			RETURNING id, student_id, student_name, request_type, reason, status, 
					  created_at, updated_at, responded_by, response_time, 
					  live_activity_id, live_activity_token`,
			updateData.LiveActivityId,
			updateData.LiveActivityToken,
			requestId,
		).Scan(
			&leaveRequest.ID, &leaveRequest.StudentID, &leaveRequest.StudentName,
			&leaveRequest.RequestType, &leaveRequest.Reason, &leaveRequest.Status,
			&leaveRequest.CreatedAt, &leaveRequest.UpdatedAt, &leaveRequest.RespondedBy,
			&leaveRequest.ResponseTime, &leaveRequest.LiveActivityId, &leaveRequest.LiveActivityToken,
		)

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

		// Verify the saved data
		log.Printf("🔍 VERIFICATION - UPDATED VALUES:")
		if leaveRequest.LiveActivityId != nil {
			log.Printf("📱 Updated Activity ID: %s", *leaveRequest.LiveActivityId)
		}
		if leaveRequest.LiveActivityToken != nil {
			log.Printf("🔑 Updated Token: %s", *leaveRequest.LiveActivityToken)

			// Verify the update was successful
			if *leaveRequest.LiveActivityToken != updateData.LiveActivityToken {
				log.Printf("❌ CRITICAL ERROR: Token mismatch after update!")
				log.Printf("❌ Request token: %s", updateData.LiveActivityToken)
				log.Printf("❌ Saved token: %s", *leaveRequest.LiveActivityToken)

				// Fix with a direct update
				log.Printf("🔄 Forcing direct update...")
				_, fixErr := db.Exec(`
					UPDATE leave_requests 
					SET live_activity_token = $1
					WHERE id = $2`,
					updateData.LiveActivityToken,
					requestId)

				if fixErr != nil {
					log.Printf("❌ Fix failed: %v", fixErr)
				} else {
					log.Printf("✅ Token directly corrected in database")
					// Update the returned object
					tokenCopy := updateData.LiveActivityToken
					leaveRequest.LiveActivityToken = &tokenCopy
				}
			} else {
				log.Printf("✅ Token update verified")
			}
		}

		// Return the updated leave request
		c.JSON(http.StatusOK, models.LeaveRequestResponse{
			Success: true,
			Request: &leaveRequest,
		})
	})

	// Delete a leave request from the database
	router.DELETE("/leave-requests/:requestId", func(c *gin.Context) {
		requestIdStr := c.Param("requestId")
		requestId, err := strconv.Atoi(requestIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request ID",
			})
			return
		}

		var deleteData struct {
			StudentID int `json:"student_id" binding:"required"`
		}

		if err := c.BindJSON(&deleteData); err != nil {
			c.JSON(http.StatusBadRequest, models.LeaveRequestResponse{
				Success: false,
				Message: "Invalid request data: " + err.Error(),
			})
			return
		}

		// Get the existing request to verify student ID and get a copy before deletion
		var existingRequest models.LeaveRequest
		err = db.QueryRow(`
			SELECT id, student_id, student_name, request_type, reason, status, 
			       created_at, updated_at, responded_by, response_time, 
			       live_activity_id, live_activity_token
			FROM leave_requests
			WHERE id = $1`, requestId).Scan(
			&existingRequest.ID, &existingRequest.StudentID, &existingRequest.StudentName,
			&existingRequest.RequestType, &existingRequest.Reason, &existingRequest.Status,
			&existingRequest.CreatedAt, &existingRequest.UpdatedAt, &existingRequest.RespondedBy,
			&existingRequest.ResponseTime, &existingRequest.LiveActivityId, &existingRequest.LiveActivityToken)

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
		if existingRequest.StudentID != deleteData.StudentID {
			c.JSON(http.StatusForbidden, models.LeaveRequestResponse{
				Success: false,
				Message: "You are not authorized to delete this request",
			})
			return
		}

		// Delete the leave request from the database
		_, err = db.Exec(`DELETE FROM leave_requests WHERE id = $1`, requestId)
		if err != nil {
			log.Printf("Error deleting leave request: %v", err)
			c.JSON(http.StatusInternalServerError, models.LeaveRequestResponse{
				Success: false,
				Message: "Failed to delete leave request: " + err.Error(),
			})
			return
		}

		// Return success response with the deleted request
		c.JSON(http.StatusOK, models.LeaveRequestResponse{
			Success: true,
			Request: &existingRequest,
			Message: "Leave request deleted successfully",
		})
	})
}

// Struct for Live Activity update payload
type LiveActivityPayload struct {
	APS struct {
		Event        string `json:"event"`
		Timestamp    int64  `json:"timestamp"`
		ContentState struct {
			Status       string     `json:"status"`
			ResponseTime *time.Time `json:"responseTime"`
			RespondedBy  string     `json:"respondedBy"`
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
	log.Printf("🔍 Status: %s", request.Status)
	log.Printf("🔍 Staff Name: %s", staffName)

	// Use our new function that exactly mimics the shell script
	// This completely bypasses the old APNS library and uses direct HTTP
	resp, err := notifications.SendAPNsNotificationExact(
		deviceToken,
		activityId,
		request.Status,
		staffName,
	)

	if err != nil {
		log.Printf("❌ Error sending Live Activity update: %v", err)

		// Try the old method as a fallback
		log.Printf("⚠️ Trying fallback method...")

		// Create a payload that EXACTLY matches the shell script structure
		timestamp := time.Now().Unix()

		var contentState map[string]interface{}

		// Handle status-specific payload structure
		if request.Status == "pending" {
			contentState = map[string]interface{}{
				"status": request.Status,
				// Explicitly omit responseTime and respondedBy for pending
			}
		} else {
			// For approved, rejected, etc.
			contentState = map[string]interface{}{
				"status":       request.Status,
				"responseTime": responseTime, // Let the JSON marshaling handle the formatting
				"respondedBy":  staffName,
			}
		}

		// Build the exact same structure as the shell script
		payload := map[string]interface{}{
			"aps": map[string]interface{}{
				"event":         "update",
				"timestamp":     timestamp,
				"content-state": contentState,
			},
			"activity-id": activityId,
		}

		// Convert payload to JSON
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshalling Live Activity payload: %v", err)
			return
		}

		log.Printf("📱 Fallback - Sending Live Activity update for request %d with status %s", request.ID, request.Status)
		log.Printf("Payload: %s", jsonPayload)

		// The bundle ID for Live Activities needs .push-type.liveactivity appended
		bundleID := "com.leo.hsannu.push-type.liveactivity"

		// ENHANCED LOGGING: Log all details about the notification
		log.Printf("📲 APNS DETAILS:")
		log.Printf("Token: %s", deviceToken)
		log.Printf("Bundle ID: %s", bundleID)
		log.Printf("Push Type: liveactivity")
		log.Printf("Activity ID: %s", activityId)

		// Send the push notification using the copied token
		resp, err = notifications.SendAPNsNotification(deviceToken, bundleID, string(jsonPayload), true)
		if err != nil {
			log.Printf("❌ Error sending Live Activity update (fallback): %v", err)
			return
		}
	}

	log.Printf("✅ Live Activity update sent successfully: %s", resp)
}
