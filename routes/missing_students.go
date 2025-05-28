package routes

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// MissingStudentReport represents a report of a missing student
type MissingStudentReport struct {
	ID           int        `json:"id"`
	StudentID    int        `json:"student_id"`
	StudentName  string     `json:"student_name"`
	ReportedBy   int        `json:"reported_by"`
	ReporterName string     `json:"reporter_name"`
	YearGroup    string     `json:"year_group"`
	ReportDate   string     `json:"report_date"`
	ReportTime   string     `json:"report_time"`
	Status       string     `json:"status"`
	Notes        string     `json:"notes"`
	ResolvedBy   *int       `json:"resolved_by,omitempty"`
	ResolverName *string    `json:"resolver_name,omitempty"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// YearGroupCoordinator represents a staff member assigned to a year group
type YearGroupCoordinator struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Name      string    `json:"name"`
	YearGroup string    `json:"year_group"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReportMissingStudentHandler handles the reporting of a missing student
//
// Endpoint: POST /api/missing-students/report
//
// Request Body:
//
//	{
//	  "student_id": int,
//	  "notes": string (optional)
//	}
//
// Returns:
//   - 200 OK: Successfully reported missing student
//   - 400 Bad Request: Invalid request format or data
//   - 403 Forbidden: User is not authorized to report for this year group
//   - 500 Internal Server Error: Database error
func ReportMissingStudentHandler(c *gin.Context, db *sql.DB) {
	var request struct {
		StudentID int    `json:"student_id" binding:"required"`
		Notes     string `json:"notes"`
	}

	fmt.Printf("📝 [MissingStudentReport] Received report request: %v\n", c.Request.URL.String())

	if err := c.ShouldBindJSON(&request); err != nil {
		fmt.Printf("❌ [MissingStudentReport] Invalid request format: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format",
			"error":   err.Error(),
		})
		return
	}

	fmt.Printf("📋 [MissingStudentReport] Request data: student_id=%d, notes_length=%d\n",
		request.StudentID, len(request.Notes))

	// Get the reporting staff member's ID from the request header or query param
	reporterIDStr := c.Query("reporter_id")
	if reporterIDStr == "" {
		fmt.Printf("❌ [MissingStudentReport] Missing reporter_id parameter\n")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Reporter ID is required",
		})
		return
	}

	reporterID, err := strconv.Atoi(reporterIDStr)
	if err != nil {
		fmt.Printf("❌ [MissingStudentReport] Invalid reporter_id format: %s\n", reporterIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid reporter ID format",
		})
		return
	}

	fmt.Printf("👤 [MissingStudentReport] Reporter ID: %d\n", reporterID)

	// Check if the reporter is a staff member
	var isStaff bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'staff')", reporterID).Scan(&isStaff)
	if err != nil {
		fmt.Printf("❌ [MissingStudentReport] Error checking reporter role: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking reporter role",
			"error":   err.Error(),
		})
		return
	}

	if !isStaff {
		fmt.Printf("⚠️ [MissingStudentReport] Reporter %d is not a staff member\n", reporterID)
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Only staff members can report missing students",
		})
		return
	}

	fmt.Printf("✅ [MissingStudentReport] Reporter %d is a staff member\n", reporterID)

	// Check if the student exists and get their year group
	var studentExists bool
	var studentName, studentYearGroup string

	studentQuery := `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'student'),
		       COALESCE(users.name, ''),
		       COALESCE((SELECT year FROM attendance WHERE user_id = $1), '')
		FROM users
		WHERE users.id = $1
	`
	fmt.Printf("🔍 [MissingStudentReport] Checking student %d with query: %s\n", request.StudentID, studentQuery)

	err = db.QueryRow(studentQuery, request.StudentID).Scan(&studentExists, &studentName, &studentYearGroup)

	if err != nil {
		fmt.Printf("❌ [MissingStudentReport] Error checking student information: %v\n", err)

		// Additional error details
		var exists bool
		errCheck := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", request.StudentID).Scan(&exists)
		if errCheck != nil {
			fmt.Printf("❌ [MissingStudentReport] Additional error checking if user exists: %v\n", errCheck)
		} else {
			fmt.Printf("🔍 [MissingStudentReport] User exists in users table: %v\n", exists)
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking student information",
			"error":   err.Error(),
		})
		return
	}

	fmt.Printf("👨‍🎓 [MissingStudentReport] Student: exists=%v, name=%s, yearGroup=%s\n",
		studentExists, studentName, studentYearGroup)

	if !studentExists {
		fmt.Printf("⚠️ [MissingStudentReport] Student %d not found or not a student\n", request.StudentID)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Student not found or not a student",
		})
		return
	}

	// Since we've confirmed the reporter is a staff member, we allow them to report missing students
	// regardless of year group assignment or additional roles
	fmt.Printf("✅ [MissingStudentReport] Staff member is authorized to report missing students\n")

	// Check if the student is already reported as missing and not resolved
	var alreadyReported bool
	alreadyReportedQuery := `
		SELECT EXISTS(
			SELECT 1 FROM missing_students
			WHERE student_id = $1 AND status = 'reported'
		)
	`
	fmt.Printf("🔍 [MissingStudentReport] Checking existing reports with query: %s\n", alreadyReportedQuery)

	err = db.QueryRow(alreadyReportedQuery, request.StudentID).Scan(&alreadyReported)

	if err != nil {
		fmt.Printf("❌ [MissingStudentReport] Error checking existing reports: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking existing reports",
			"error":   err.Error(),
		})
		return
	}

	fmt.Printf("✅ [MissingStudentReport] Student already reported: %v\n", alreadyReported)

	if alreadyReported {
		fmt.Printf("⚠️ [MissingStudentReport] Student %d already reported as missing\n", request.StudentID)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This student is already reported as missing",
		})
		return
	}

	// Insert the missing student report
	var reportID int
	insertQuery := `
		INSERT INTO missing_students 
		(student_id, reported_by, year_group, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	fmt.Printf("📝 [MissingStudentReport] Inserting report with query: %s\n", insertQuery)
	fmt.Printf("📝 [MissingStudentReport] Values: student_id=%d, reported_by=%d, year_group=%s, notes_length=%d\n",
		request.StudentID, reporterID, studentYearGroup, len(request.Notes))

	err = db.QueryRow(insertQuery, request.StudentID, reporterID, studentYearGroup, request.Notes).Scan(&reportID)

	if err != nil {
		fmt.Printf("❌ [MissingStudentReport] Error creating missing student report: %v\n", err)

		// Check if the missing_students table exists
		var tableExists bool
		errCheck := db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'missing_students')").Scan(&tableExists)
		if errCheck != nil {
			fmt.Printf("❌ [MissingStudentReport] Error checking if table exists: %v\n", errCheck)
		} else {
			fmt.Printf("🔍 [MissingStudentReport] missing_students table exists: %v\n", tableExists)
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error creating missing student report",
			"error":   err.Error(),
		})
		return
	}

	fmt.Printf("✅ [MissingStudentReport] Successfully created report with ID: %d\n", reportID)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Missing student reported successfully",
		"report_id": reportID,
	})
}

// GetMissingStudentsHandler retrieves the list of missing students
//
// Endpoint: GET /api/missing-students
//
// Query Parameters:
//   - status: optional filter by status (e.g., "reported", "resolved")
//   - year_group: optional filter by year group (e.g., "PIB", "IB1")
//   - user_id: ID of the user making the request (for permission checks)
//   - reporter_id: optional filter to specifically include reports made by this user ID
//
// Returns:
//   - 200 OK: List of missing student reports
//   - 403 Forbidden: User is not authorized to view this data
//   - 500 Internal Server Error: Database error
func GetMissingStudentsHandler(c *gin.Context, db *sql.DB) {
	reports := []MissingStudentReport{}

	statusFilter := c.Query("status")
	yearGroupFilter := c.Query("year_group")
	requestingUserIDStr := c.Query("user_id")
	reporterIDFilterStr := c.Query("reporter_id") // To fetch reports specifically made by this user

	if requestingUserIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "User ID is required"})
		return
	}
	requestingUserID, err := strconv.Atoi(requestingUserIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID format"})
		return
	}

	var isStaff bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'staff')", requestingUserID).Scan(&isStaff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error checking user role", "error": err.Error()})
		return
	}
	if !isStaff {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Only staff members can view missing students"})
		return
	}

	var hasAttendanceRole bool
	err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM additional_roles WHERE user_id = $1 AND role = 'attendance')`, requestingUserID).Scan(&hasAttendanceRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error checking attendance role", "error": err.Error()})
		return
	}

	baseQuery := `
		SELECT 
			m.id, m.student_id, student.name as student_name,
			m.reported_by, reporter.name as reporter_name, m.year_group, 
			TO_CHAR(m.report_date, 'YYYY-MM-DD') as report_date, 
			TO_CHAR(m.report_time, 'HH24:MI:SS') as report_time, 
			m.status, m.notes, m.resolved_by, resolver.name as resolver_name,
			m.resolved_at, m.created_at, m.updated_at
		FROM missing_students m
		JOIN users student ON m.student_id = student.id
		JOIN users reporter ON m.reported_by = reporter.id
		LEFT JOIN users resolver ON m.resolved_by = resolver.id
	`
	var conditions []string
	var args []interface{}
	argCount := 1

	// --- Main Visibility Logic ---
	var mainVisibilityClauses []string

	// Clause 1: Permission-based visibility (attendance admin or year group coordinator)
	var permissionBasedSubClauses []string
	if hasAttendanceRole {
		if yearGroupFilter != "" {
			permissionBasedSubClauses = append(permissionBasedSubClauses, fmt.Sprintf("m.year_group = $%d", argCount))
			args = append(args, yearGroupFilter)
			argCount++
		} else {
			permissionBasedSubClauses = append(permissionBasedSubClauses, "1=1") // Sees all year groups
		}
	} else { // Not an attendance admin, restrict by year group coordination
		if yearGroupFilter != "" {
			var canViewThisYearGroup bool
			errDb := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM year_group_coordinators WHERE user_id = $1 AND year_group = $2)`, requestingUserID, yearGroupFilter).Scan(&canViewThisYearGroup)
			if errDb != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error checking year group permissions", "error": errDb.Error()})
				return
			}
			if canViewThisYearGroup {
				permissionBasedSubClauses = append(permissionBasedSubClauses, fmt.Sprintf("m.year_group = $%d", argCount))
				args = append(args, yearGroupFilter)
				argCount++
			} else {
				permissionBasedSubClauses = append(permissionBasedSubClauses, "1=0") // Not authorized for this specific year group
			}
		} else { // No specific year group filter, so show all reports from year_groups they coordinate
			permissionBasedSubClauses = append(permissionBasedSubClauses, fmt.Sprintf("m.year_group IN (SELECT year_group FROM year_group_coordinators WHERE user_id = $%d)", argCount))
			args = append(args, requestingUserID)
			argCount++
		}
	}
	if len(permissionBasedSubClauses) > 0 {
		mainVisibilityClauses = append(mainVisibilityClauses, "("+strings.Join(permissionBasedSubClauses, " AND ")+")")
	}

	// Clause 2: User sees their own reports (if reporterIDFilterStr is provided and matches requestingUserID)
	if reporterIDFilterStr != "" {
		reporterID, errConv := strconv.Atoi(reporterIDFilterStr)
		if errConv == nil && reporterID == requestingUserID { // Check if the filter is for the current user
			mainVisibilityClauses = append(mainVisibilityClauses, fmt.Sprintf("m.reported_by = $%d", argCount))
			args = append(args, reporterID)
			argCount++
		}
	}

	if len(mainVisibilityClauses) > 0 {
		conditions = append(conditions, "("+strings.Join(mainVisibilityClauses, " OR ")+")")
	} else {
		// Should ideally not happen if a staff user always has some visibility or is fetching their own.
		// Default to no reports if no visibility clauses are met.
		conditions = append(conditions, "1=0")
	}

	// --- Status filter (applies to all visible reports) ---
	if statusFilter != "" {
		conditions = append(conditions, fmt.Sprintf("m.status = $%d", argCount))
		args = append(args, statusFilter)
		argCount++
	}

	fullQuery := baseQuery
	if len(conditions) > 0 {
		fullQuery += " WHERE " + strings.Join(conditions, " AND ")
	} else {
		// Fallback, though ideally the logic above ensures `conditions` is not empty.
		fullQuery += " WHERE 1=0" // No reports if no conditions met
	}

	fullQuery += " ORDER BY m.created_at DESC"

	fmt.Printf("Query: %s\nArgs: %v\n", fullQuery, args)

	rows, err := db.Query(fullQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error retrieving missing students", "error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var report MissingStudentReport
		var resolverName sql.NullString
		var resolvedAt sql.NullTime
		var resolvedBy sql.NullInt64

		errScan := rows.Scan(
			&report.ID, &report.StudentID, &report.StudentName,
			&report.ReportedBy, &report.ReporterName, &report.YearGroup,
			&report.ReportDate, &report.ReportTime, &report.Status, &report.Notes,
			&resolvedBy, &resolverName, &resolvedAt,
			&report.CreatedAt, &report.UpdatedAt,
		)

		if errScan != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error scanning missing student report", "error": errScan.Error()})
			return
		}

		if resolvedBy.Valid {
			id := int(resolvedBy.Int64)
			report.ResolvedBy = &id
		}
		if resolverName.Valid {
			name := resolverName.String
			report.ResolverName = &name
		}
		if resolvedAt.Valid {
			report.ResolvedAt = &resolvedAt.Time
		}
		reports = append(reports, report)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error iterating through reports", "error": err.Error()})
		return
	}

	if reports == nil { // Should be initialized to empty slice, but as a safeguard
		reports = []MissingStudentReport{}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "reports": reports, "count": len(reports)})
}

// ResolveMissingStudentHandler marks a missing student as found
//
// Endpoint: POST /api/missing-students/resolve/:id
//
// Request Body:
//
//	{
//	  "notes": string (optional)
//	}
//
// Returns:
//   - 200 OK: Successfully resolved missing student report
//   - 400 Bad Request: Invalid report ID
//   - 403 Forbidden: User is not authorized to resolve this report
//   - 404 Not Found: Report not found or already resolved
//   - 500 Internal Server Error: Database error
func ResolveMissingStudentHandler(c *gin.Context, db *sql.DB) {
	reportIDStr := c.Param("id")
	reportID, err := strconv.Atoi(reportIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid report ID format",
		})
		return
	}

	// Get the resolving staff member's ID
	resolverIDStr := c.Query("resolver_id")
	if resolverIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Resolver ID is required",
		})
		return
	}

	resolverID, err := strconv.Atoi(resolverIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid resolver ID format",
		})
		return
	}

	var request struct {
		Notes string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		// Notes are optional, so we can proceed with an empty request
		request.Notes = ""
	}

	// Check if the resolver is a staff member
	var isStaff bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'staff')", resolverID).Scan(&isStaff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking resolver role",
			"error":   err.Error(),
		})
		return
	}

	if !isStaff {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Only staff members can resolve missing student reports",
		})
		return
	}

	// Check if the report exists and is not resolved
	var exists bool
	var reportStatus string
	var yearGroup string
	var studentID int

	err = db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM missing_students WHERE id = $1),
		       (SELECT status FROM missing_students WHERE id = $1),
		       (SELECT year_group FROM missing_students WHERE id = $1),
		       (SELECT student_id FROM missing_students WHERE id = $1)
	`, reportID).Scan(&exists, &reportStatus, &yearGroup, &studentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking report status",
			"error":   err.Error(),
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Missing student report not found",
		})
		return
	}

	if reportStatus != "reported" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This report has already been resolved",
		})
		return
	}

	// Check if the resolver has the attendance role or is assigned to the year group
	var hasAttendanceRole bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM additional_roles 
			WHERE user_id = $1 AND role = 'attendance'
		)
	`, resolverID).Scan(&hasAttendanceRole)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking attendance role",
			"error":   err.Error(),
		})
		return
	}

	if !hasAttendanceRole {
		// Check if assigned to the year group
		var canResolve bool
		err = db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM year_group_coordinators
				WHERE user_id = $1 AND year_group = $2
			)
		`, resolverID, yearGroup).Scan(&canResolve)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error checking year group assignment",
				"error":   err.Error(),
			})
			return
		}

		if !canResolve {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "You are not authorized to resolve reports for this year group",
			})
			return
		}
	}

	// Update the report status to resolved
	_, err = db.Exec(`
		UPDATE missing_students
		SET status = 'resolved',
		    resolved_by = $1,
		    resolved_at = CURRENT_TIMESTAMP,
		    notes = CASE WHEN $2 <> '' THEN COALESCE(notes, '') || E'\n' || $2 ELSE notes END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`, resolverID, request.Notes, reportID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error resolving missing student report",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Missing student report resolved successfully",
	})
}

