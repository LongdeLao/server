package routes

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Document represents a document in the system
type Document struct {
	ID              string    `json:"id"`
	FileName        string    `json:"file_name"`
	FileDescription string    `json:"file_description"`
	FilePath        string    `json:"file_path"`
	FileType        string    `json:"file_type"`
	FileSize        int       `json:"file_size"`
	UploadedBy      int       `json:"uploaded_by"`
	UploaderName    string    `json:"uploader_name"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Status          string    `json:"status"`
	Checksum        string    `json:"checksum"`
	Version         int       `json:"version"`
}

// GetDocumentsHandler handles requests to get all documents
func GetDocumentsHandler(c *gin.Context, db *sql.DB) {
	// Query to get all active documents
	query := `
		SELECT d.id, d.file_name, d.file_description, d.file_path, 
			   d.file_type, d.file_size, d.uploaded_by, 
			   u.name AS uploader_name,
			   d.created_at, d.updated_at, d.status, d.checksum, d.version
		FROM documents d
		LEFT JOIN users u ON d.uploaded_by = u.id
		WHERE d.status = 'active'
		ORDER BY d.created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve documents",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	var documents []Document
	for rows.Next() {
		var doc Document
		var uploaderName, fileDescription, checksum sql.NullString
		var createdAt, updatedAt sql.NullTime

		err := rows.Scan(
			&doc.ID,
			&doc.FileName,
			&fileDescription,
			&doc.FilePath,
			&doc.FileType,
			&doc.FileSize,
			&doc.UploadedBy,
			&uploaderName,
			&createdAt,
			&updatedAt,
			&doc.Status,
			&checksum,
			&doc.Version,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error scanning document row",
				"error":   err.Error(),
			})
			return
		}

		// Handle nullable fields
		if fileDescription.Valid {
			doc.FileDescription = fileDescription.String
		}
		if checksum.Valid {
			doc.Checksum = checksum.String
		}
		if uploaderName.Valid {
			doc.UploaderName = uploaderName.String
		}
		if createdAt.Valid {
			doc.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			doc.UpdatedAt = updatedAt.Time
		}

		// Convert file path to public URL
		doc.FilePath = "/document-files/" + filepath.Base(doc.FilePath)

		documents = append(documents, doc)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error iterating document rows",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"documents": documents,
		"count":     len(documents),
	})
}

// UploadDocumentHandler handles document uploads
func UploadDocumentHandler(c *gin.Context, db *sql.DB) {
	// Get user ID from the request
	userIDStr := c.PostForm("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Missing user ID",
		})
		return
	}

	// Convert user ID to integer
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID format",
			"error":   err.Error(),
		})
		return
	}

	// Get the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No file uploaded or invalid file",
			"error":   err.Error(),
		})
		return
	}

	// Generate a unique ID for the document
	docID := uuid.New().String()

	// Get file description
	fileDescription := c.PostForm("description")

	// Determine file type
	fileType := filepath.Ext(file.Filename)
	if fileType == "" {
		fileType = "unknown"
	} else {
		// Remove the dot from the extension
		fileType = fileType[1:]
	}

	// Create documents directory if it doesn't exist
	if _, err := os.Stat("./documents"); os.IsNotExist(err) {
		if err := os.MkdirAll("./documents", 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to create documents directory",
				"error":   err.Error(),
			})
			return
		}
	}

	// Save file with a unique name to prevent overwriting
	fileName := fmt.Sprintf("%s_%s", docID, file.Filename)
	filePath := filepath.Join("documents", fileName)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save file",
			"error":   err.Error(),
		})
		return
	}

	// Insert document information into the database
	query := `
		INSERT INTO documents (
			id, file_name, file_description, file_path, 
			file_type, file_size, uploaded_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at, status, version
	`

	var createdAt, updatedAt time.Time
	var status string
	var version int

	err = db.QueryRow(
		query,
		docID,
		file.Filename,
		fileDescription,
		filePath,
		fileType,
		file.Size,
		userID,
	).Scan(&createdAt, &updatedAt, &status, &version)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save document information to database",
			"error":   err.Error(),
		})
		return
	}

	// Create document URL for the response
	fileURL := "/document-files/" + fileName

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Document uploaded successfully",
		"document": Document{
			ID:              docID,
			FileName:        file.Filename,
			FileDescription: fileDescription,
			FilePath:        fileURL,
			FileType:        fileType,
			FileSize:        int(file.Size),
			UploadedBy:      userID,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
			Status:          status,
			Version:         version,
		},
	})
}

// GetDocumentByIDHandler handles request to get a specific document by ID
func GetDocumentByIDHandler(c *gin.Context, db *sql.DB) {
	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Document ID is required",
		})
		return
	}

	query := `
		SELECT d.id, d.file_name, d.file_description, d.file_path, 
			   d.file_type, d.file_size, d.uploaded_by, 
			   u.name AS uploader_name,
			   d.created_at, d.updated_at, d.status, d.checksum, d.version
		FROM documents d
		LEFT JOIN users u ON d.uploaded_by = u.id
		WHERE d.id = $1 AND d.status = 'active'
	`

	var doc Document
	var uploaderName, fileDescription, checksum sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := db.QueryRow(query, documentID).Scan(
		&doc.ID,
		&doc.FileName,
		&fileDescription,
		&doc.FilePath,
		&doc.FileType,
		&doc.FileSize,
		&doc.UploadedBy,
		&uploaderName,
		&createdAt,
		&updatedAt,
		&doc.Status,
		&checksum,
		&doc.Version,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Document not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error retrieving document",
				"error":   err.Error(),
			})
		}
		return
	}

	// Handle nullable fields
	if fileDescription.Valid {
		doc.FileDescription = fileDescription.String
	}
	if checksum.Valid {
		doc.Checksum = checksum.String
	}
	if uploaderName.Valid {
		doc.UploaderName = uploaderName.String
	}
	if createdAt.Valid {
		doc.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		doc.UpdatedAt = updatedAt.Time
	}

	// Convert file path to public URL
	doc.FilePath = "/document-files/" + filepath.Base(doc.FilePath)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"document": doc,
	})
}

// DeleteDocumentHandler handles document deletion (soft delete)
func DeleteDocumentHandler(c *gin.Context, db *sql.DB) {
	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Document ID is required",
		})
		return
	}

	// Update the document status to 'deleted' instead of actually deleting
	query := `
		UPDATE documents 
		SET status = 'deleted', updated_at = NOW() 
		WHERE id = $1 AND status = 'active'
		RETURNING 1
	`

	var result int
	err := db.QueryRow(query, documentID).Scan(&result)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Document not found or already deleted",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Error deleting document",
				"error":   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Document deleted successfully",
	})
}

// SetupDocumentRoutes registers all document-related routes
func SetupDocumentRoutes(router gin.IRouter, db *sql.DB) {
	// Create documents directory if it doesn't exist
	if _, err := os.Stat("./documents"); os.IsNotExist(err) {
		if err := os.MkdirAll("./documents", 0755); err != nil {
			fmt.Printf("Error creating documents directory: %v\n", err)
		}
	}

	// Get all documents
	router.GET("/documents", func(c *gin.Context) {
		GetDocumentsHandler(c, db)
	})

	// Get document by ID
	router.GET("/documents/:id", func(c *gin.Context) {
		GetDocumentByIDHandler(c, db)
	})

	// Upload a document
	router.POST("/documents", func(c *gin.Context) {
		UploadDocumentHandler(c, db)
	})

	// Delete a document
	router.DELETE("/documents/:id", func(c *gin.Context) {
		DeleteDocumentHandler(c, db)
	})
}
