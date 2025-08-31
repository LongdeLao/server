package routes

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server/models"

	"github.com/gin-gonic/gin"
)

// GetStudentAttendanceStatusForDate returns the attendance status for students in a year group on a specific date
//
// Endpoint: GET /api/attendance/status/:yearGroupId?date=YYYY-MM-DD
//
// Parameters:
//   - yearGroupId: The year group ID (string, e.g., "pib-a")
//   - date: Optional query parameter for date (defaults to today)
//
// Returns:
//   - 200 OK: Successfully retrieved attendance status
//     {
//     "success": true,
//     "yearGroup": {...},
//     "date": "2024-01-15",
//     "students": [
//     {
//     "user_id": 1,
//     "name": "John Doe",
//     "year": "PIB",
//     "group_name": "A",
//     "current_status": "present|absent|late|medical|early|pending",
//     "arrived_at": "08:30:00" // only if late and has arrived
//     }
//     ]
//     }
func GetStudentAttendanceStatusForDate(c *gin.Context, db *sql.DB) {
	yearGroupID := c.Param("yearGroupId")
	dateParam := c.Query("date")

	// Default to today if no date provided
	if dateParam == "" {
		dateParam = time.Now().Format("2006-01-02")
	}

	// Validate date format
	_, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid date format. Use YYYY-MM-DD",
		})
		return
	}

	// Convert ID to YearGroup
	yearGroup, exists := models.GetYearGroupByID(yearGroupID)
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid year group ID",
		})
		return
	}

	// Get all students in this year group with their attendance status for the specified date
	query := `
		SELECT 
			u.id,
			u.name,
			$1 as year,
			$2 as group_name,
			COALESCE(ah.status, 'pending') as current_status,
			ah.arrived_at
		FROM users u
		WHERE u.role = 'student' 
			AND u.id IN (
				SELECT user_id FROM attendance 
				WHERE year = $1 AND group_name = $2
			)
		LEFT JOIN attendance_history ah ON u.id = ah.student_id 
			AND ah.attendance_date = $3
		ORDER BY u.name
	`

	rows, err := db.Query(query, yearGroup.Year, yearGroup.Section, dateParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying attendance status: %v", err),
		})
		return
	}
	defer rows.Close()

	var students []models.StudentAttendanceStatus
	for rows.Next() {
		var student models.StudentAttendanceStatus
		var arrivedAt sql.NullString

		err := rows.Scan(
			&student.UserID,
			&student.Name,
			&student.Year,
			&student.GroupName,
			&student.CurrentStatus,
			&arrivedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning student data: %v", err),
			})
			return
		}

		if arrivedAt.Valid {
			student.ArrivedAt = &arrivedAt.String
		}

		students = append(students, student)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error iterating through students: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"yearGroup": yearGroup,
		"date":      dateParam,
		"students":  students,
	})
}

