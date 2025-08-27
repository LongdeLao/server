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
	FilePath string `json:"filePath"`
}

/**
 * Event represents the complete event data structure.
 */
type Event struct {
	EventID          string       `json:"eventID"`
	AuthorID         int          `json:"authorID"`
	AuthorName       string       `json:"authorName"`
	Title            string       `json:"title"`
	EventDescription string       `json:"eventDescription"`
	Images           []ImageModel `json:"images"`
	Address          string       `json:"address"`
	EventDate        time.Time    `json:"eventDate"`
	IsWholeDay       bool         `json:"isWholeDay"`
	StartTime        *time.Time   `json:"startTime,omitempty"`
	EndTime          *time.Time   `json:"endTime,omitempty"`
}

/**
 * SaveImage saves the uploaded image to the server.
 *
 * Parameters:
 *   - file: Multipart file header
 *   - c: Gin context
 *
 * Returns:
 *   - string: File path of saved image
 *   - error: Any error that occurred during saving
 */
func SaveImage(file *multipart.FileHeader, c *gin.Context) (string, error) {
	imageID := uuid.New().String()
	extension := filepath.Ext(file.Filename)
	imagePath := fmt.Sprintf("images/%s%s", imageID, extension)

	err := os.MkdirAll("images", os.ModePerm)
	if err != nil {
		log.Println("Error creating directory for image storage:", err)
		return "", fmt.Errorf("error creating directory: %v", err)
	}

	if err := c.SaveUploadedFile(file, imagePath); err != nil {
		log.Println("Error saving image to file:", err)
		return "", fmt.Errorf("error saving image to file: %v", err)
	}

	return imagePath, nil
}

/**
 * InsertEvent inserts the event into the events table and the image file paths into the event_images table.
 *
 * Parameters:
 *   - db: Database connection
 *   - event: Event data to insert
 *   - c: Gin context for file handling
 *
 * Returns:
 *   - error: Any error that occurred during insertion
 */
func InsertEvent(db *sql.DB, event Event, c *gin.Context) error {
	log.Println("Starting transaction for event:", event.EventID)
	tx, err := db.Begin()
	if err != nil {
		log.Println("Error beginning transaction:", err)
		return err
	}

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

	queryImage := `
		INSERT INTO event_images (id, event_id, file_path) 
		VALUES ($1, $2, $3)
	`
	files := c.Request.MultipartForm.File["images"]
	for _, file := range files {
		imagePath, err := SaveImage(file, c)
		if err != nil {
			log.Println("Error saving image:", err)
			tx.Rollback()
			return err
		}

		imageID := uuid.New().String()
		_, err = tx.Exec(queryImage, imageID, event.EventID, imagePath)
		if err != nil {
			log.Println("Error inserting image data:", err)
			tx.Rollback()
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		return err
	}
	log.Println("Event and images inserted successfully.")
	return nil
}

/**
 * UpdateEvent updates an existing event in the events table and manages its images.
 *
 * Parameters:
 *   - db: Database connection
 *   - event: Updated event data
 *   - c: Gin context for file handling
 *
 * Returns:
 *   - error: Any error that occurred during update
 */
func UpdateEvent(db *sql.DB, event Event, c *gin.Context) error {
	log.Println("Starting transaction for updating event:", event.EventID)
	tx, err := db.Begin()
	if err != nil {
		log.Println("Error beginning transaction:", err)
		return err
	}

	queryEvent := `
		UPDATE events 
		SET author_id = $2, author_name = $3, title = $4, event_description = $5, 
		    address = $6, event_date = $7, is_whole_day = $8, start_time = $9, end_time = $10
		WHERE event_id = $1
	`
	result, err := tx.Exec(queryEvent, event.EventID, event.AuthorID, event.AuthorName, event.Title,
		event.EventDescription, event.Address, event.EventDate, event.IsWholeDay, event.StartTime, event.EndTime)
	if err != nil {
		log.Println("Error updating event:", err)
		tx.Rollback()
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("Error getting rows affected:", err)
		tx.Rollback()
		return err
	}
	if rowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("event not found")
	}

	deleteImagesQuery := `DELETE FROM event_images WHERE event_id = $1`
	_, err = tx.Exec(deleteImagesQuery, event.EventID)
	if err != nil {
		log.Println("Error deleting existing images:", err)
		tx.Rollback()
		return err
	}

	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File["images"] != nil {
		queryImage := `
			INSERT INTO event_images (id, event_id, file_path) 
			VALUES ($1, $2, $3)
		`
		files := c.Request.MultipartForm.File["images"]
		for _, file := range files {
			imagePath, err := SaveImage(file, c)
			if err != nil {
				log.Println("Error saving image:", err)
				tx.Rollback()
				return err
			}

			imageID := uuid.New().String()
			_, err = tx.Exec(queryImage, imageID, event.EventID, imagePath)
			if err != nil {
				log.Println("Error inserting image data:", err)
				tx.Rollback()
				return err
			}
		}
	}

	if err = tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		return err
	}
	log.Println("Event updated successfully.")
	return nil
}

