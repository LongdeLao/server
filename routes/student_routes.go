package routes

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// StudentProfile represents a comprehensive profile of a student
type StudentProfile struct {
	ID            int        `json:"id"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	FullName      string     `json:"full_name"`
	FormalPicture string     `json:"formal_picture"`
	YearGroup     string     `json:"year_group"`
	GroupName     string     `json:"group_name"`
	Classes       []Subject  `json:"classes"`
	Attendance    Attendance `json:"attendance"`
}

// Attendance represents attendance statistics for a student
type Attendance struct {
	Present   int    `json:"present"`
	Absent    int    `json:"absent"`
	Late      int    `json:"late"`
	Medical   int    `json:"medical"`
	Early     int    `json:"early"`
	Today     string `json:"today"`
}

// GetAllStudentsHandler handles the request to get all student users
func GetAllStudentsHandler(c *gin.Context, db *sql.DB) {
	userID := c.Query("userid")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Missing required userid parameter",
		})
		return
	}

	// Check if user has attendance role
	var hasAttendanceRole bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM additional_roles 
			WHERE user_id = $1 AND role = 'attendance'
		)
	`
	
	err := db.QueryRow(query, userID).Scan(&hasAttendanceRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to check user permissions",
			"error":   err.Error(),
		})
		return
	}

	if !hasAttendanceRole {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "User does not have attendance admin permissions",
		})
		return
	}

	// Get all students (users with role = 'student')
	students, err := getStudents(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve students",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"students": students,
		"count":    len(students),
	})
}

// getStudents retrieves all users with role 'student'
func getStudents(db *sql.DB) ([]map[string]interface{}, error) {
	// Debug: let's examine the structure of the attendance table
	debugQuery := `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = 'attendance'
	`
	debugRows, err := db.Query(debugQuery)
	if err == nil {
		defer debugRows.Close()
		fmt.Println("Attendance table structure:")
		for debugRows.Next() {
			var columnName, dataType string
			if err := debugRows.Scan(&columnName, &dataType); err == nil {
				fmt.Printf("  Column: %s, Type: %s\n", columnName, dataType)
			}
		}
	}

	// Simplified query to just join with attendance table
	query := `
		SELECT u.id, u.first_name, u.last_name, u.name, u.formal_picture, 
		       a.year, a.group_name
		FROM users u
		LEFT JOIN attendance a ON u.id = a.user_id
		WHERE u.role = 'student'
		ORDER BY u.last_name, u.first_name
	`

	rows, err := db.Query(query)
	if err != nil {
		fmt.Println("Error in student query:", err)
		return nil, err
	}
	defer rows.Close()

	var students []map[string]interface{}
	for rows.Next() {
		var id int
		var firstName, lastName, name, formalPicture sql.NullString
		var year, groupName sql.NullString
		
		err := rows.Scan(&id, &firstName, &lastName, &name, &formalPicture, &year, &groupName)
		if err != nil {
			fmt.Println("Error scanning student row:", err)
			return nil, err
		}

		// Debug: Print student year group info
		fmt.Printf("Student %d - Year: %v, Group: %v\n", 
			id, getStringValue(year), getStringValue(groupName))

		// Combine year and group for display
		var displayYearGroup string
		yearStr := getStringValue(year)
		groupStr := getStringValue(groupName)
		
		if yearStr != "" {
			displayYearGroup = yearStr
			if groupStr != "" {
				displayYearGroup += " " + groupStr
			}
		}

		student := map[string]interface{}{
			"id":             id,
			"first_name":     getStringValue(firstName),
			"last_name":      getStringValue(lastName),
			"name":           getStringValue(name),
			"formal_picture": getStringValue(formalPicture),
			"year_group":     displayYearGroup,
		}

		students = append(students, student)
	}

	if err = rows.Err(); err != nil {
		fmt.Println("Error after scanning all rows:", err)
		return nil, err
	}

	return students, nil
}

