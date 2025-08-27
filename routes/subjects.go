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
	Subject       string `json:"subject"`
	Code          string `json:"code"`
	Initials      string `json:"initials"`
	TeachingGroup string `json:"teaching_group"`
	TeacherID     int    `json:"teacher_id"`
	TeacherName   string `json:"teacher_name"`
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
 *         "subject": string,
 *         "code": string,
 *         "initials": string,
 *         "teaching_group": string,
 *         "teacher_id": number,
 *         "teacher_name": string
 *       }
 *     ]
 *   - 400 Bad Request: Invalid student_id format
 *   - 500 Internal Server Error: Database error
 */
func RegisterGetSubjectsRoute(router gin.IRouter, db *sql.DB) {
	router.GET("/get_subjects/:student_id", func(c *gin.Context) {
		studentIDStr := c.Param("student_id")
		studentID, err := strconv.Atoi(studentIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student_id"})
			return
		}

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

		re := regexp.MustCompile(`[\p{Han}]+`)

		subjects := []Subject{}

		for rows.Next() {
			var subjectName, code, initials, teachingGroup string
			var teacherID int
			if err := rows.Scan(&subjectName, &code, &initials, &teachingGroup, &teacherID); err != nil {
				continue
			}

			subjectName = strings.TrimSpace(re.ReplaceAllString(subjectName, ""))

			if strings.HasSuffix(code, "SL") {
				subjectName += " SL"
			} else if strings.HasSuffix(code, "HL") {
				subjectName += " HL"
			}

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

		c.JSON(http.StatusOK, subjects)
	})
}