// UpdateStudentAttendance updates attendance status for students
//
// Endpoint: POST /api/attendance/update
//
// Request Body:
//
//	{
//	  "date": "2024-01-15", // optional, defaults to today
//	  "updates": [
//	    {
//	      "student_id": 1,
//	      "status": "present|absent|late|medical|early"
//	    }
//	  ]
//	}
//
// Returns:
//   - 200 OK: Successfully updated attendance
func UpdateStudentAttendance(c *gin.Context, db *sql.DB) {
	var request struct {
		Date    string `json:"date"`
		Updates []struct {
			StudentID int    `json:"student_id"`
			Status    string `json:"status"`
		} `json:"updates"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request format: %v", err),
		})
		return
	}

	// Default to today if no date provided
	if request.Date == "" {
		request.Date = time.Now().Format("2006-01-02")
	}

	// Validate date format
	_, err := time.Parse("2006-01-02", request.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid date format. Use YYYY-MM-DD",
		})
		return
	}

	// Validate status values
	validStatuses := map[string]bool{
		"present": true,
		"absent":  true,
		"late":    true,
		"medical": true,
		"early":   true,
	}

	for _, update := range request.Updates {
		if !validStatuses[update.Status] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid status '%s' for student %d", update.Status, update.StudentID),
			})
			return
		}
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error starting transaction: %v", err),
		})
		return
	}

	updatedCount := 0

	for _, update := range request.Updates {
		// Check if record already exists for this student and date
		var existingID int
		err = tx.QueryRow(`
				SELECT id FROM attendance_history 
				WHERE student_id = $1 AND attendance_date = $2
		`, update.StudentID, request.Date).Scan(&existingID)

		if err != nil && err != sql.ErrNoRows {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error checking existing record: %v", err),
			})
			return
		}

		if err == sql.ErrNoRows {
			// Insert new record
			_, err = tx.Exec(`
				INSERT INTO attendance_history (student_id, status, attendance_date, created_at)
				VALUES ($1, $2, $3, $4)
			`, update.StudentID, update.Status, request.Date, time.Now())

			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("Error inserting attendance record: %v", err),
				})
				return
			}
		} else {
			// Update existing record
			_, err = tx.Exec(`
						UPDATE attendance_history 
						SET status = $1, arrived_at = NULL
						WHERE id = $2
			`, update.Status, existingID)

			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("Error updating attendance record: %v", err),
				})
				return
			}
		}

		updatedCount++
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error committing transaction: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Attendance updated successfully",
		"updatedCount": updatedCount,
		"date":         request.Date,
	})
}

// MarkStudentArrival marks a late student as arrived
//
// Endpoint: POST /api/attendance/mark-arrival
//
// Request Body:
//
//	{
//	  "student_id": 1,
//	  "date": "2024-01-15" // optional, defaults to today
//	}
//
// Returns:
//   - 200 OK: Successfully marked arrival
func MarkStudentArrival(c *gin.Context, db *sql.DB) {
	var request struct {
		StudentID int    `json:"student_id"`
		Date      string `json:"date"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request format: %v", err),
		})
		return
	}

	// Default to today if no date provided
	if request.Date == "" {
		request.Date = time.Now().Format("2006-01-02")
	}

	// Validate date format
	_, err := time.Parse("2006-01-02", request.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid date format. Use YYYY-MM-DD",
		})
		return
	}

	// Check if student has a late record for this date
	var recordID int
	var currentStatus string
	err = db.QueryRow(`
		SELECT id, status FROM attendance_history
		WHERE student_id = $1 AND attendance_date = $2
	`, request.StudentID, request.Date).Scan(&recordID, &currentStatus)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "No attendance record found for this student on this date",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking attendance record: %v", err),
		})
		return
	}

	if currentStatus != "late" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Student is not marked as late (current status: %s)", currentStatus),
		})
		return
	}

	// Update with arrival time
	arrivedTime := time.Now().Format("15:04:05")
	_, err = db.Exec(`
		UPDATE attendance_history 
		SET arrived_at = $1
		WHERE id = $2
	`, arrivedTime, recordID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error updating arrival time: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Arrival time recorded successfully",
		"student_id": request.StudentID,
		"arrived_at": arrivedTime,
		"date":       request.Date,
	})
}

// GetStudentAttendanceHistory retrieves attendance history for a student
//
// Endpoint: GET /api/attendance/history/:studentId
//
// Returns:
//   - 200 OK: Successfully retrieved attendance history
func GetStudentAttendanceHistory(c *gin.Context, db *sql.DB) {
	studentIDStr := c.Param("studentId")

	studentID, err := strconv.Atoi(studentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid student ID format",
		})
		return
	}

	// Get student info
	var student models.StudentAttendanceDetails
	err = db.QueryRow(`
		SELECT id, name FROM users WHERE id = $1 AND role = 'student'
	`, studentID).Scan(&student.UserID, &student.Name)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Student not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error retrieving student info: %v", err),
		})
		return
	}

	// Get attendance history
	rows, err := db.Query(`
		SELECT id, student_id, status, attendance_date, arrived_at, created_at
		FROM attendance_history
		WHERE student_id = $1
		ORDER BY attendance_date DESC
		LIMIT 50
	`, studentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error retrieving attendance history: %v", err),
		})
		return
	}
	defer rows.Close()

	var history []models.AttendanceHistory
	stats := models.AttendanceStats{}

	for rows.Next() {
		var record models.AttendanceHistory
		var arrivedAt sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.StudentID,
			&record.Status,
			&record.AttendanceDate,
			&arrivedAt,
			&record.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning attendance record: %v", err),
			})
			return
		}

		if arrivedAt.Valid {
			record.ArrivedAt = &arrivedAt.String
		}

		// Count statistics
		switch record.Status {
		case "present":
			stats.Present++
		case "absent":
			stats.Absent++
		case "late":
			stats.Late++
		case "medical":
			stats.Medical++
		case "early":
			stats.Early++
		}

		history = append(history, record)
	}

	// Calculate statistics
	stats.Total = stats.Present + stats.Absent + stats.Late + stats.Medical + stats.Early
	if stats.Total > 0 {
		attendedDays := stats.Present + stats.Late
		stats.Percentage = (float64(attendedDays) / float64(stats.Total)) * 100
	}

	student.History = history
	student.Stats = stats

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"student": student,
	})
}

