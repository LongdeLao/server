package routes

import (
	"database/sql"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Subject represents the subject data returned as JSON.
type Subject struct {
	Subject       string `json:"subject"`
	Code          string `json:"code"`
	Initials      string `json:"initials"`
	TeachingGroup string `json:"teaching_group"`
	TeacherID     int    `json:"teacher_id"`
	TeacherName   string `json:"teacher_name"`
}

// RegisterGetSubjectsRoute registers the /get_subjects/:student_id endpoint.
func RegisterGetSubjectsRoute(router *gin.Engine, db *sql.DB) {
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
