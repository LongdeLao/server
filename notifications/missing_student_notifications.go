package notifications

import (
	"database/sql"
	"fmt"
	"log"
)

// SendMissingStudentReportNotification sends a push notification to the appropriate year group coordinator(s)
// when a student is reported missing
func SendMissingStudentReportNotification(db *sql.DB, studentID, reporterID int, studentName, yearGroup string) error {
	log.Printf("🚨 Preparing missing student report notification for %s (ID: %d) in year group %s", studentName, studentID, yearGroup)

	// Find the coordinators for this year group
	query := `
		SELECT u.id, u.name, u.device_id 
		FROM users u
		JOIN year_group_coordinators ygc ON u.id = ygc.user_id
		WHERE ygc.year_group = $1
	`

	log.Printf("🔍 Searching for year group coordinators for: %s", yearGroup)
	rows, err := db.Query(query, yearGroup)
	if err != nil {
		log.Printf("❌ Database error finding coordinators: %v", err)
		return err
	}
	defer rows.Close()

	// Track how many notifications were sent
	notificationsSent := 0
	coordinatorsFound := 0

	for rows.Next() {
		coordinatorsFound++
		var coordinatorID int
		var coordinatorName string
		var deviceID sql.NullString

		if err := rows.Scan(&coordinatorID, &coordinatorName, &deviceID); err != nil {
			log.Printf("❌ Error scanning coordinator data: %v", err)
			continue
		}

		log.Printf("🔍 Found coordinator: %s (ID: %d)", coordinatorName, coordinatorID)

		// Skip if this user has no device token
		if !deviceID.Valid || deviceID.String == "" {
			log.Printf("⚠️ Coordinator %s has no device token, skipping notification", coordinatorName)
			continue
		}

		// Prepare notification payload
		customData := map[string]string{
			"type":       "missing_student_report",
			"studentID":  fmt.Sprintf("%d", studentID),
			"reporterID": fmt.Sprintf("%d", reporterID),
			"yearGroup":  yearGroup,
		}

		// Send the notification
		title := "Missing Student Report"
		body := fmt.Sprintf("%s has been reported missing", studentName)

		log.Printf("📱 Sending notification to coordinator %s (device: %s)", coordinatorName, deviceID.String)
		if err := SendPushNotification(deviceID.String, title, body, customData); err != nil {
			log.Printf("❌ Failed to send notification to coordinator %s: %v", coordinatorName, err)
		} else {
			notificationsSent++
			log.Printf("✅ Successfully sent notification to coordinator %s", coordinatorName)
		}
	}

	if coordinatorsFound == 0 {
		log.Printf("⚠️ No coordinators found for year group: %s", yearGroup)
	}

	log.Printf("📊 Missing student report summary: %d coordinators found, %d notifications sent",
		coordinatorsFound, notificationsSent)

	return nil
}

// SendMissingStudentResolvedNotification sends a push notification to the reporter
// when a missing student case is resolved
func SendMissingStudentResolvedNotification(db *sql.DB, studentID, reporterID, resolverID int, studentName string) error {
	log.Printf("🚨 Preparing missing student resolution notification for %s (ID: %d), reporter ID: %d",
		studentName, studentID, reporterID)

	// Get the reporter's device token
	var deviceID sql.NullString
	var reporterName sql.NullString

	query := "SELECT device_id, name FROM users WHERE id = $1"
	log.Printf("🔍 Looking up reporter (ID: %d) device token", reporterID)

	err := db.QueryRow(query, reporterID).Scan(&deviceID, &reporterName)
	if err != nil {
		log.Printf("❌ Database error finding reporter: %v", err)
		return err
	}

	// Skip if this user has no device token
	if !deviceID.Valid || deviceID.String == "" {
		log.Printf("⚠️ Reporter %s has no device token, cannot send notification",
			reporterName.String)
		return fmt.Errorf("reporter has no device token")
	}

	log.Printf("🔍 Found reporter: %s with device token", reporterName.String)

	// Get resolver name if available
	var resolverName string = "Staff"
	if resolverID > 0 {
		err := db.QueryRow("SELECT name FROM users WHERE id = $1", resolverID).Scan(&resolverName)
		if err != nil {
			log.Printf("⚠️ Could not get resolver name: %v, using 'Staff'", err)
		} else {
			log.Printf("🔍 Found resolver: %s", resolverName)
		}
	}

	// Prepare notification payload
	customData := map[string]string{
		"type":         "missing_student_resolved",
		"studentID":    fmt.Sprintf("%d", studentID),
		"reporterID":   fmt.Sprintf("%d", reporterID),
		"resolverID":   fmt.Sprintf("%d", resolverID),
		"resolverName": resolverName,
	}

	// Send the notification
	title := "Missing Student Update"
	body := fmt.Sprintf("%s has been found and is safe", studentName)

	log.Printf("📱 Sending resolution notification to reporter %s (device: %s)",
		reporterName.String, deviceID.String)

	if err := SendPushNotification(deviceID.String, title, body, customData); err != nil {
		log.Printf("❌ Failed to send resolution notification: %v", err)
		return err
	}

	log.Printf("✅ Successfully sent resolution notification to reporter %s", reporterName.String)
	return nil
}
