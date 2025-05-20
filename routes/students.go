package routes

import (
	"database/sql"
	"net/http"
	"strconv"

	"server/models"

	"github.com/gin-gonic/gin"
)

// GetAllStudentsHandler handles the request to get all students
// Requires a user ID with admin rights to perform this operation
func GetAllStudentsHandler(c *gin.Context, db *sql.DB) {
	// Get the user ID from the query parameter
	userIDStr := c.Query("userid")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "User ID is required",
		})
		return
	}

	// Convert the user ID to an integer
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID format",
		})
		return
	}

	// Check if the user has admin rights
	var hasAttendanceRole bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM additional_roles 
			WHERE user_id = $1 AND name = 'attendance'
		)
	`, userID).Scan(&hasAttendanceRole)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to check user role",
			"error":   err.Error(),
		})
		return
	}

	if !hasAttendanceRole {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "You do not have permission to access this resource",
		})
		return
	}

	// Get all users with role "student"
	rows, err := db.Query(`
		SELECT id, first_name, last_name, name, username, 
		       role, email, status, formal_picture
		FROM users
		WHERE role = 'student'
		ORDER BY id
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve students",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	// Process the result set
	var students []map[string]interface{}
	for rows.Next() {
		var id int
		var firstName, lastName, name, username, role, email, status sql.NullString
		var formalPicture sql.NullString

		err := rows.Scan(
			&id,
			&firstName,
			&lastName,
			&name,
			&username,
			&role,
			&email,
			&status,
			&formalPicture,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error scanning student record",
				"error":   err.Error(),
			})
			return
		}

		student := map[string]interface{}{
			"id": id,
		}

		if firstName.Valid {
			student["first_name"] = firstName.String
		}
		if lastName.Valid {
			student["last_name"] = lastName.String
		}
		if name.Valid {
			student["name"] = name.String
		}
		if username.Valid {
			student["username"] = username.String
		}
		if role.Valid {
			student["role"] = role.String
		}
		if email.Valid {
			student["email"] = email.String
		}
		if status.Valid {
			student["status"] = status.String
		}
		if formalPicture.Valid {
			student["formal_picture"] = formalPicture.String
		}

		students = append(students, student)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"students": students,
		"count":    len(students),
	})
}

// GetStudentInformationHandler gets detailed information about a specific student
func GetStudentInformationHandler(c *gin.Context, db *sql.DB) {
	// Get the student ID from the query parameter
	studentIDStr := c.Query("userid")
	if studentIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Student ID is required",
		})
		return
	}

	// Convert the student ID to an integer
	studentID, err := strconv.Atoi(studentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid student ID format",
		})
		return
	}

	// Get the student's basic information
	var firstName, lastName, name sql.NullString
	var formalPicture sql.NullString
	var role sql.NullString

	err = db.QueryRow(`
		SELECT first_name, last_name, name, formal_picture, role
		FROM users
		WHERE id = $1
	`, studentID).Scan(&firstName, &lastName, &name, &formalPicture, &role)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Student not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to retrieve student information",
				"error":   err.Error(),
			})
		}
		return
	}

	// Check if the user is a student
	if !role.Valid || role.String != "student" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "The specified user is not a student",
		})
		return
	}

	// Get the classes (subjects) the student is enrolled in
	rows, err := db.Query(`
		SELECT id, subject, code, teaching_group, initials
		FROM subjects
		WHERE student_id = $1
	`, studentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve student classes",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	var classes []map[string]interface{}
	for rows.Next() {
		var id int
		var subject, code, teachingGroup, initials sql.NullString

		err := rows.Scan(&id, &subject, &code, &teachingGroup, &initials)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error scanning class record",
				"error":   err.Error(),
			})
			return
		}

		class := map[string]interface{}{
			"id": id,
		}

		if subject.Valid {
			class["subject"] = subject.String
		}
		if code.Valid {
			class["code"] = code.String
		}
		if teachingGroup.Valid {
			class["teaching_group"] = teachingGroup.String
		}
		if initials.Valid {
			class["initials"] = initials.String
		}

		classes = append(classes, class)
	}

	// Get the student's attendance statistics
	var attendanceData struct {
		Present int `json:"present"`
		Absent  int `json:"absent"`
		Late    int `json:"late"`
		Medical int `json:"medical"`
		Early   int `json:"early"`
		Today   sql.NullString
	}

	err = db.QueryRow(`
		SELECT present, absent, late, medical, early, today
		FROM attendance
		WHERE user_id = $1
	`, studentID).Scan(
		&attendanceData.Present,
		&attendanceData.Absent,
		&attendanceData.Late,
		&attendanceData.Medical,
		&attendanceData.Early,
		&attendanceData.Today,
	)

	// If there's no attendance record, set defaults
	if err == sql.ErrNoRows {
		attendanceData.Present = 0
		attendanceData.Absent = 0
		attendanceData.Late = 0
		attendanceData.Medical = 0
		attendanceData.Early = 0
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve attendance statistics",
			"error":   err.Error(),
		})
		return
	}

	// Build and return the response
	response := gin.H{
		"success": true,
		"student": gin.H{
			"id":         studentID,
			"classes":    classes,
			"attendance": attendanceData,
		},
	}

	if firstName.Valid {
		response["student"].(gin.H)["first_name"] = firstName.String
	}
	if lastName.Valid {
		response["student"].(gin.H)["last_name"] = lastName.String
	}
	if name.Valid {
		response["student"].(gin.H)["name"] = name.String
	}
	if formalPicture.Valid {
		response["student"].(gin.H)["formal_picture"] = formalPicture.String
	}
	if attendanceData.Today.Valid {
		response["student"].(gin.H)["today_status"] = attendanceData.Today.String
	}

	c.JSON(http.StatusOK, response)
}

// SetupStudentRoutes registers all student management routes
func SetupStudentRoutes(router gin.IRouter, db *sql.DB) {
	// Get all students (requires admin rights)
	router.GET("/get_all_students", func(c *gin.Context) {
		GetAllStudentsHandler(c, db)
	})

	// Get detailed information about a specific student
	router.GET("/get_student_information", func(c *gin.Context) {
		GetStudentInformationHandler(c, db)
	})
} 