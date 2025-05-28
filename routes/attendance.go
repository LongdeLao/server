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

/*
CRON JOB SETUP FOR AUTO-MARKING LATE STUDENTS

This server includes a function to automatically mark students as late at 7:40 AM Shanghai time.
To set this up, you need to create a cron job on the server.

1. Create a small Go program that calls the AutoMarkLateStudents function:

```go
// auto_mark_late.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"server/routes"  // Import your server's routes package
	"time"

	_ "github.com/lib/pq"  // Or whatever database driver you're using
)

func main() {
	// Connect to the database
	db, err := sql.Open("postgres", "your_connection_string_here")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Log the execution time
	now := time.Now()
	fmt.Printf("Auto-marking late students at %s\n", now.Format(time.RFC3339))

	// Call the auto-mark function with nil to use current time
	routes.AutoMarkLateStudents(db, nil)
}
```

2. Compile this program:
   ```
   go build -o auto_mark_late auto_mark_late.go
   ```

3. Set up a cron job to run at 7:40 AM Shanghai time (which is UTC+8):
   - 7:40 AM Shanghai time = 23:40 PM UTC (previous day)

   Add this to your crontab (run `crontab -e`):
   ```
   # Run at 7:40 AM Shanghai time (23:40 UTC)
   40 23 * * 1-5 /path/to/auto_mark_late >> /var/log/auto_mark_late.log 2>&1
   ```

   The '1-5' means Monday through Friday (weekdays only).

4. Make sure the log file is writable:
   ```
   touch /var/log/auto_mark_late.log
   chmod 644 /var/log/auto_mark_late.log
   ```

This setup will automatically mark pending students as late at 7:40 AM Shanghai time
on weekdays, which matches the behavior previously implemented on the client side.
*/

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

		// Query to get attendance statistics for this year group - counting students with "Present" and "Late" status today
		var presentToday, lateToday, totalStudents int
		err = db.QueryRow(`
			SELECT 
				COUNT(CASE WHEN today = 'Present' THEN 1 END),
				COUNT(CASE WHEN today = 'Late' THEN 1 END),
				COUNT(*)
			FROM attendance 
			WHERE year = $1 AND group_name = $2
		`, group.Year, group.Section).Scan(&presentToday, &lateToday, &totalStudents)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error getting attendance stats: %v", err),
			})
			return
		}

		// Calculate attendance percentage correctly based on today's status
		var attendancePercentage string
		if totalStudents > 0 {
			// Students who are present or late today both count as "attending"
			attendingToday := presentToday + lateToday

			// Calculate percentage - this should never exceed 100%
			percentage := float64(attendingToday) / float64(totalStudents) * 100
			attendancePercentage = fmt.Sprintf("%.1f%%", percentage)

			fmt.Printf("[DEBUG] Year Group %s %s: PresentToday=%d, LateToday=%d, TotalStudents=%d, Attendance=%.1f%%\n",
				group.Year, group.Section, presentToday, lateToday, totalStudents, percentage)
		} else {
			attendancePercentage = "0%"
			fmt.Printf("[DEBUG] Year Group %s %s: Attendance Percentage = 0%% (No students)\n",
				group.Year, group.Section)
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

// GetStudentAttendanceHistory retrieves all attendance records for a specific student
//
// Endpoint: GET /api/attendance/history/:id
//
// Parameters:
//   - id: The student's user ID (integer)
//
// Returns:
//   - 200 OK: Successfully retrieved attendance history records
//     {
//     "success": true,
//     "records": [
//     {
//     "id": int,
//     "student_id": int,
//     "status": string,        // "present", "absent", "late", "medical", or "early"
//     "attendance_date": string, // YYYY-MM-DD format
//     "arrived_at": string,    // HH:MM:SS format, null for non-late status
//     "created_at": string     // timestamp
//     }
//     ]
//     }
//   - 400 Bad Request: Invalid student ID format
//   - 404 Not Found: No attendance records found for the student
//   - 500 Internal Server Error: Database error
func GetStudentAttendanceHistory(c *gin.Context, db *sql.DB) {
	studentIDStr := c.Param("id")
	fmt.Printf("Received request for student attendance history, ID: %s\n", studentIDStr)

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

	// First, check if the student exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", studentID).Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking if student exists: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking if student exists: %v", err),
		})
		return
	}

	if !exists {
		fmt.Printf("Student not found with ID: %d\n", studentID)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("Student not found with ID: %d", studentID),
		})
		return
	}

	// Query to get attendance history records for the student
	query := `
		SELECT 
			id,
			student_id,
			status,
			attendance_date,
			arrived_at,
			created_at
		FROM attendance_history 
		WHERE student_id = $1
		ORDER BY attendance_date DESC
	`

	rows, err := db.Query(query, studentID)
	if err != nil {
		fmt.Printf("Error querying attendance history: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error querying attendance history: %v", err),
		})
		return
	}
	defer rows.Close()

	var records []gin.H
	for rows.Next() {
		var id, studentID int
		var status string
		var attendanceDate time.Time
		var arrivedAt sql.NullTime
		var createdAt time.Time

		err := rows.Scan(
			&id,
			&studentID,
			&status,
			&attendanceDate,
			&arrivedAt,
			&createdAt,
		)

		if err != nil {
			fmt.Printf("Error scanning attendance record: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Error scanning attendance record: %v", err),
			})
			return
		}

		record := gin.H{
			"id":              id,
			"student_id":      studentID,
			"status":          status,
			"attendance_date": attendanceDate.Format("2006-01-02"),
			"created_at":      createdAt.Format(time.RFC3339),
		}

		if arrivedAt.Valid {
			record["arrived_at"] = arrivedAt.Time.Format("15:04:05")
		} else {
			record["arrived_at"] = nil
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("Error iterating through records: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error iterating through records: %v", err),
		})
		return
	}

	// If no records found
	if len(records) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"records": []gin.H{},
			"message": "No attendance history records found for this student",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"records": records,
	})
}

