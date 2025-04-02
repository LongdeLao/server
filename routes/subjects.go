package routes

import (
	"database/sql"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

/**
 * Subject represents the subject data structure returned by the API.
 */
type Subject struct {
	Subject       string `json:"subject"`        // Subject name (e.g., "Mathematics SL")
	Code          string `json:"code"`           // Subject code (e.g., "MATH-SL")
	Initials      string `json:"initials"`       // Teacher's initials
	TeachingGroup string `json:"teaching_group"` // Teaching group identifier
	TeacherID     int    `json:"teacher_id"`     // Teacher's user ID
	TeacherName   string `json:"teacher_name"`   // Teacher's full name
}

/**
 * RegisterGetSubjectsRoute registers the route for fetching student subjects.
 *
 * Endpoint: GET /get_subjects/:student_id
 *
 * Parameters:
 *   - student_id: The ID of the student (integer)
 *
 * Returns:
 *   - 200 OK: Successfully retrieved subjects
 *     [
 *       {
 *         "subject": string,       // Subject name (e.g., "Mathematics SL")
 *         "code": string,          // Subject code (e.g., "MATH-SL")
 *         "initials": string,      // Teacher's initials
 *         "teaching_group": string, // Teaching group identifier
 *         "teacher_id": number,    // Teacher's user ID
 *         "teacher_name": string   // Teacher's full name
 *       }
 *     ]
 *   - 400 Bad Request: Invalid student_id format
 *   - 500 Internal Server Error: Database error
 */
func RegisterGetSubjectsRoute(router gin.IRouter, db *sql.DB) {
	router.GET("/get_subjects/:student_id", func(c *gin.Context) {
		// Retrieve the student ID from the URL parameters.
		studentIDStr := c.Param("student_id")
		studentID, err := strconv.Atoi(studentIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student_id"})
			return
		}

		// Query to fetch subject details for the given student_id.
		query := `
			SELECT subject, code, initials, teaching_group, teacher_id
			FROM subjects
			WHERE student_id = $1;
		`
		rows, err := db.Query(query, studentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query error"})
			return
		}
		defer rows.Close()

		// Regular expression to remove Chinese characters.
		re := regexp.MustCompile(`[\p{Han}]+`)

		subjects := []Subject{}

		// Process each row.
		for rows.Next() {
			var subjectName, code, initials, teachingGroup string
			var teacherID int
			if err := rows.Scan(&subjectName, &code, &initials, &teachingGroup, &teacherID); err != nil {
				continue
			}

			// Remove Chinese characters while keeping the English part.
			subjectName = strings.TrimSpace(re.ReplaceAllString(subjectName, ""))

			// Append " SL" or " HL" based on the subject code.
			if strings.HasSuffix(code, "SL") {
				subjectName += " SL"
			} else if strings.HasSuffix(code, "HL") {
				subjectName += " HL"
			}

			// Query to fetch teacher's first and last name.
			teacherQuery := "SELECT first_name, last_name FROM users WHERE id = $1 LIMIT 1;"
			var firstName, lastName string
			err := db.QueryRow(teacherQuery, teacherID).Scan(&firstName, &lastName)
			teacherName := "Unknown"
			if err == nil {
				teacherName = firstName + " " + lastName
			}

			subjects = append(subjects, Subject{
				Subject:       subjectName,
				Code:          code,
				Initials:      initials,
				TeachingGroup: teachingGroup,
				TeacherID:     teacherID,
				TeacherName:   teacherName,
			})
		}

		// Return the results as JSON.
		c.JSON(http.StatusOK, subjects)
	})
}