/**
 * DeleteEvent deletes an event and all its associated images from the database.
 *
 * Parameters:
 *   - db: Database connection
 *   - eventID: Event ID to delete
 *
 * Returns:
 *   - error: Any error that occurred during deletion
 */
func DeleteEvent(db *sql.DB, eventID string) error {
	log.Println("Starting transaction for deleting event:", eventID)
	tx, err := db.Begin()
	if err != nil {
		log.Println("Error beginning transaction:", err)
		return err
	}

	getImagesQuery := `SELECT file_path FROM event_images WHERE event_id = $1`
	rows, err := tx.Query(getImagesQuery, eventID)
	if err != nil {
		log.Println("Error getting image paths:", err)
		tx.Rollback()
		return err
	}

	var imagePaths []string
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			log.Println("Error scanning image path:", err)
			rows.Close()
			tx.Rollback()
			return err
		}
		imagePaths = append(imagePaths, filePath)
	}
	rows.Close()

	deleteImagesQuery := `DELETE FROM event_images WHERE event_id = $1`
	_, err = tx.Exec(deleteImagesQuery, eventID)
	if err != nil {
		log.Println("Error deleting image records:", err)
		tx.Rollback()
		return err
	}

	deleteEventQuery := `DELETE FROM events WHERE event_id = $1`
	result, err := tx.Exec(deleteEventQuery, eventID)
	if err != nil {
		log.Println("Error deleting event:", err)
		tx.Rollback()
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("Error getting rows affected:", err)
		tx.Rollback()
		return err
	}
	if rowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("event not found")
	}

	if err = tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		return err
	}

	for _, imagePath := range imagePaths {
		if err := os.Remove(imagePath); err != nil {
			log.Printf("Warning: Could not delete image file %s: %v", imagePath, err)
		}
	}

	log.Println("Event and associated images deleted successfully.")
	return nil
}

/**
 * RegisterEventRoutes registers all event-related routes.
 *
 * Endpoints:
 * 1. POST /post_event
 *    - Creates a new event
 *    - Accepts multipart form data with event details and images
 *    - Returns success message with event ID
 *
 * 2. PUT /update_event
 *    - Updates an existing event
 *    - Accepts multipart form data with updated event details and images
 *    - Returns success message
 *
 * 3. DELETE /delete_event
 *    - Deletes an event by ID
 *    - Accepts eventID in JSON format
 *    - Returns success message
 */
func RegisterEventRoutes(router gin.IRouter, db *sql.DB) {
	router.POST("/post_event", func(c *gin.Context) {
		log.Println("Received POST request for /post_event")

		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}

		authorID, err := strconv.Atoi(c.PostForm("authorID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authorID"})
			return
		}

		eventDate, err := time.Parse("2006-01-02T15:04:05.000Z", c.PostForm("eventDate"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event date format"})
			return
		}

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

		if err := InsertEvent(db, event, c); err != nil {
			log.Println("Failed to insert event:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert event: " + err.Error()})
			return
		}

		log.Println("Event inserted successfully:", event.EventID)
		c.JSON(http.StatusCreated, gin.H{"message": "Event created successfully", "eventID": event.EventID})
	})

	router.PUT("/update_event", func(c *gin.Context) {
		log.Println("Received PUT request for /update_event")

		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}

		eventID := c.PostForm("eventID")
		if eventID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Event ID is required"})
			return
		}

		authorID, err := strconv.Atoi(c.PostForm("authorID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authorID"})
			return
		}

		eventDate, err := time.Parse("2006-01-02T15:04:05.000Z", c.PostForm("eventDate"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event date format"})
			return
		}

		event := Event{
			EventID:          eventID,
			AuthorID:         authorID,
			AuthorName:       c.PostForm("authorName"),
			Title:            c.PostForm("title"),
			EventDescription: c.PostForm("eventDescription"),
			Address:          c.PostForm("address"),
			EventDate:        eventDate,
			IsWholeDay:       c.PostForm("isWholeDay") == "true",
		}

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

		if err := UpdateEvent(db, event, c); err != nil {
			log.Println("Failed to update event:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event: " + err.Error()})
			return
		}

		log.Println("Event updated successfully:", event.EventID)
		c.JSON(http.StatusOK, gin.H{"message": "Event updated successfully", "eventID": event.EventID})
	})

	router.DELETE("/delete_event", func(c *gin.Context) {
		log.Println("Received DELETE request for /delete_event")

		var requestBody struct {
			EventID string `json:"eventID"`
		}

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if requestBody.EventID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Event ID is required"})
			return
		}

		if err := DeleteEvent(db, requestBody.EventID); err != nil {
			log.Println("Failed to delete event:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete event: " + err.Error()})
			return
		}

		log.Println("Event deleted successfully:", requestBody.EventID)
		c.JSON(http.StatusOK, gin.H{"message": "Event deleted successfully", "eventID": requestBody.EventID})
	})
}