// Helper function to map status to counter field name
func getCounterField(status string) string {
	switch status {
	case "present":
		return "present"
	case "absent":
		return "absent"
	case "late":
		return "late"
	case "medical":
		return "medical"
	case "early":
		return "early"
	default:
		return ""
	}
}

// Helper function to validate and normalize attendance status
func validateAndNormalizeStatus(status string) (bool, string, string, bool) {
	// Convert to uppercase for validation
	upperStatus := strings.ToUpper(status)

	// Track if this is a special "Late_Arrived" status
	isLateArrived := upperStatus == "LATE_ARRIVED"

	// Check if it's a valid status or the special Late_Arrived status
	isValid := upperStatus == "" ||
		upperStatus == "PRESENT" ||
		upperStatus == "ABSENT" ||
		upperStatus == "LATE" ||
		upperStatus == "MEDICAL" ||
		upperStatus == "EARLY" ||
		upperStatus == "PENDING" ||
		upperStatus == "LATE_ARRIVED" // Added special status

	// Get properly cased status for the attendance table
	var properCaseStatus string
	switch upperStatus {
	case "PRESENT":
		properCaseStatus = "Present"
	case "ABSENT":
		properCaseStatus = "Absent"
	case "LATE", "LATE_ARRIVED": // Both are saved as "Late" in the attendance table
		properCaseStatus = "Late"
	case "MEDICAL":
		properCaseStatus = "Medical"
	case "EARLY":
		properCaseStatus = "Early"
	case "PENDING", "":
		properCaseStatus = "Pending"
	default:
		properCaseStatus = status
	}

	// Get lowercase status for attendance_history table
	var lowerCaseStatus string
	switch upperStatus {
	case "PRESENT":
		lowerCaseStatus = "present"
	case "ABSENT":
		lowerCaseStatus = "absent"
	case "LATE", "LATE_ARRIVED": // Both are saved as "late" in the attendance_history table
		lowerCaseStatus = "late"
	case "MEDICAL":
		lowerCaseStatus = "medical"
	case "EARLY":
		lowerCaseStatus = "early"
	case "PENDING", "":
		lowerCaseStatus = "pending"
	default:
		lowerCaseStatus = strings.ToLower(status)
	}

	return isValid, properCaseStatus, lowerCaseStatus, isLateArrived
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

	// Parse the attendance date from the request or use today's date
	attendanceDate := time.Now().UTC()
	if request.Date != "" {
		parsedDate, err := time.Parse("2006-01-02", request.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid date format: %v", err),
			})
			return
		}
		attendanceDate = parsedDate
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

		// Validate and normalize the status
		isValid, properCaseStatus, lowerCaseStatus, isLateArrived := validateAndNormalizeStatus(student.Status)

		if !isValid {
			// Invalid status provided
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid status '%s' for user ID %d", student.Status, student.UserID),
			})
			return
		}

		// Print detailed debug information
		fmt.Printf("Processing attendance update: UserID=%d, RawStatus=%s, NormalizedStatus=%s, IsLateArrived=%v\n",
			student.UserID, student.Status, lowerCaseStatus, isLateArrived)

		// Update the today field in the attendance table
		result, err := tx.Exec(`
			UPDATE attendance 
			SET today = $1
			WHERE user_id = $2
		`, properCaseStatus, student.UserID)

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

		// Only insert into attendance_history if status is not empty or "Pending"
		if lowerCaseStatus != "" && lowerCaseStatus != "pending" {
			// Check if an entry already exists for this student on this date
			var existingId int
			err = tx.QueryRow(`
				SELECT id FROM attendance_history 
				WHERE student_id = $1 AND attendance_date = $2
			`, student.UserID, attendanceDate.Format("2006-01-02")).Scan(&existingId)

			if err != nil && err != sql.ErrNoRows {
				tx.Rollback()
				fmt.Printf("Error checking for existing attendance history: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("Error checking for existing attendance history: %v", err),
				})
				return
			}

			if err == sql.ErrNoRows {
				// No existing entry, insert a new one
				var err error

				if lowerCaseStatus == "late" && isLateArrived {
					// Try a direct update bypassing potential constraints with verbose error logging
					arrivedTime := time.Now().UTC().Format("15:04:05")
					fmt.Printf("ATTEMPTING DIRECT ARRIVAL TIME UPDATE: UserID=%d, Time=%s\n", student.UserID, arrivedTime)

					// First, try to just update without constraints using a direct SQL query
					directSQL := `
						UPDATE attendance_history 
						SET arrived_at = $1
						WHERE student_id = $2 AND attendance_date = $3
					`
					_, err = tx.Exec(directSQL, arrivedTime, student.UserID, attendanceDate.Format("2006-01-02"))

					if err != nil {
						// Try to determine the exact constraint that's failing
						fmt.Printf("DIRECT UPDATE FAILED: %v\n", err)

						// Let's try to inspect the constraint
						var constraintName string
						err = tx.QueryRow(`
							SELECT constraint_name 
							FROM information_schema.table_constraints 
							WHERE table_name = 'attendance_history' 
							AND constraint_type = 'CHECK'
						`).Scan(&constraintName)

						if err == nil {
							fmt.Printf("CONSTRAINT NAME: %s\n", constraintName)

							// Try to get the constraint definition
							var constraintDef string
							err = tx.QueryRow(`
								SELECT pg_get_constraintdef(oid) 
								FROM pg_constraint 
								WHERE conname = $1
							`, constraintName).Scan(&constraintDef)

							if err == nil {
								fmt.Printf("CONSTRAINT DEFINITION: %s\n", constraintDef)
							} else {
								fmt.Printf("FAILED TO GET CONSTRAINT DEFINITION: %v\n", err)
							}
						} else {
							fmt.Printf("FAILED TO GET CONSTRAINT NAME: %v\n", err)
						}

						// Let's try a more aggressive approach - first make sure the status is 'late'
						_, err = tx.Exec(`
							UPDATE attendance_history 
							SET status = 'late'
							WHERE student_id = $1 AND attendance_date = $2
						`, student.UserID, attendanceDate.Format("2006-01-02"))

						if err != nil {
							fmt.Printf("FAILED TO UPDATE STATUS TO LATE: %v\n", err)
						} else {
							// Now try to set the arrived_at field
							_, err = tx.Exec(`
								UPDATE attendance_history 
								SET arrived_at = $1
								WHERE student_id = $2 AND attendance_date = $3
							`, arrivedTime, student.UserID, attendanceDate.Format("2006-01-02"))

							if err != nil {
								fmt.Printf("FAILED TO UPDATE ARRIVED_AT AFTER STATUS SET: %v\n", err)
							} else {
								fmt.Printf("SUCCESS! DIRECT ARRIVAL TIME UPDATE WORKED AFTER STATUS SET\n")
							}
						}
					} else {
						fmt.Printf("SUCCESS! DIRECT ARRIVAL TIME UPDATE WORKED\n")
					}
				} else if lowerCaseStatus == "late" {
					// Regular late student (auto-marked) - don't set arrived_at
					fmt.Printf("Auto-marked late student - NOT setting arrival time\n")

					_, err = tx.Exec(`
						INSERT INTO attendance_history 
						(student_id, status, attendance_date, arrived_at, created_at)
						VALUES ($1, $2, $3, NULL, $4)
					`, student.UserID, lowerCaseStatus, attendanceDate.Format("2006-01-02"), time.Now().UTC())
				} else {
					// Any other status - don't set arrived_at
					fmt.Printf("Non-late status - NOT setting arrival time\n")

					_, err = tx.Exec(`
						INSERT INTO attendance_history 
						(student_id, status, attendance_date, arrived_at, created_at)
						VALUES ($1, $2, $3, NULL, $4)
					`, student.UserID, lowerCaseStatus, attendanceDate.Format("2006-01-02"), time.Now().UTC())
				}

				if err != nil {
					tx.Rollback()
					fmt.Printf("Error inserting into attendance_history for user ID %d: %v\n", student.UserID, err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": fmt.Sprintf("Error inserting into attendance_history for user ID %d: %v", student.UserID, err),
					})
					return
				}

				// Increment the corresponding counter in the attendance table for new entries
				counterField := getCounterField(lowerCaseStatus)
				if counterField != "" {
					_, err = tx.Exec(fmt.Sprintf(`
						UPDATE attendance 
						SET %s = %s + 1
						WHERE user_id = $1
					`, counterField, counterField), student.UserID)

					if err != nil {
						tx.Rollback()
						fmt.Printf("Error updating %s counter for user ID %d: %v\n", counterField, student.UserID, err)
						c.JSON(http.StatusInternalServerError, gin.H{
							"success": false,
							"message": fmt.Sprintf("Error updating %s counter for user ID %d: %v", counterField, student.UserID, err),
						})
						return
					}
				}
			} else {
				// Entry exists, get the old status before updating
				var oldStatus string
				var oldArrivedAt sql.NullString
				err = tx.QueryRow(`
					SELECT status, arrived_at FROM attendance_history 
					WHERE id = $1
				`, existingId).Scan(&oldStatus, &oldArrivedAt)

				if err != nil {
					tx.Rollback()
					fmt.Printf("Error getting old status: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": fmt.Sprintf("Error getting old status: %v", err),
					})
					return
				}

				fmt.Printf("Existing record found - OldStatus: %s, HasArrivalTime: %v\n",
					oldStatus, oldArrivedAt.Valid)

				// Update based on status and whether this is a manual arrival
				var err error

				if lowerCaseStatus == "late" && isLateArrived {
					// Student is late and teacher is marking arrival
					arrivedTime := time.Now().UTC().Format("15:04:05")
					fmt.Printf("Setting arrival time for existing late student: %s\n", arrivedTime)

					// Update with arrival time
					_, err = tx.Exec(`
						UPDATE attendance_history 
						SET status = $1, arrived_at = $2
						WHERE id = $3
					`, lowerCaseStatus, arrivedTime, existingId)
				} else if lowerCaseStatus == "late" {
					// Regular late status update - keep existing arrived_at value if any
					fmt.Printf("Updating to late status but not marking arrival - keeping existing arrived_at\n")

					// Only update the status, don't touch arrived_at
					_, err = tx.Exec(`
						UPDATE attendance_history 
						SET status = $1
						WHERE id = $2
					`, lowerCaseStatus, existingId)
				} else {
					// Non-late status - set arrived_at to NULL
					fmt.Printf("Updating to non-late status - setting arrived_at to NULL\n")

					// Set arrived_at to NULL for non-late statuses
					_, err = tx.Exec(`
						UPDATE attendance_history 
						SET status = $1, arrived_at = NULL
						WHERE id = $2
					`, lowerCaseStatus, existingId)
				}

				if err != nil {
					tx.Rollback()
					fmt.Printf("Error updating attendance_history for user ID %d: %v\n", student.UserID, err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": fmt.Sprintf("Error updating attendance_history for user ID %d: %v", student.UserID, err),
					})
					return
				}

				// If the status has changed, update the counters in the attendance table
				if oldStatus != lowerCaseStatus {
					// Decrement the old status counter
					oldCounterField := getCounterField(oldStatus)
					if oldCounterField != "" {
						_, err = tx.Exec(fmt.Sprintf(`
							UPDATE attendance 
							SET %s = GREATEST(0, %s - 1)
							WHERE user_id = $1
						`, oldCounterField, oldCounterField), student.UserID)

						if err != nil {
							tx.Rollback()
							fmt.Printf("Error decrementing %s counter for user ID %d: %v\n", oldCounterField, student.UserID, err)
							c.JSON(http.StatusInternalServerError, gin.H{
								"success": false,
								"message": fmt.Sprintf("Error decrementing %s counter for user ID %d: %v", oldCounterField, student.UserID, err),
							})
							return
						}
					}

					// Increment the new status counter
					newCounterField := getCounterField(lowerCaseStatus)
					if newCounterField != "" {
						_, err = tx.Exec(fmt.Sprintf(`
							UPDATE attendance 
							SET %s = %s + 1
							WHERE user_id = $1
						`, newCounterField, newCounterField), student.UserID)

						if err != nil {
							tx.Rollback()
							fmt.Printf("Error incrementing %s counter for user ID %d: %v\n", newCounterField, student.UserID, err)
							c.JSON(http.StatusInternalServerError, gin.H{
								"success": false,
								"message": fmt.Sprintf("Error incrementing %s counter for user ID %d: %v", newCounterField, student.UserID, err),
							})
							return
						}
					}
				}

				// Add the same direct approach for existing records
				if existingId > 0 && lowerCaseStatus == "late" && isLateArrived {
					// Try a direct update bypassing potential constraints with verbose error logging
					arrivedTime := time.Now().UTC().Format("15:04:05")
					fmt.Printf("ATTEMPTING DIRECT ARRIVAL TIME UPDATE FOR EXISTING RECORD: ID=%d, UserID=%d, Time=%s\n",
						existingId, student.UserID, arrivedTime)

					// First, make sure the status is set to 'late'
					_, err = tx.Exec(`
						UPDATE attendance_history 
						SET status = 'late'
						WHERE id = $1
					`, existingId)

					if err != nil {
						fmt.Printf("FAILED TO UPDATE STATUS TO LATE: %v\n", err)
					} else {
						// Now try to set the arrived_at field
						_, err = tx.Exec(`
							UPDATE attendance_history 
							SET arrived_at = $1
							WHERE id = $2
						`, arrivedTime, existingId)

						if err != nil {
							fmt.Printf("FAILED TO UPDATE ARRIVED_AT AFTER STATUS SET: %v\n", err)
						} else {
							fmt.Printf("SUCCESS! DIRECT ARRIVAL TIME UPDATE WORKED FOR EXISTING RECORD\n")
						}
					}
				}
			}
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

	// Calculate attendance statistics based on the historical counters
	// These accumulated counters are more reliable for historical attendance calculation
	totalDays := record.Present + record.Absent + record.Late + record.Medical + record.Early
	var attendancePercentage float64
	if totalDays > 0 {
		// Calculate the percentage of days the student was present or late (both count as attending)
		attendedDays := record.Present + record.Late
		attendancePercentage = float64(attendedDays) / float64(totalDays) * 100

		// Add debug logging
		fmt.Printf("[DEBUG] Student %s (ID=%d): Historical attendance - Present=%d, Late=%d, Total=%d, Attendance=%.1f%%\n",
			record.Name, record.UserID, record.Present, record.Late, totalDays, attendancePercentage)
	} else {
		// Add debug logging
		fmt.Printf("[DEBUG] Student %s (ID=%d): No historical attendance data\n", record.Name, record.UserID)
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
				"total":      totalDays,
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

// New function for marking student arrival time
func MarkStudentArrival(c *gin.Context, db *sql.DB) {
	var request struct {
		StudentID int    `json:"student_id"`
		Date      string `json:"date"`
	}

	// Read and parse the request
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request format: %v", err),
		})
		return
	}

	// Log the request
	fmt.Printf("[MARK ARRIVAL] Processing arrival for Student ID=%d, Date=%s\n",
		request.StudentID, request.Date)

	// Validate student ID
	if request.StudentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid student ID",
		})
		return
	}

	// Parse the attendance date from the request or use today's date
	attendanceDate := time.Now().UTC()
	if request.Date != "" {
		parsedDate, err := time.Parse("2006-01-02", request.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid date format: %v", err),
			})
			return
		}
		attendanceDate = parsedDate
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

	// First, check if student is marked as late for this date
	var existingId int
	var currentStatus string
	var studentName string
	err = tx.QueryRow(`
		SELECT a.id, a.status, s.name 
		FROM attendance_history a
		JOIN attendance s ON a.student_id = s.user_id
		WHERE a.student_id = $1 AND a.attendance_date = $2
	`, request.StudentID, attendanceDate.Format("2006-01-02")).Scan(&existingId, &currentStatus, &studentName)

	if err != nil {
		if err == sql.ErrNoRows {
			tx.Rollback()
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "No attendance record found for this student on this date",
			})
			return
		}

		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error checking attendance history: %v", err),
		})
		return
	}

	// Log the current status
	fmt.Printf("[MARK ARRIVAL] Student %s (ID=%d) current status: %s\n",
		studentName, request.StudentID, currentStatus)

	// Verify the student is marked as late
	if currentStatus != "late" {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Student is not marked as late (current status: %s)", currentStatus),
		})
		return
	}

	// Set the arrival time in UTC
	arrivedTime := time.Now().UTC().Format("15:04:05")

	// Log the arrival time
	fmt.Printf("[MARK ARRIVAL] Setting arrival time for Student %s (ID=%d): %s UTC\n",
		studentName, request.StudentID, arrivedTime)

	// 1. Update the attendance_history table: set arrived_at time and change status to "present"
	_, err = tx.Exec(`
		UPDATE attendance_history 
		SET arrived_at = $1, status = 'present'
		WHERE id = $2
	`, arrivedTime, existingId)

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error updating arrival time: %v", err),
		})
		return
	}

	// 2. Update the attendance table: change today's status from "Late" to "Present"
	_, err = tx.Exec(`
		UPDATE attendance 
		SET today = 'Present'
		WHERE user_id = $1
	`, request.StudentID)

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error updating attendance status: %v", err),
		})
		return
	}

	// 3. Update the counters in the attendance table
	// Decrement the late counter
	_, err = tx.Exec(`
		UPDATE attendance 
		SET late = GREATEST(0, late - 1)
		WHERE user_id = $1
	`, request.StudentID)

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error updating late counter: %v", err),
		})
		return
	}

	// Increment the present counter
	_, err = tx.Exec(`
		UPDATE attendance 
		SET present = present + 1
		WHERE user_id = $1
	`, request.StudentID)

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error updating present counter: %v", err),
		})
		return
	}

	// Get the updated counters for logging
	var present, late int
	err = tx.QueryRow(`
		SELECT present, late FROM attendance WHERE user_id = $1
	`, request.StudentID).Scan(&present, &late)

	if err == nil {
		fmt.Printf("[MARK ARRIVAL] Updated counters for Student %s (ID=%d): Present=%d, Late=%d\n",
			studentName, request.StudentID, present, late)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error committing transaction: %v", err),
		})
		return
	}

	fmt.Printf("[MARK ARRIVAL] Successfully marked arrival for Student %s (ID=%d)\n",
		studentName, request.StudentID)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Arrival time recorded successfully and status updated to Present",
		"student_id": request.StudentID,
		"arrived_at": arrivedTime,
	})
}

