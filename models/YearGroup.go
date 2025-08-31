package models

import "fmt"

// YearGroup represents a year group with a name and section
type YearGroup struct {
	Year     string `json:"year"`
	Section  string `json:"section"`
	FullName string `json:"fullName"`
}

// Student represents a student with year group information
type Student struct {
	UserID    int    `json:"user_id"`
	Name      string `json:"name"`
	Year      string `json:"year"`
	GroupName string `json:"group_name"`
	Today     string `json:"today"`
	Present   int    `json:"present"`
	Absent    int    `json:"absent"`
	Late      int    `json:"late"`
	Medical   int    `json:"medical"`
	Early     int    `json:"early"`
}

// AttendanceRecord represents the daily attendance for a class
type AttendanceRecord struct {
	Date      string    `json:"date"`
	YearGroup YearGroup `json:"yearGroup"`
	Students  []Student `json:"students"`
}

// GenerateYearGroups returns all combinations of year groups and sections
func GenerateYearGroups() []YearGroup {
	// Define the available year groups and sections
	years := []string{"PIB", "IB1"}
	sections := []string{"A", "B"}

	var yearGroups []YearGroup

	// Generate all combinations
	for _, year := range years {
		for _, section := range sections {
			fullName := fmt.Sprintf("%s %s", year, section)
			yearGroups = append(yearGroups, YearGroup{
				Year:     year,
				Section:  section,
				FullName: fullName,
			})
		}
	}

	return yearGroups
}

// GetYearGroupByID returns a year group based on a formatted ID string (e.g., "pib-a")
func GetYearGroupByID(id string) (YearGroup, bool) {
	// Map to translate IDs to year groups
	idToYearGroup := map[string]YearGroup{
		"pib-a": {Year: "PIB", Section: "A", FullName: "PIB A"},
		"pib-b": {Year: "PIB", Section: "B", FullName: "PIB B"},
		"ib1-a": {Year: "IB1", Section: "A", FullName: "IB1 A"},
		"ib1-b": {Year: "IB1", Section: "B", FullName: "IB1 B"},
	}

	yearGroup, exists := idToYearGroup[id]
	return yearGroup, exists
}

// GetStudentsByYearGroup returns students for a specific year group
// In a real application, this would query the database
func GetStudentsByYearGroup(yearGroup YearGroup) []Student {
	// This is a placeholder - in a real application,
	// you would query the database for students in this year group

	// Example mock implementation
	if yearGroup.Year == "PIB" && yearGroup.Section == "A" {
		return []Student{
			{UserID: 1001, Name: "Emma Johnson", Year: "PIB", GroupName: "A", Today: "Pending", Present: 42, Absent: 2, Late: 1},
			{UserID: 1002, Name: "Noah Williams", Year: "PIB", GroupName: "A", Today: "Pending", Present: 44, Absent: 0, Late: 1},
			// Add more students as needed
		}
	}

	// Return empty slice if no matches
	return []Student{}
}
