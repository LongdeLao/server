package routes

import (
	"database/sql"

	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"mime/multipart"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

/**
 * ImageModel represents an image associated with an event.
 */
type ImageModel struct {
	FilePath string `json:"filePath"` // The file path of the image
}

/**
 * Event represents the complete event data structure.
 */
type Event struct {
	EventID          string       `json:"eventID"`             // Unique identifier for the event
	AuthorID         int          `json:"authorID"`            // ID of the event creator
	AuthorName       string       `json:"authorName"`          // Name of the event creator
	Title            string       `json:"title"`               // Event title
	EventDescription string       `json:"eventDescription"`    // Detailed description
	Images           []ImageModel `json:"images"`              // List of associated images
	Address          string       `json:"address"`             // Event location
	EventDate        time.Time    `json:"eventDate"`           // Date of the event
	IsWholeDay       bool         `json:"isWholeDay"`          // Whether it's a whole-day event
	StartTime        *time.Time   `json:"startTime,omitempty"` // Start time (if not whole-day)
	EndTime          *time.Time   `json:"endTime,omitempty"`   // End time (if not whole-day)
}

// SaveImage saves the uploaded image to the server.
func SaveImage(file *multipart.FileHeader, c *gin.Context) (string, error) {
	// Generate a unique filename for the image
	imageID := uuid.New().String()
	extension := filepath.Ext(file.Filename)
	imagePath := fmt.Sprintf("images/%s%s", imageID, extension)

	// Create the images directory if it doesn't exist
	err := os.MkdirAll("images", os.ModePerm)
	if err != nil {
		log.Println("Error creating directory for image storage:", err)
		return "", fmt.Errorf("error creating directory: %v", err)
	}

	// Save the uploaded file
	if err := c.SaveUploadedFile(file, imagePath); err != nil {
		log.Println("Error saving image to file:", err)
		return "", fmt.Errorf("error saving image to file: %v", err)
	}

	return imagePath, nil
}

// InsertEvent inserts the event into the events table and the image file paths into the event_images table.
func InsertEvent(db *sql.DB, event Event, c *gin.Context) error {
	log.Println("Starting transaction for event:", event.EventID)
	tx, err := db.Begin()
	if err != nil {
		log.Println("Error beginning transaction:", err)
		return err
	}

	// Insert event data into events table
	queryEvent := `
		INSERT INTO events (event_id, author_id, author_name, title, event_description, address, event_date, is_whole_day, start_time, end_time) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = tx.Exec(queryEvent, event.EventID, event.AuthorID, event.AuthorName, event.Title, event.EventDescription, event.Address, event.EventDate, event.IsWholeDay, event.StartTime, event.EndTime)
	if err != nil {
		log.Println("Error inserting event:", err)
		tx.Rollback()
		return err
	}

	// Insert images with file paths
	queryImage := `
		INSERT INTO event_images (id, event_id, file_path) 
		VALUES ($1, $2, $3)
	`
	files := c.Request.MultipartForm.File["images"]
	for _, file := range files {
		// Save the image and get the file path
		imagePath, err := SaveImage(file, c)
		if err != nil {
			log.Println("Error saving image:", err)
			tx.Rollback()
			return err
		}

		// Insert image data into the database
		imageID := uuid.New().String()
		_, err = tx.Exec(queryImage, imageID, event.EventID, imagePath)
		if err != nil {
			log.Println("Error inserting image data:", err)
			tx.Rollback()
			return err
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		return err
	}
	log.Println("Event and images inserted successfully.")
	return nil
}

/**
 * RegisterEventRoutes registers all event-related routes.
 *
 * Endpoints:
 * 1. POST /post_event
 *    - Creates a new event
 *    - Accepts event data in JSON format
 *    - Returns success message with event ID
 *
 * 2. GET /event/:id
 *    - Retrieves event details by ID
 *    - Returns complete event data including images
 */
func RegisterEventRoutes(router gin.IRouter, db *sql.DB) {
	router.POST("/post_event", func(c *gin.Context) {
		log.Println("Received POST request for /post_event")

		// Parse multipart form
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}

		// Parse authorID as integer
		authorID, err := strconv.Atoi(c.PostForm("authorID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authorID"})
			return
		}

		// Parse eventDate in ISO format
		eventDate, err := time.Parse("2006-01-02T15:04:05.000Z", c.PostForm("eventDate"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event date format"})
			return
		}

		// Create event from form data
		event := Event{
			EventID:          c.PostForm("eventID"),
			AuthorID:         authorID,
			AuthorName:       c.PostForm("authorName"),
			Title:            c.PostForm("title"),
			EventDescription: c.PostForm("eventDescription"),
			Address:          c.PostForm("address"),
			EventDate:        eventDate,
			IsWholeDay:       c.PostForm("isWholeDay") == "true",
		}

		// Parse optional time fields in ISO format
		if startTime := c.PostForm("startTime"); startTime != "" {
			t, err := time.Parse("2006-01-02T15:04:05.000Z", startTime)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start time format"})
				return
			}
			event.StartTime = &t
		}
		if endTime := c.PostForm("endTime"); endTime != "" {
			t, err := time.Parse("2006-01-02T15:04:05.000Z", endTime)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end time format"})
				return
			}
			event.EndTime = &t
		}

		// Insert the event (and images) into the database
		if err := InsertEvent(db, event, c); err != nil {
			log.Println("Failed to insert event:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert event: " + err.Error()})
			return
		}

		// Return success message
		log.Println("Event inserted successfully:", event.EventID)
		c.JSON(http.StatusCreated, gin.H{"message": "Event created successfully", "eventID": event.EventID})
	})
}

// Helper function to parse integers
func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}


// Helper function to parse time
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// EventWithoutImages represents the event data structure without images.
type EventWithoutImages struct {
	EventID          string     `json:"eventID"`
	AuthorID         int        `json:"authorID"`
	AuthorName       string     `json:"authorName"`
	Title            string     `json:"title"`
	EventDescription string     `json:"eventDescription"`
	Images           []string   `json:"images"` // Ensure images come after eventDescription
	Address          string     `json:"address"`
	EventDate        time.Time  `json:"eventDate"`
	IsWholeDay       bool       `json:"isWholeDay"`
	StartTime        *time.Time `json:"startTime,omitempty"` // Use pointer for optional fields
	EndTime          *time.Time `json:"endTime,omitempty"`   // Use pointer for optional fields
}

// RegisterGetAllEvents registers a route that returns all events without images.
func RegisterGetAllEvents(router gin.IRouter, db *sql.DB) {
	router.GET("/events", func(c *gin.Context) {
		// Query all events from the database (no filtering by month or year)
		query := `
			SELECT event_id, author_id, author_name, title, event_description, address,
			       event_date, is_whole_day, start_time, end_time
			FROM events;
		`
		rows, err := db.Query(query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var events []EventWithoutImages
		for rows.Next() {
			var ev EventWithoutImages
			var startTime, endTime *time.Time // Initialize as pointers to nil
			if err := rows.Scan(&ev.EventID, &ev.AuthorID, &ev.AuthorName, &ev.Title, &ev.EventDescription, &ev.Address, &ev.EventDate, &ev.IsWholeDay, &startTime, &endTime); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Assign startTime and endTime pointers if they are not null
			ev.StartTime = startTime
			ev.EndTime = endTime

			// Set the images field to an empty array since we're not returning image data
			ev.Images = []string{} // Empty array for images

			events = append(events, ev)
		}

		if err = rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"events": events})
	})
}

/**
 * RegisterGetEventByID registers the route for fetching an event by its ID.
 *
 * Endpoint: GET /event/:id
 *
 * Parameters:
 *   - id: The unique identifier of the event
 *
 * Returns:
 *   - 200 OK: Successfully retrieved event
 *     {
 *       "event": {
 *         "eventID": string,          // Unique identifier
 *         "authorID": number,         // Creator's ID
 *         "authorName": string,       // Creator's name
 *         "title": string,            // Event title
 *         "eventDescription": string, // Description
 *         "images": [                 // List of images
 *           {
 *             "id": string,          // Image ID
 *             "data": string         // Base64 image data
 *           }
 *         ],
 *         "address": string,          // Location
 *         "eventDate": string,        // Event date
 *         "isWholeDay": boolean,      // Whole-day flag
 *         "startTime": string,        // Start time (optional)
 *         "endTime": string          // End time (optional)
 *       }
 *     }
 *   - 404 Not Found: Event not found
 *   - 500 Internal Server Error: Database error
 */
func RegisterGetEventByID(router gin.IRouter, db *sql.DB) {
	router.GET("/event/:id", func(c *gin.Context) {
		// Get the eventID from the URL parameters
		eventID := c.Param("id")

		// Fetch the event from the database using GetEventByID function
		event, err := GetEventByID(db, eventID)
		if err != nil {
			if err.Error() == "event not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		// Print the event data that will be sent as JSON
		fmt.Printf("Event to be sent: %+v\n", event)

		// Return the event details as a JSON response
		c.JSON(http.StatusOK, gin.H{"event": event})
	})
}

/**
 * GetEventByID fetches the event by its ID and returns the complete event including images.
 *
 * @param db *sql.DB - Database connection
 * @param eventID string - The unique identifier of the event
 * @return *Event - The complete event data
 * @return error - Any error that occurred during the process
 */
func GetEventByID(db *sql.DB, eventID string) (*Event, error) {
	// Query to fetch event details
	eventQuery := `
		SELECT event_id, author_id, author_name, title, event_description, address,
		       event_date, is_whole_day, start_time, end_time
		FROM events
		WHERE event_id = $1;
	`

	// Query to fetch images for the event
	imagesQuery := `
		SELECT file_path
		FROM event_images
		WHERE event_id = $1;
	`

	// Create an event variable to hold the event data
	var event Event
	var startTime, endTime *time.Time

	// Fetch event details from the database
	err := db.QueryRow(eventQuery, eventID).Scan(
		&event.EventID, &event.AuthorID, &event.AuthorName, &event.Title,
		&event.EventDescription, &event.Address, &event.EventDate,
		&event.IsWholeDay, &startTime, &endTime,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, err
	}

	// Set startTime and endTime
	event.StartTime = startTime
	event.EndTime = endTime

	// Fetch image paths for the event
	rows, err := db.Query(imagesQuery, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Initialize images slice
	event.Images = []ImageModel{}

	// Collect image paths
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, err
		}
		event.Images = append(event.Images, ImageModel{
			FilePath: filePath,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &event, nil
}