// New function to auto-mark students as late at 7:40 AM Shanghai time
func AutoMarkLateStudents(db *sql.DB, targetTime *time.Time) {
	// This function can be called by a cron job at 7:40 AM Shanghai time
	// Or it can be called with a specific targetTime for testing

	// If targetTime is nil, use current time
	now := time.Now().UTC()
	if targetTime != nil {
		now = targetTime.UTC()
	}

	// Get date in YYYY-MM-DD format from the time
	today := now.Format("2006-01-02")

	fmt.Printf("[AUTO-MARK] Starting auto-marking of late students at %s (UTC)\n", now.Format(time.RFC3339))
	fmt.Printf("[AUTO-MARK] Using date: %s\n", today)

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("[AUTO-MARK ERROR] Error starting transaction: %v\n", err)
		return
	}

	// Get all students with "Pending" status
	fmt.Println("[AUTO-MARK] Querying for students with 'Pending' status...")
	rows, err := tx.Query(`
		SELECT user_id, name
		FROM attendance 
		WHERE today = 'Pending'
	`)

	if err != nil {
		tx.Rollback()
		fmt.Printf("[AUTO-MARK ERROR] Error querying pending students: %v\n", err)
		return
	}
	defer rows.Close()

	// Collect student IDs
	type studentInfo struct {
		ID   int
		Name string
	}
	var students []studentInfo

	for rows.Next() {
		var student studentInfo
		if err := rows.Scan(&student.ID, &student.Name); err != nil {
			tx.Rollback()
			fmt.Printf("[AUTO-MARK ERROR] Error scanning student data: %v\n", err)
			return
		}
		students = append(students, student)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		tx.Rollback()
		fmt.Printf("[AUTO-MARK ERROR] Error iterating through students: %v\n", err)
		return
	}

	// Early exit if no pending students
	if len(students) == 0 {
		tx.Rollback() // No changes made
		fmt.Println("[AUTO-MARK] No pending students to mark late")
		return
	}

	fmt.Printf("[AUTO-MARK] Found %d students with 'Pending' status\n", len(students))
	for i, student := range students {
		if i < 5 { // Log only the first 5 to avoid overwhelming logs
			fmt.Printf("[AUTO-MARK]   - Student #%d: ID=%d, Name=%s\n", i+1, student.ID, student.Name)
		} else if i == 5 {
			fmt.Printf("[AUTO-MARK]   - ...and %d more students\n", len(students)-5)
			break
		}
	}

	// Update each student to "Late" status
	successCount := 0
	errorCount := 0

	for i, student := range students {
		fmt.Printf("[AUTO-MARK] Processing student %d/%d: ID=%d, Name=%s\n",
			i+1, len(students), student.ID, student.Name)

		// Update attendance table
		_, err = tx.Exec(`
			UPDATE attendance 
			SET today = 'Late'
			WHERE user_id = $1
		`, student.ID)

		if err != nil {
			fmt.Printf("[AUTO-MARK ERROR] Error updating attendance for student %d: %v\n", student.ID, err)
			errorCount++
			continue
		}

		// Check if a record already exists for today
		var existingId int
		err = tx.QueryRow(`
			SELECT id FROM attendance_history 
			WHERE student_id = $1 AND attendance_date = $2
		`, student.ID, today).Scan(&existingId)

		if err != nil && err != sql.ErrNoRows {
			fmt.Printf("[AUTO-MARK ERROR] Error checking for existing attendance history for student %d: %v\n", student.ID, err)
			errorCount++
			continue
		}

		if err == sql.ErrNoRows {
			// Insert new record with NULL arrived_at
			fmt.Printf("[AUTO-MARK] No existing record found for student %d, creating new entry\n", student.ID)
			_, err = tx.Exec(`
				INSERT INTO attendance_history 
				(student_id, status, attendance_date, arrived_at, created_at)
				VALUES ($1, 'late', $2, NULL, $3)
			`, student.ID, today, time.Now().UTC())

			if err != nil {
				fmt.Printf("[AUTO-MARK ERROR] Error inserting attendance history for student %d: %v\n", student.ID, err)
				errorCount++
				continue
			}

			// Increment the late counter
			_, err = tx.Exec(`
				UPDATE attendance 
				SET late = late + 1
				WHERE user_id = $1
			`, student.ID)

			if err != nil {
				fmt.Printf("[AUTO-MARK ERROR] Error incrementing late counter for student %d: %v\n", student.ID, err)
				errorCount++
				continue
			}
		} else {
			// Update existing record to late with NULL arrived_at
			fmt.Printf("[AUTO-MARK] Existing record found (ID=%d) for student %d, updating to 'late'\n",
				existingId, student.ID)

			_, err = tx.Exec(`
				UPDATE attendance_history 
				SET status = 'late', arrived_at = NULL
				WHERE id = $1
			`, existingId)

			if err != nil {
				fmt.Printf("[AUTO-MARK ERROR] Error updating attendance history for student %d: %v\n", student.ID, err)
				errorCount++
				continue
			}

			// Adjust the counters - first get the old status
			var oldStatus string
			err = tx.QueryRow(`
				SELECT status FROM attendance_history 
				WHERE id = $1
			`, existingId).Scan(&oldStatus)

			if err != nil {
				fmt.Printf("[AUTO-MARK ERROR] Error getting old status for student %d: %v\n", student.ID, err)
				errorCount++
				continue
			}

			fmt.Printf("[AUTO-MARK] Student %d previous status was '%s'\n", student.ID, oldStatus)

			// Decrement the old counter if it's not 'late' already
			if oldStatus != "late" {
				oldCounterField := getCounterField(oldStatus)
				if oldCounterField != "" {
					_, err = tx.Exec(fmt.Sprintf(`
						UPDATE attendance 
						SET %s = GREATEST(0, %s - 1)
						WHERE user_id = $1
					`, oldCounterField, oldCounterField), student.ID)

					if err != nil {
						fmt.Printf("[AUTO-MARK ERROR] Error decrementing %s counter for student %d: %v\n",
							oldCounterField, student.ID, err)
						errorCount++
						continue
					}
				}

				// Increment the late counter
				_, err = tx.Exec(`
					UPDATE attendance 
					SET late = late + 1
					WHERE user_id = $1
				`, student.ID)

				if err != nil {
					fmt.Printf("[AUTO-MARK ERROR] Error incrementing late counter for student %d: %v\n", student.ID, err)
					errorCount++
					continue
				}
			}
		}

		successCount++
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		fmt.Printf("[AUTO-MARK ERROR] Error committing transaction: %v\n", err)
		return
	}

	fmt.Printf("[AUTO-MARK] Successfully auto-marked %d students as late (%d errors)\n",
		successCount, errorCount)
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
		attendanceGroup.GET("/history/:id", func(c *gin.Context) {
			GetStudentAttendanceHistory(c, db)
		})
		attendanceGroup.POST("/mark-arrival", func(c *gin.Context) {
			MarkStudentArrival(c, db)
		})
	}
}