// GetYearGroupCoordinatorsHandler retrieves all staff members assigned to year groups
//
// Endpoint: GET /api/missing-students/coordinators
//
// Returns:
//   - 200 OK: List of year group coordinators
//   - 403 Forbidden: User is not authorized to view this data
//   - 500 Internal Server Error: Database error
func GetYearGroupCoordinatorsHandler(c *gin.Context, db *sql.DB) {
	// Get the requesting user's ID
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "User ID is required",
		})
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID format",
		})
		return
	}

	// Check if the user is a staff member
	var isStaff bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'staff')", userID).Scan(&isStaff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking user role",
			"error":   err.Error(),
		})
		return
	}

	if !isStaff {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Only staff members can view year group coordinators",
		})
		return
	}

	// Query all year group coordinators
	query := `
		SELECT 
			y.id, 
			y.user_id, 
			u.name, 
			y.year_group, 
			y.created_at, 
			y.updated_at
		FROM year_group_coordinators y
		JOIN users u ON y.user_id = u.id
		ORDER BY y.year_group, u.name
	`

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error retrieving year group coordinators",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	// Process the results
	var coordinators []YearGroupCoordinator
	for rows.Next() {
		var coordinator YearGroupCoordinator

		err := rows.Scan(
			&coordinator.ID,
			&coordinator.UserID,
			&coordinator.Name,
			&coordinator.YearGroup,
			&coordinator.CreatedAt,
			&coordinator.UpdatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error scanning year group coordinator",
				"error":   err.Error(),
			})
			return
		}

		coordinators = append(coordinators, coordinator)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error iterating through coordinators",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"coordinators": coordinators,
		"count":        len(coordinators),
	})
}

