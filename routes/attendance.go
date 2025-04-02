package routes

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server/models"

	"github.com/gin-gonic/gin"
)

// GetYearGroups returns all available year groups with attendance statistics from DB
//
// Endpoint: GET /api/attendance/year-groups
//
// Returns:
//   - 200 OK: Successfully retrieved year groups
//     {
//     "success": true,
//     "yearGroups": [
//     {
//     "id": string,      // e.g., "pib-a"
//     "name": string,    // e.g., "PIB A"
//     "year": string,    // e.g., "PIB"
//     "section": string, // e.g., "A"
//     "students": int,   // Number of students in the group
//     "attendance": string // e.g., "95.5%"
//     }
//     ]
//     }
//   - 500 Internal Server Error: Database error
func GetYearGroups(c *gin.Context, db *sql.DB) {
	yearGroups := models.GenerateYearGroups()

	// Format the response with additional information
	type YearGroupResponse struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Year         string `json:"year"`
		Section      string `json:"section"`
		Students     int    `json:"students"`
		Attendance   string `json:"attendance"`
		LateStudents []struct {
			UserID int    `json:"user_id"`
			Name   string `json:"name"`
		} `json:"late_students"`
		AbsentStudents []struct {
			UserID int    `json:"user_id"`
			Name   string `json:"name"`
		} `json:"absent_students"`
		MedicalStudents []struct {
			UserID int    `json:"user_id"`
			Name   string `json:"name"`
		} `json:"medical_students"`
	}

	response := make([]YearGroupResponse, 0, len(yearGroups))

	// For each year group, query the database for student count and attendance stats
	for _, group := range yearGroups {
		id := strings.ToLower(group.Year + "-" + group.Section)

		// Query to get student count for this year group
		var studentCount int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM attendance 
			WHERE year = $1 AND group_name = $2
		`, group.Year, group.Section).Scan(&studentCount)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error getting student count: %v", err),
			})
			return
		}

		// Query to get attendance statistics for this year group
		var totalPresent, totalCount int
		err = db.QueryRow(`
			SELECT COALESCE(SUM(present), 0), 
			       COUNT(*) 
			FROM attendance 
			WHERE year = $1 AND group_name = $2
		`, group.Year, group.Section).Scan(&totalPresent, &totalCount)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error getting attendance stats: %v", err),
			})
			return
		}

		// Calculate attendance percentage
		var attendancePercentage string
		if totalCount > 0 {
			percentage := float64(totalPresent) / float64(totalCount) * 100
			attendancePercentage = fmt.Sprintf("%.1f%%", percentage)
		} else {
			attendancePercentage = "0%"
		}

		// Query to get late, absent, and medical students
		lateStudents := []struct {
			UserID int    `json:"user_id"`
			Name   string `json:"name"`
		}{}
		absentStudents := []struct {
			UserID int    `json:"user_id"`
			Name   string `json:"name"`
		}{}
		medicalStudents := []struct {
			UserID int    `json:"user_id"`
			Name   string `json:"name"`
		}{}

		// Get late students
		lateRows, err := db.Query(`
			SELECT user_id, name 
			FROM attendance 
			WHERE year = $1 AND group_name = $2 AND today = 'Late'
		`, group.Year, group.Section)
		if err == nil {
			defer lateRows.Close()
			for lateRows.Next() {
				var student struct {
					UserID int    `json:"user_id"`
					Name   string `json:"name"`
				}
				if err := lateRows.Scan(&student.UserID, &student.Name); err == nil {
					lateStudents = append(lateStudents, student)
				}
			}
		}

		// Get absent students
		absentRows, err := db.Query(`
			SELECT user_id, name 
			FROM attendance 
			WHERE year = $1 AND group_name = $2 AND today = 'Absent'
		`, group.Year, group.Section)
		if err == nil {
			defer absentRows.Close()
			for absentRows.Next() {
				var student struct {
					UserID int    `json:"user_id"`
					Name   string `json:"name"`
				}
				if err := absentRows.Scan(&student.UserID, &student.Name); err == nil {
					absentStudents = append(absentStudents, student)
				}
			}
		}

		// Get medical students
		medicalRows, err := db.Query(`
			SELECT user_id, name 
			FROM attendance 
			WHERE year = $1 AND group_name = $2 AND today = 'Medical'
		`, group.Year, group.Section)
		if err == nil {
			defer medicalRows.Close()
			for medicalRows.Next() {
				var student struct {
					UserID int    `json:"user_id"`
					Name   string `json:"name"`
				}
				if err := medicalRows.Scan(&student.UserID, &student.Name); err == nil {
					medicalStudents = append(medicalStudents, student)
				}
			}
		}

		response = append(response, YearGroupResponse{
			ID:              id,
			Name:            group.FullName,
			Year:            group.Year,
			Section:         group.Section,
			Students:        studentCount,
			Attendance:      attendancePercentage,
			LateStudents:    lateStudents,
			AbsentStudents:  absentStudents,
			MedicalStudents: medicalStudents,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"yearGroups": response,
	})
}

// GetStudentsByYearGroup returns students for a specific year group from DB
//
// Endpoint: GET /api/attendance/students/:id
//
// Parameters:
//   - id: The year group ID (string, e.g., "pib-a")
//
// Returns:
//   - 200 OK: Successfully retrieved students
//     {
//     "success": true,
//     "yearGroup": {
//     "year": string,
//     "section": string,
//     "fullName": string
//     },
//     "students": [
//     {
//     "user_id": int,
//     "name": string,
//     "year": string,
//     "group_name": string,
//     "today": string,
//     "present": int,
//     "absent": int,
//     "late": int,
//     "medical": int,
//     "early": int
//     }
//     ],
//     "date": string // Current date in YYYY-MM-DD format
//     }
//   - 400 Bad Request: Invalid year group ID
//   - 500 Internal Server Error: Database error
func GetStudentsByYearGroup(c *gin.Context, db *sql.DB) {
	yearGroupID := c.Param("id")

	// Convert ID to YearGroup
	yearGroup, exists := models.GetYearGroupByID(yearGroupID)
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid year group ID",
		})
		return
	}

	// Query the database for students in this year group
	rows, err := db.Query(`
		SELECT user_id, name, year, group_name, today, present, absent, late, medical, early 
		FROM attendance 
		WHERE year = $1 AND group_name = $2
	`, yearGroup.Year, yearGroup.Section)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying students: %v", err),
		})
		return
	}
	defer rows.Close()

	// Parse the query results
	var students []models.Student
	for rows.Next() {
		var student models.Student
		if err := rows.Scan(
			&student.UserID,
			&student.Name,
			&student.Year,
			&student.GroupName,
			&student.Today,
			&student.Present,
			&student.Absent,
			&student.Late,
			&student.Medical,
			&student.Early,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning student data: %v", err),
			})
			return
		}
		students = append(students, student)
	}

	// Check for errors during iteration
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
		"students":  students,
		"date":      time.Now().Format("2006-01-02"),
	})
}

// UpdateAttendance updates the attendance status for students in DB
//
// Endpoint: POST /api/attendance/update
//
// Request Body:
//
//	{
//	  "yearGroupId": string,  // e.g., "pib-a"
//	  "date": string,         // YYYY-MM-DD format
//	  "students": [
//	    {
//	      "user_id": int,
//	      "status": string    // "Present", "Absent", "Late", "Medical", "Early", or "Pending"
//	    }
//	  ]
//	}
//
// Returns:
//   - 200 OK: Successfully updated attendance
//     {
//     "success": true,
//     "message": "Attendance updated successfully",
//     "updatedCount": int
//     }
//   - 400 Bad Request: Invalid request format or data
//   - 500 Internal Server Error: Database error
func UpdateAttendance(c *gin.Context, db *sql.DB) {
	var request struct {
		YearGroupID string `json:"yearGroupId"`
		Date        string `json:"date"`
		Students    []struct {
			UserID int    `json:"user_id"`
			Status string `json:"status"`
		} `json:"students"`
	}

	// Read the raw body first for debugging
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error reading request body: %v", err),
		})
		return
	}

	// Log the raw request body
	fmt.Printf("Raw request body: %s\n", string(bodyBytes))

	// Create a new reader from the body bytes
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Try to bind the JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		fmt.Printf("Error binding JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request format: %v", err),
		})
		return
	}

	// Validate the request
	if len(request.Students) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No students provided in the request",
		})
		return
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error starting transaction: %v", err),
		})
		return
	}

	// Update each student's attendance status
	for _, student := range request.Students {
		// Validate the user ID
		if student.UserID <= 0 {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid user ID: %d", student.UserID),
			})
			return
		}

		// Validate the status - allow empty status or "Pending" for resetting
		if student.Status != "" &&
			student.Status != "Present" &&
			student.Status != "Absent" &&
			student.Status != "Late" &&
			student.Status != "Medical" &&
			student.Status != "Early" &&
			student.Status != "Pending" {
			// Invalid status provided
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid status '%s' for user ID %d", student.Status, student.UserID),
			})
			return
		}

		// Update the today field - always set "Pending" for empty values, never NULL
		result, err := tx.Exec(`
			UPDATE attendance 
			SET today = CASE WHEN $1 = '' OR $1 = 'Pending' THEN 'Pending' ELSE $1 END
			WHERE user_id = $2
		`, student.Status, student.UserID)

		if err != nil {
			tx.Rollback()
			fmt.Printf("Error updating attendance for user ID %d: %v\n", student.UserID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error updating attendance for user ID %d: %v", student.UserID, err),
			})
			return
		}

		// Check if any rows were affected
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			fmt.Printf("Warning: No rows updated for user ID %d\n", student.UserID)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		fmt.Printf("Error committing transaction: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error committing transaction: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Attendance updated successfully",
		"updatedCount": len(request.Students),
	})
}

// GetStudentAttendance returns attendance records for a specific student
//
// Endpoint: GET /api/attendance/student/:id
//
// Parameters:
//   - id: The student's user ID (integer)
//
// Returns:
//   - 200 OK: Successfully retrieved attendance records
//     {
//     "success": true,
//     "student": {
//     "user_id": int,
//     "name": string,
//     "year": string,
//     "group_name": string,
//     "today": string, // "Present", "Absent", "Late", "Medical", "Early", or "Pending"
//     "stats": {
//     "present": int,
//     "absent": int,
//     "late": int,
//     "medical": int,
//     "early": int,
//     "total": int,
//     "percentage": string // e.g., "95.5%"
//     }
//     }
//     }
//   - 400 Bad Request: Invalid student ID format
//   - 404 Not Found: No attendance records found for the student
//   - 500 Internal Server Error: Database error
func GetStudentAttendance(c *gin.Context, db *sql.DB) {
	studentIDStr := c.Param("id")
	fmt.Printf("Received request for student ID: %s\n", studentIDStr)

	if studentIDStr == "" {
		fmt.Println("Error: Student ID is empty")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Student ID is required",
		})
		return
	}

	// Convert student ID from string to integer
	studentID, err := strconv.Atoi(studentIDStr)
	if err != nil {
		fmt.Printf("Error converting student ID to integer: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid student ID format: %s", studentIDStr),
		})
		return
	}

	// Query to get attendance records for the student
	query := `
		SELECT 
			user_id,
			name,
			year,
			group_name,
			today,
			present,
			absent,
			late,
			medical,
			early
		FROM attendance 
		WHERE user_id = $1;
	`
	fmt.Printf("Executing query: %s with student ID: %d\n", query, studentID)

	var record struct {
		UserID    int    `json:"user_id"`
		Name      string `json:"name"`
		Year      string `json:"year"`
		GroupName string `json:"group_name"`
		Today     string `json:"today"`
		Present   int    `json:"present"`
		Absent    int    `json:"absent"`
		Late      int    `json:"late"`
		Medical   int    `json:"medical"`
		Early     int    `json:"early"`
	}

	// First, check if the student exists in the attendance table
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM attendance WHERE user_id = $1", studentID).Scan(&count)
	if err != nil {
		fmt.Printf("Error checking student existence: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking student existence: %v", err),
		})
		return
	}
	fmt.Printf("Found %d records for student ID %d\n", count, studentID)

	if count == 0 {
		fmt.Printf("No records found for student ID: %d\n", studentID)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("No attendance records found for student ID: %d", studentID),
		})
		return
	}

	err = db.QueryRow(query, studentID).Scan(
		&record.UserID,
		&record.Name,
		&record.Year,
		&record.GroupName,
		&record.Today,
		&record.Present,
		&record.Absent,
		&record.Late,
		&record.Medical,
		&record.Early,
	)

	if err != nil {
		fmt.Printf("Error scanning attendance record: %v\n", err)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": fmt.Sprintf("Student not found with ID: %d", studentID),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying attendance: %v", err),
		})
		return
	}

	fmt.Printf("Successfully retrieved attendance record for student: %s (ID: %d)\n", record.Name, record.UserID)

	// Calculate attendance statistics
	totalClasses := record.Present + record.Absent + record.Late + record.Medical + record.Early
	var attendancePercentage float64
	if totalClasses > 0 {
		attendancePercentage = float64(record.Present) / float64(totalClasses) * 100
	}

	response := gin.H{
		"success": true,
		"student": gin.H{
			"user_id":    record.UserID,
			"name":       record.Name,
			"year":       record.Year,
			"group_name": record.GroupName,
			"today":      record.Today,
			"stats": gin.H{
				"present":    record.Present,
				"absent":     record.Absent,
				"late":       record.Late,
				"medical":    record.Medical,
				"early":      record.Early,
				"total":      totalClasses,
				"percentage": fmt.Sprintf("%.1f%%", attendancePercentage),
			},
		},
	}

	fmt.Printf("Sending response for student %s: %+v\n", record.Name, response)
	c.JSON(http.StatusOK, response)
}

// GetAllAttendance returns all attendance records
//
// Endpoint: GET /api/attendance/all
//
// Returns:
//   - 200 OK: Successfully retrieved all attendance records
//     {
//     "success": true,
//     "data": [
//     {
//     "user_id": int,
//     "name": string,
//     "year": string,
//     "group_name": string,
//     "today": string,
//     "present": int,
//     "absent": int,
//     "late": int,
//     "medical": int
//     }
//     ]
//     }
//   - 500 Internal Server Error: Database error
func GetAllAttendance(c *gin.Context, db *sql.DB) {
	query := `
		SELECT 
			user_id,
			name,
			year,
			group_name,
			today,
			present,
			absent,
			late,
			medical
		FROM attendance
		ORDER BY year, group_name, name
	`

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying attendance data: %v", err),
		})
		return
	}
	defer rows.Close()

	var records []struct {
		UserID    int    `json:"user_id"`
		Name      string `json:"name"`
		Year      string `json:"year"`
		GroupName string `json:"group_name"`
		Today     string `json:"today"`
		Present   int    `json:"present"`
		Absent    int    `json:"absent"`
		Late      int    `json:"late"`
		Medical   int    `json:"medical"`
	}

	for rows.Next() {
		var record struct {
			UserID    int    `json:"user_id"`
			Name      string `json:"name"`
			Year      string `json:"year"`
			GroupName string `json:"group_name"`
			Today     string `json:"today"`
			Present   int    `json:"present"`
			Absent    int    `json:"absent"`
			Late      int    `json:"late"`
			Medical   int    `json:"medical"`
		}
		err := rows.Scan(
			&record.UserID,
			&record.Name,
			&record.Year,
			&record.GroupName,
			&record.Today,
			&record.Present,
			&record.Absent,
			&record.Late,
			&record.Medical,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning attendance record: %v", err),
			})
			return
		}
		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error iterating through records: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    records,
	})
}

// SetupAttendanceRoutes sets up the attendance routes
func SetupAttendanceRoutes(router gin.IRouter, db *sql.DB) {
	attendanceGroup := router.Group("/attendance")
	{
		attendanceGroup.GET("/year-groups", func(c *gin.Context) {
			GetYearGroups(c, db)
		})
		attendanceGroup.GET("/students/:id", func(c *gin.Context) {
			GetStudentsByYearGroup(c, db)
		})
		attendanceGroup.POST("/update", func(c *gin.Context) {
			UpdateAttendance(c, db)
		})
		attendanceGroup.GET("/student/:id", func(c *gin.Context) {
			GetStudentAttendance(c, db)
		})
		attendanceGroup.GET("/all", func(c *gin.Context) {
			GetAllAttendance(c, db)
		})
	}
}
