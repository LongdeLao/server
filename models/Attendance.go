package models

// StudentAttendanceStatus represents a student's attendance status for a specific date
type StudentAttendanceStatus struct {
	UserID        int     `json:"user_id"`
	Name          string  `json:"name"`
	Year          string  `json:"year"`
	GroupName     string  `json:"group_name"`
	CurrentStatus string  `json:"current_status"`
	ArrivedAt     *string `json:"arrived_at,omitempty"`
}

// AttendanceHistory represents a single attendance record for a student
type AttendanceHistory struct {
	ID             int     `json:"id"`
	StudentID      int     `json:"student_id"`
	Status         string  `json:"status"`
	AttendanceDate string  `json:"attendance_date"`
	ArrivedAt      *string `json:"arrived_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// AttendanceStats represents attendance statistics for a student
type AttendanceStats struct {
	Total      int     `json:"total"`
	Present    int     `json:"present"`
	Absent     int     `json:"absent"`
	Late       int     `json:"late"`
	Medical    int     `json:"medical"`
	Early      int     `json:"early"`
	Percentage float64 `json:"percentage"`
}

// StudentAttendanceDetails represents detailed attendance information for a student
type StudentAttendanceDetails struct {
	UserID  int                 `json:"user_id"`
	Name    string              `json:"name"`
	History []AttendanceHistory `json:"history"`
	Stats   AttendanceStats     `json:"stats"`
}