// SetupMissingStudentsRoutes registers all the missing students routes
func SetupMissingStudentsRoutes(router gin.IRouter, db *sql.DB) {
	missingStudentsGroup := router.Group("/missing-students")
	{
		missingStudentsGroup.POST("/report", func(c *gin.Context) {
			ReportMissingStudentHandler(c, db)
		})

		missingStudentsGroup.GET("", func(c *gin.Context) {
			GetMissingStudentsHandler(c, db)
		})

		missingStudentsGroup.POST("/resolve/:id", func(c *gin.Context) {
			ResolveMissingStudentHandler(c, db)
		})

		missingStudentsGroup.POST("/cancel/:id", func(c *gin.Context) {
			CancelMissingStudentReportHandler(c, db)
		})

		missingStudentsGroup.GET("/coordinators", func(c *gin.Context) {
			GetYearGroupCoordinatorsHandler(c, db)
		})
	}

	// Add student search endpoint
	router.GET("/students/search", func(c *gin.Context) {
		SearchStudentsHandler(c, db)
	})

	// Add endpoint to get all students at once for client-side searching
	router.GET("/attendance/all-students", func(c *gin.Context) {
		GetAllStudentsForAttendanceHandler(c, db)
	})
}

// SearchStudentsHandler searches for students by name
//
// Endpoint: GET /api/students/search
//
// Query Parameters:
//   - query: The search query string (minimum 2 characters)
//
// Returns:
//   - 200 OK: List of matching students
//   - 400 Bad Request: Invalid or missing query parameter
//   - 500 Internal Server Error: Database error
func SearchStudentsHandler(c *gin.Context, db *sql.DB) {
	// Get search query from request
	query := c.Query("query")
	if query == "" || len(query) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Search query must be at least 2 characters",
		})
		return
	}

	// Search for students with names containing the query string
	searchQuery := `
		SELECT u.id, COALESCE(u.name, ''), COALESCE(a.year || ' ' || a.group_name, '')
		FROM users u
		LEFT JOIN attendance a ON u.id = a.user_id
		WHERE u.role = 'student' AND u.name ILIKE '%' || $1 || '%'
		ORDER BY u.name
		LIMIT 20
	`

	rows, err := db.Query(searchQuery, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error searching for students",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	// Process the results
	var students []map[string]interface{}
	for rows.Next() {
		var id int
		var name, yearGroup string

		err := rows.Scan(&id, &name, &yearGroup)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error scanning student data",
				"error":   err.Error(),
			})
			return
		}

		students = append(students, map[string]interface{}{
			"user_id":    id,
			"name":       name,
			"year_group": yearGroup,
		})
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error iterating through students",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, students)
}

