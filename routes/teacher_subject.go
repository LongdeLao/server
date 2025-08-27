package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

/**
 * Student represents a student's basic details in the teacher's subject view.
 */
type Student struct {
	ID        int    `json:"id"`
	FirstName string `json:"name"`
	LastName  string `json:"last_name"`
}

/**
 * AdminClass represents a class group with subject details.
 */
type AdminClass struct {
	SubjectName   string    `json:"subject_name"`
	Code          string    `json:"code"`
	TeachingGroup string    `json:"teaching_group"`
	Students      []Student `json:"students"`
}

/**
 * RegisterGetSubjectsTeacherRoute registers the route for fetching subjects by teacher.
 *
 * Endpoint: GET /get_subjects_by_teacher/:teacher_id
 *
 * Parameters:
 *   - teacher_id: The ID of the teacher (integer)
 *
 * Returns:
 *   - 200 OK: Successfully retrieved teacher's subjects
 *     [
 *       {
 *         "subject_name": string,
 *         "code": string,
 *         "teaching_group": string,
 *         "students": [
 *           {
 *             "id": number,
 *             "name": string,
 *             "last_name": string
 *           }
 *         ]
 *       }
 *     ]
 *   - 400 Bad Request: Invalid teacher_id format
 *   - 500 Internal Server Error: Database error
 */
func RegisterGetSubjectsTeacherRoute(router gin.IRouter, db *sql.DB) {
	router.GET("/get_subjects_by_teacher/:teacher_id", func(c *gin.Context) {
		teacherIDParam := c.Param("teacher_id")
		var teacherID int
		_, err := fmt.Sscanf(teacherIDParam, "%d", &teacherID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid teacher ID"})
			return
		}

		adminClasses, err := getSubjectsByTeacher(db, teacherID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Admin Classes: %+v\n", adminClasses)
		c.JSON(http.StatusOK, adminClasses)
	})
}

/**
 * getSubjectsByTeacher queries the database and groups subjects with student details.
 *
 * This function:
 * 1. Fetches all subjects taught by the teacher
 * 2. For each subject, retrieves the list of students
 * 3. Groups the data by subject, code, and teaching group
 * 4. Cleans subject names and adds SL/HL suffixes
 *
 * Parameters:
 *   - db: Database connection
 *   - teacherID: The ID of the teacher
 *
 * Returns:
 *   - []AdminClass: List of grouped subjects with student details
 *   - error: Any error that occurred during the process
 */
func getSubjectsByTeacher(db *sql.DB, teacherID int) ([]AdminClass, error) {
	query := `
		SELECT DISTINCT s.subject, s.code, s.teaching_group, s.student_id
		FROM subjects s
		WHERE s.teacher_id = $1
		ORDER BY s.subject, s.code, s.teaching_group;`

	rows, err := db.Query(query, teacherID)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	subjectGroups := make(map[string]map[string]map[string][]Student)

	for rows.Next() {
		var subjectName, code, teachingGroup string
		var studentID int

		if err := rows.Scan(&subjectName, &code, &teachingGroup, &studentID); err != nil {
			return nil, fmt.Errorf("row scan error: %v", err)
		}

		studentQuery := `SELECT name, last_name FROM users WHERE id = $1;`
		var firstName, lastName string
		err = db.QueryRow(studentQuery, studentID).Scan(&firstName, &lastName)
		if err != nil {
			log.Printf("Error fetching student with id %d: %v", studentID, err)
			continue
		}

		student := Student{
			ID:        studentID,
			FirstName: firstName,
			LastName:  lastName,
		}

		if _, exists := subjectGroups[subjectName]; !exists {
			subjectGroups[subjectName] = make(map[string]map[string][]Student)
		}
		if _, exists := subjectGroups[subjectName][code]; !exists {
			subjectGroups[subjectName][code] = make(map[string][]Student)
		}
		subjectGroups[subjectName][code][teachingGroup] = append(subjectGroups[subjectName][code][teachingGroup], student)
	}

	var result []AdminClass
	re := regexp.MustCompile(`\s.*`)
	for subjectName, codes := range subjectGroups {
		for code, groups := range codes {
			for teachingGroup, students := range groups {
				cleanSubject := re.ReplaceAllString(subjectName, "")
				if strings.HasSuffix(code, "SL") {
					cleanSubject += " SL"
				} else if strings.HasSuffix(code, "HL") {
					cleanSubject += " HL"
				}

				adminClass := AdminClass{
					SubjectName:   cleanSubject,
					Code:          code,
					TeachingGroup: teachingGroup,
					Students:      students,
				}
				result = append(result, adminClass)
			}
		}
	}
	return result, nil
}