/**
 * parseInt parses a string to integer.
 *
 * Parameters:
 *   - s: String to parse
 *
 * Returns:
 *   - int: Parsed integer value
 */
func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

/**
 * parseTime parses a string to time.Time.
 *
 * Parameters:
 *   - s: String to parse
 *
 * Returns:
 *   - time.Time: Parsed time value
 */
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

/**
 * EventWithoutImages represents the event data structure without images.
 */
type EventWithoutImages struct {
	EventID          string     `json:"eventID"`
	AuthorID         int        `json:"authorID"`
	AuthorName       string     `json:"authorName"`
	Title            string     `json:"title"`
	EventDescription string     `json:"eventDescription"`
	Images           []string   `json:"images"`
	Address          string     `json:"address"`
	EventDate        time.Time  `json:"eventDate"`
	IsWholeDay       bool       `json:"isWholeDay"`
	StartTime        *time.Time `json:"startTime,omitempty"`
	EndTime          *time.Time `json:"endTime,omitempty"`
}

/**
 * RegisterGetAllEvents registers a route that returns all events without images.
 *
 * Endpoint: GET /events
 *
 * Returns:
 *   - 200 OK: All events retrieved successfully
 *     {
 *       "events": array
 *     }
 *   - 500 Internal Server Error: Database error
 */
func RegisterGetAllEvents(router gin.IRouter, db *sql.DB) {
	router.GET("/events", func(c *gin.Context) {
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

		var events []Event
		for rows.Next() {
			var ev Event
			var startTime, endTime *time.Time
			if err := rows.Scan(&ev.EventID, &ev.AuthorID, &ev.AuthorName, &ev.Title, &ev.EventDescription, &ev.Address, &ev.EventDate, &ev.IsWholeDay, &startTime, &endTime); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			ev.StartTime = startTime
			ev.EndTime = endTime

			imagesQuery := `
				SELECT file_path
				FROM event_images
				WHERE event_id = $1;
			`
			imageRows, err := db.Query(imagesQuery, ev.EventID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching images: " + err.Error()})
				return
			}
			defer imageRows.Close()

			ev.Images = []ImageModel{}

			for imageRows.Next() {
				var filePath string
				if err := imageRows.Scan(&filePath); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning image: " + err.Error()})
					return
				}
				ev.Images = append(ev.Images, ImageModel{
					FilePath: filePath,
				})
			}

			if err = imageRows.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error in image rows: " + err.Error()})
				return
			}

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
 * Path Parameters:
 *   - id: The unique identifier of the event
 *
 * Returns:
 *   - 200 OK: Successfully retrieved event
 *     {
 *       "event": object
 *     }
 *   - 404 Not Found: Event not found
 *   - 500 Internal Server Error: Database error
 */
func RegisterGetEventByID(router gin.IRouter, db *sql.DB) {
	router.GET("/event/:id", func(c *gin.Context) {
		eventID := c.Param("id")

		event, err := GetEventByID(db, eventID)
		if err != nil {
			if err.Error() == "event not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		fmt.Printf("Event to be sent: %+v\n", event)

		c.JSON(http.StatusOK, gin.H{"event": event})
	})
}

/**
 * GetEventByID fetches the event by its ID and returns the complete event including images.
 *
 * Parameters:
 *   - db: Database connection
 *   - eventID: The unique identifier of the event
 *
 * Returns:
 *   - *Event: The complete event data
 *   - error: Any error that occurred during the process
 */
func GetEventByID(db *sql.DB, eventID string) (*Event, error) {
	eventQuery := `
		SELECT event_id, author_id, author_name, title, event_description, address,
		       event_date, is_whole_day, start_time, end_time
		FROM events
		WHERE event_id = $1;
	`

	imagesQuery := `
		SELECT file_path
		FROM event_images
		WHERE event_id = $1;
	`

	var event Event
	var startTime, endTime *time.Time

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

	event.StartTime = startTime
	event.EndTime = endTime

	rows, err := db.Query(imagesQuery, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	event.Images = []ImageModel{}

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