// GetAllStudentsForAttendanceHandler retrieves all students for client-side filtering
//
// Endpoint: GET /api/attendance/all-students
//
// Returns:
//   - 200 OK: List of all students with basic information
//   - 500 Internal Server Error: Database error
func GetAllStudentsForAttendanceHandler(c *gin.Context, db *sql.DB) {
	// Query to get all students with their year groups
	query := `
		SELECT u.id, COALESCE(u.name, ''), COALESCE(a.year, ''), COALESCE(a.group_name, '')
		FROM users u
		LEFT JOIN attendance a ON u.id = a.user_id
		WHERE u.role = 'student'
		ORDER BY u.name
	`

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error retrieving students",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	// Process the results
	var students []map[string]interface{}
	for rows.Next() {
		var id int
		var name, year, groupName string

		err := rows.Scan(&id, &name, &year, &groupName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error scanning student data",
				"error":   err.Error(),
			})
			return
		}

		students = append(students, map[string]interface{}{
			"user_id":    id,
			"name":       name,
			"year":       year,
			"group_name": groupName,
		})
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error iterating through students",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"students": students,
	})
}

// CancelMissingStudentReportHandler cancels a missing student report
//
// Endpoint: POST /api/missing-students/cancel/:id
//
// Request Body:
//
//	{
//	  "notes": string (optional)
//	}
//
// Returns:
//   - 200 OK: Successfully cancelled missing student report
//   - 400 Bad Request: Invalid report ID
//   - 403 Forbidden: User is not authorized to cancel this report
//   - 404 Not Found: Report not found or already resolved
//   - 500 Internal Server Error: Database error
func CancelMissingStudentReportHandler(c *gin.Context, db *sql.DB) {
	reportIDStr := c.Param("id")
	reportID, err := strconv.Atoi(reportIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid report ID format",
		})
		return
	}

	// Get the requesting staff member's ID
	requesterIDStr := c.Query("requester_id")
	if requesterIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Requester ID is required",
		})
		return
	}

	requesterID, err := strconv.Atoi(requesterIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid requester ID format",
		})
		return
	}

	var request struct {
		Notes string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		// Notes are optional, so we can proceed with an empty request
		request.Notes = ""
	}

	// Check if the requester is a staff member
	var isStaff bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'staff')", requesterID).Scan(&isStaff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking requester role",
			"error":   err.Error(),
		})
		return
	}

	if !isStaff {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Only staff members can cancel missing student reports",
		})
		return
	}

	// Check if the report exists and is not resolved
	var exists bool
	var reportStatus string
	var reportedByID int

	err = db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM missing_students WHERE id = $1),
		       (SELECT status FROM missing_students WHERE id = $1),
		       (SELECT reported_by FROM missing_students WHERE id = $1)
	`, reportID).Scan(&exists, &reportStatus, &reportedByID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error checking report status",
			"error":   err.Error(),
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Missing student report not found",
		})
		return
	}

	if reportStatus != "reported" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This report has already been resolved or cancelled",
		})
		return
	}

	// Check if the requester is the one who reported the student
	if reportedByID != requesterID {
		// Check if they have attendance admin role
		var hasAttendanceRole bool
		err = db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM additional_roles 
				WHERE user_id = $1 AND role = 'attendance'
			)
		`, requesterID).Scan(&hasAttendanceRole)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error checking attendance role",
				"error":   err.Error(),
			})
			return
		}

		if !hasAttendanceRole {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "You can only cancel reports that you submitted, unless you have attendance admin privileges",
			})
			return
		}
	}

	// Update the report status to cancelled
	_, err = db.Exec(`
		UPDATE missing_students
		SET status = 'cancelled',
		    resolved_by = $1,
		    resolved_at = CURRENT_TIMESTAMP,
		    notes = CASE WHEN $2 <> '' THEN COALESCE(notes, '') || E'\n' || $2 ELSE notes END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`, requesterID, request.Notes, reportID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error cancelling missing student report",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Missing student report cancelled successfully",
	})
}