// GetStudentInformationHandler handles the request to get detailed student information
func GetStudentInformationHandler(c *gin.Context, db *sql.DB) {
	studentID := c.Query("userid")
	if studentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Missing required userid parameter",
		})
		return
	}

	// Get student basic info
	var student StudentProfile
	query := `
		SELECT id, first_name, last_name, formal_picture
		FROM users
		WHERE id = $1 AND role = 'student'
	`
	
	var firstName, lastName, formalPicture sql.NullString
	err := db.QueryRow(query, studentID).Scan(
		&student.ID,
		&firstName,
		&lastName,
		&formalPicture,
	)
	
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Student not found",
			"error":   err.Error(),
		})
		return
	}

	student.FirstName = getStringValue(firstName)
	student.LastName = getStringValue(lastName)
	student.FullName = student.FirstName + " " + student.LastName
	student.FormalPicture = getStringValue(formalPicture)
	
	// Get student attendance
	attendanceQuery := `
		SELECT present, absent, late, medical, early, today, year, group_name
		FROM attendance
		WHERE user_id = $1
	`
	
	var today, year, groupName sql.NullString
	err = db.QueryRow(attendanceQuery, studentID).Scan(
		&student.Attendance.Present,
		&student.Attendance.Absent,
		&student.Attendance.Late,
		&student.Attendance.Medical,
		&student.Attendance.Early,
		&today,
		&year,
		&groupName,
	)
	
	if err == nil {
		student.Attendance.Today = getStringValue(today)
		student.YearGroup = getStringValue(year)
		student.GroupName = getStringValue(groupName)
	}
	
	// Get student classes (subjects)
	student.Classes, err = getStudentClasses(db, studentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve student classes",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"student": student,
	})
}

// getStudentClasses retrieves all classes for a student
func getStudentClasses(db *sql.DB, studentID string) ([]Subject, error) {
	// Query to fetch subject details for the given student_id.
	query := `
		SELECT subject, code, initials, teaching_group, teacher_id
		FROM subjects
		WHERE student_id = $1;
	`
	
	rows, err := db.Query(query, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []Subject
	for rows.Next() {
		var subject Subject
		if err := rows.Scan(
			&subject.Subject,
			&subject.Code,
			&subject.Initials,
			&subject.TeachingGroup,
			&subject.TeacherID,
		); err != nil {
			continue
		}

		// Query to fetch teacher's full name
		var firstName, lastName sql.NullString
		teacherQuery := "SELECT first_name, last_name FROM users WHERE id = $1 LIMIT 1;"
		err := db.QueryRow(teacherQuery, subject.TeacherID).Scan(&firstName, &lastName)
		
		if err == nil {
			subject.TeacherName = getStringValue(firstName) + " " + getStringValue(lastName)
		} else {
			subject.TeacherName = "Unknown"
		}

		subjects = append(subjects, subject)
	}

	return subjects, nil
}

// Helper function to safely handle NULL strings from the database
func getStringValue(nullString sql.NullString) string {
	if nullString.Valid {
		return nullString.String
	}
	return ""
}

// SetupStudentRoutes registers all student management routes
func SetupStudentRoutes(router gin.IRouter, db *sql.DB) {
	// Add the formal_picture column if it doesn't exist
	addFormalPictureColumn(db)

	// Configure static serving of formal pictures
	router.Static("/formal_pictures", "./formal_pictures")
	router.Static("/api/formal_pictures", "./formal_pictures")

	// Get all students (for administrators with attendance role)
	router.GET("/get_all_students", func(c *gin.Context) {
		GetAllStudentsHandler(c, db)
	})

	// Get detailed information for a specific student
	router.GET("/get_student_information", func(c *gin.Context) {
		GetStudentInformationHandler(c, db)
	})
}

// addFormalPictureColumn adds the formal_picture column to the users table if it doesn't exist
func addFormalPictureColumn(db *sql.DB) {
	// Check if the column already exists
	var exists bool
	checkQuery := `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'users' AND column_name = 'formal_picture'
		);
	`
	
	err := db.QueryRow(checkQuery).Scan(&exists)
	if err != nil || exists {
		// Column exists or error occurred - no need to proceed
		return
	}
	
	// If column doesn't exist, add it
	_, err = db.Exec(`ALTER TABLE users ADD COLUMN formal_picture TEXT;`)
	if err != nil {
		// Log error but don't fail
		return
	}
} 