// GetYearGroups returns all year groups with today's attendance summary
//
// Endpoint: GET /api/attendance/year-groups
func GetYearGroups(c *gin.Context, db *sql.DB) {
	yearGroups := models.GenerateYearGroups()
	today := time.Now().Format("2006-01-02")

	type YearGroupSummary struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Year           string `json:"year"`
		Section        string `json:"section"`
		TotalStudents  int    `json:"total_students"`
		PresentToday   int    `json:"present_today"`
		LateToday      int    `json:"late_today"`
		AbsentToday    int    `json:"absent_today"`
		MedicalToday   int    `json:"medical_today"`
		EarlyToday     int    `json:"early_today"`
		PendingToday   int    `json:"pending_today"`
		AttendanceRate string `json:"attendance_rate"`
	}

	var summaries []YearGroupSummary

	for _, group := range yearGroups {
		id := strings.ToLower(group.Year + "-" + group.Section)

		// Get total students in this year group
		var totalStudents int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM users u
			JOIN attendance a ON u.id = a.user_id
			WHERE u.role = 'student' AND a.year = $1 AND a.group_name = $2
		`, group.Year, group.Section).Scan(&totalStudents)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error getting student count: %v", err),
			})
			return
		}

		// Get today's attendance counts
		var present, late, absent, medical, early int
		rows, err := db.Query(`
			SELECT 
				COALESCE(ah.status, 'pending') as status,
				COUNT(*) as count
			FROM users u
			JOIN attendance a ON u.id = a.user_id
			LEFT JOIN attendance_history ah ON u.id = ah.student_id AND ah.attendance_date = $3
			WHERE u.role = 'student' AND a.year = $1 AND a.group_name = $2
			GROUP BY COALESCE(ah.status, 'pending')
		`, group.Year, group.Section, today)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error getting attendance counts: %v", err),
			})
			return
		}
		defer rows.Close()

		pending := totalStudents // Start with all students as pending

		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("Error scanning attendance counts: %v", err),
				})
				return
			}

			switch status {
			case "present":
				present = count
				pending -= count
			case "late":
				late = count
				pending -= count
			case "absent":
				absent = count
				pending -= count
			case "medical":
				medical = count
				pending -= count
			case "early":
				early = count
				pending -= count
			}
		}

		// Calculate attendance rate (present + late / total)
		var attendanceRate string
		if totalStudents > 0 {
			rate := float64(present+late) / float64(totalStudents) * 100
			attendanceRate = fmt.Sprintf("%.1f%%", rate)
		} else {
			attendanceRate = "0%"
		}

		summaries = append(summaries, YearGroupSummary{
			ID:             id,
			Name:           group.FullName,
			Year:           group.Year,
			Section:        group.Section,
			TotalStudents:  totalStudents,
			PresentToday:   present,
			LateToday:      late,
			AbsentToday:    absent,
			MedicalToday:   medical,
			EarlyToday:     early,
			PendingToday:   pending,
			AttendanceRate: attendanceRate,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"yearGroups": summaries,
		"date":       today,
	})
}

// SetupAttendanceRoutes sets up the new attendance routes
func SetupAttendanceRoutes(router gin.IRouter, db *sql.DB) {
	attendanceGroup := router.Group("/attendance")
	{
		// Get year groups with attendance summary
		attendanceGroup.GET("/year-groups", func(c *gin.Context) {
			GetYearGroups(c, db)
		})

		// Get attendance status for students in a year group on a specific date
		attendanceGroup.GET("/status/:yearGroupId", func(c *gin.Context) {
			GetStudentAttendanceStatusForDate(c, db)
		})

		// Update attendance for multiple students
		attendanceGroup.POST("/update", func(c *gin.Context) {
			UpdateStudentAttendance(c, db)
		})

		// Mark late student as arrived
		attendanceGroup.POST("/mark-arrival", func(c *gin.Context) {
			MarkStudentArrival(c, db)
		})

		// Get attendance history for a student
		attendanceGroup.GET("/history/:studentId", func(c *gin.Context) {
			GetStudentAttendanceHistory(c, db)
		})
	}
}
