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

// Student represents a student's basic details.
type Student struct {
	ID        int    `json:"id"`
	FirstName string `json:"name"`      // Matching Python's "name" key.
	LastName  string `json:"last_name"` // Matching Python's "last_name" key.
}

// AdminClass represents the grouped subject details.
type AdminClass struct {
	SubjectName   string    `json:"subject_name"`
	Code          string    `json:"code"`
	TeachingGroup string    `json:"teaching_group"`
	Students      []Student `json:"students"`
}

// RegisterGetSubjectsTeacherRoute registers the route for fetching subjects by teacher.
func RegisterGetSubjectsTeacherRoute(router *gin.Engine, db *sql.DB) {
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

		// For debugging purposes, print the result like the Python version does.
		log.Printf("Admin Classes: %+v\n", adminClasses)
		c.JSON(http.StatusOK, adminClasses)
	})
}

// getSubjectsByTeacher queries the database and groups subjects along with student details.
// It mimics the Python version exactly: performing a query to fetch subject details,
// then for each row fetching student details, grouping by subject, code, teaching group,
// cleaning the subject name, and appending " SL" or " HL" if needed.
func getSubjectsByTeacher(db *sql.DB, teacherID int) ([]AdminClass, error) {
	// Query to get subject, code, teaching_group, and student_id.
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

	// Grouping: subject -> code -> teaching_group -> []Student.
	// This mirrors the Python defaultdict(lambda: defaultdict(lambda: defaultdict(list))).
	subjectGroups := make(map[string]map[string]map[string][]Student)

	for rows.Next() {
		var subjectName, code, teachingGroup string
		var studentID int

		if err := rows.Scan(&subjectName, &code, &teachingGroup, &studentID); err != nil {
			return nil, fmt.Errorf("row scan error: %v", err)
		}

		// Query to fetch student details by student_id.
		studentQuery := `SELECT name, last_name FROM users WHERE id = $1;`
		var firstName, lastName string
		err = db.QueryRow(studentQuery, studentID).Scan(&firstName, &lastName)
		if err != nil {
			log.Printf("Error fetching student with id %d: %v", studentID, err)
			continue // Skip this student if not found.
		}

		student := Student{
			ID:        studentID,
			FirstName: firstName,
			LastName:  lastName,
		}

		// Initialize nested maps if needed.
		if _, exists := subjectGroups[subjectName]; !exists {
			subjectGroups[subjectName] = make(map[string]map[string][]Student)
		}
		if _, exists := subjectGroups[subjectName][code]; !exists {
			subjectGroups[subjectName][code] = make(map[string][]Student)
		}
		subjectGroups[subjectName][code][teachingGroup] = append(subjectGroups[subjectName][code][teachingGroup], student)
	}

	// Process grouped data into the desired structure.
	var result []AdminClass
	// Compile a regexp to remove everything after the first space.
	re := regexp.MustCompile(`\s.*`)
	for subjectName, codes := range subjectGroups {
		for code, groups := range codes {
			for teachingGroup, students := range groups {
				// Remove the Chinese part (or everything after the first space).
				cleanSubject := re.ReplaceAllString(subjectName, "")
				// Append " SL" or " HL" based on code suffix.
				if strings.HasSuffix(code, "SL") {
					cleanSubject += " SL"
				} else if strings.HasSuffix(code, "HL") {
					cleanSubject += " HL"
				}

				// Build the admin class record matching the Python dictionary.
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
