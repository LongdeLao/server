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
	ID        int    `json:"id"`        // Student's user ID
	FirstName string `json:"name"`      // Student's first name
	LastName  string `json:"last_name"` // Student's last name
}

/**
 * AdminClass represents a class group with subject details.
 */
type AdminClass struct {
	SubjectName   string    `json:"subject_name"`   // Subject name (e.g., "Mathematics SL")
	Code          string    `json:"code"`           // Subject code (e.g., "MATH-SL")
	TeachingGroup string    `json:"teaching_group"` // Teaching group identifier
	Students      []Student `json:"students"`       // List of students in the class
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
 *         "subject_name": string,   // Subject name (e.g., "Mathematics SL")
 *         "code": string,           // Subject code (e.g., "MATH-SL")
 *         "teaching_group": string, // Teaching group identifier
 *         "students": [
 *           {
 *             "id": number,        // Student's user ID
 *             "name": string,      // Student's first name
 *             "last_name": string  // Student's last name
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

		// For debugging purposes, print the result like the Python version does.
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
 * @param db *sql.DB - Database connection
 * @param teacherID int - The ID of the teacher
 * @return []AdminClass - List of grouped subjects with student details
 * @return error - Any error that occurred during the process
 */
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
