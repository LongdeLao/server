package notifications

import (
	"database/sql"
	"fmt"
	"log"
)

// SendMissingStudentReportNotification sends a push notification to the appropriate year group coordinator(s)
// when a student is reported missing
func SendMissingStudentReportNotification(db *sql.DB, studentID, reporterID int, studentName, yearGroup string) error {
	log.Printf("Sending missing student notification for %s (ID: %d) in year group %s", studentName, studentID, yearGroup)

	// Find the coordinators for this year group
	query := `
		SELECT u.id, u.name, u.device_id 
		FROM users u
		JOIN year_group_coordinators ygc ON u.id = ygc.user_id
		WHERE ygc.year_group = $1 AND u.device_id IS NOT NULL AND u.device_id != '' AND u.device_id != 'not-registered'
	`

	rows, err := db.Query(query, yearGroup)
	if err != nil {
		return fmt.Errorf("error querying year group coordinators: %v", err)
	}
	defer rows.Close()

	sentCount := 0
	for rows.Next() {
		var coordinatorID int
		var coordinatorName, deviceID string

		if err := rows.Scan(&coordinatorID, &coordinatorName, &deviceID); err != nil {
			log.Printf("Error scanning coordinator data: %v", err)
			continue
		}

		log.Printf("Sending notification to coordinator %s (ID: %d) with device ID: %s",
			coordinatorName, coordinatorID, deviceID)

		// Prepare notification content
		title := "Missing Student Report"
		body := fmt.Sprintf("%s has been reported missing from class by a staff member", studentName)
		data := map[string]string{
			"type":        "missing_student_report",
			"student_id":  fmt.Sprintf("%d", studentID),
			"reporter_id": fmt.Sprintf("%d", reporterID),
			"year_group":  yearGroup,
		}

		// Send the push notification
		if err := SendPushNotification(deviceID, title, body, data); err != nil {
			log.Printf("Failed to send notification to coordinator %d: %v", coordinatorID, err)
		} else {
			sentCount++
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating through coordinators: %v", err)
	}

	log.Printf("Sent notifications to %d coordinators for year group %s", sentCount, yearGroup)
	return nil
}

// SendMissingStudentResolvedNotification sends a push notification to the original reporter
// when a missing student case is resolved
func SendMissingStudentResolvedNotification(db *sql.DB, studentID, reporterID, resolverID int, studentName string) error {
	log.Printf("Sending resolution notification for student %s (ID: %d) to reporter (ID: %d)",
		studentName, studentID, reporterID)

	// Get the reporter's device ID
	var deviceID, reporterName string
	query := `SELECT device_id, name FROM users WHERE id = $1 AND device_id IS NOT NULL AND device_id != '' AND device_id != 'not-registered'`

	err := db.QueryRow(query, reporterID).Scan(&deviceID, &reporterName)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("reporter has no valid device ID or doesn't exist")
		}
		return fmt.Errorf("error querying reporter: %v", err)
	}

	// Get resolver's name
	var resolverName string
	err = db.QueryRow(`SELECT name FROM users WHERE id = $1`, resolverID).Scan(&resolverName)
	if err != nil {
		resolverName = "A staff member"
	}

	// Prepare notification content
	title := "Missing Student Update"
	body := fmt.Sprintf("%s has been located and is safe", studentName)
	if resolverName != "" {
		body = fmt.Sprintf("%s has been located and is safe (resolved by %s)", studentName, resolverName)
	}

	data := map[string]string{
		"type":        "missing_student_resolved",
		"student_id":  fmt.Sprintf("%d", studentID),
		"resolver_id": fmt.Sprintf("%d", resolverID),
	}

	// Send the push notification
	if err := SendPushNotification(deviceID, title, body, data); err != nil {
		return fmt.Errorf("failed to send notification to reporter %d: %v", reporterID, err)
	}

	log.Printf("Successfully sent resolution notification to reporter %s (ID: %d)", reporterName, reporterID)
	return nil
}
