package routes

import (
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Document represents a document in the database system
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

// StaticDocument represents a filesystem-based document
type StaticDocument struct {
	Name       string    `json:"name"`
	Folder     string    `json:"folder"`
	Path       string    `json:"path"`
	Extension  string    `json:"extension"`
	URL        string    `json:"url"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Type       string    `json:"type"`
}

// DocumentFolder represents a folder containing documents
type DocumentFolder struct {
	Name      string           `json:"name"`
	Path      string           `json:"path"`
	Documents []StaticDocument `json:"documents"`
	Count     int              `json:"count"`
}

// DocumentSummary provides an overview of all documents
type DocumentSummary struct {
	TotalDocuments int              `json:"total_documents"`
	Folders        []DocumentFolder `json:"folders"`
	AllDocuments   []StaticDocument `json:"all_documents"`
}

// getFileType determines the document type based on extension
func getFileType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".pdf":
		return "PDF"
	case ".doc", ".docx":
		return "Word Document"
	case ".xls", ".xlsx":
		return "Excel Spreadsheet"
	case ".ppt", ".pptx":
		return "PowerPoint Presentation"
	case ".txt":
		return "Text File"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
		return "Image"
	case ".mp4", ".avi", ".mov", ".wmv":
		return "Video"
	case ".mp3", ".wav", ".flac":
		return "Audio"
	case ".zip", ".rar", ".7z":
		return "Archive"
	default:
		return "Unknown"
	}
}

// scanDocumentsDirectory recursively scans the documents directory
func scanDocumentsDirectory(baseDir string) (DocumentSummary, error) {
	fmt.Printf("🔍 [DocumentScanner] Starting scan of directory: %s\n", baseDir)
	
	var allDocuments []StaticDocument
	folderMap := make(map[string][]StaticDocument)

	// Ensure the base directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		fmt.Printf("📁 [DocumentScanner] Creating directory: %s\n", baseDir)
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return DocumentSummary{}, fmt.Errorf("failed to create directory: %v", err)
		}
	}

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("⚠️ [DocumentScanner] Error accessing %s: %v\n", path, err)
			return nil // Continue walking
		}

		// Skip directories
		if d.IsDir() {
			fmt.Printf("📂 [DocumentScanner] Scanning folder: %s\n", path)
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			fmt.Printf("⚠️ [DocumentScanner] Error getting file info for %s: %v\n", path, err)
			return nil
		}

		// Get relative path from base directory
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			fmt.Printf("⚠️ [DocumentScanner] Error getting relative path for %s: %v\n", path, err)
			return nil
		}

		// Determine folder
		folder := "root"
		pathParts := strings.Split(relPath, string(os.PathSeparator))
		if len(pathParts) > 1 {
			folder = pathParts[0]
		}

		// Get file details
		name := info.Name()
		ext := filepath.Ext(name)
		urlPath := "/document-files/" + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		doc := StaticDocument{
			Name:       name,
			Folder:     folder,
			Path:       relPath,
			Extension:  strings.TrimPrefix(ext, "."),
			URL:        urlPath,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			Type:       getFileType(ext),
		}

		fmt.Printf("📄 [DocumentScanner] Found: %s (folder: %s, type: %s, size: %d bytes)\n", 
			name, folder, doc.Type, doc.Size)

		allDocuments = append(allDocuments, doc)
		folderMap[folder] = append(folderMap[folder], doc)

		return nil
	}

	// Walk the directory tree
	if err := filepath.WalkDir(baseDir, walkFunc); err != nil {
		return DocumentSummary{}, fmt.Errorf("failed to walk directory: %v", err)
	}

	// Sort documents by modification time (newest first)
	sort.Slice(allDocuments, func(i, j int) bool {
		return allDocuments[i].ModifiedAt.After(allDocuments[j].ModifiedAt)
	})

	// Create folder summaries
	var folders []DocumentFolder
	for folderName, docs := range folderMap {
		// Sort documents in each folder
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].ModifiedAt.After(docs[j].ModifiedAt)
		})

		folders = append(folders, DocumentFolder{
			Name:      folderName,
			Path:      folderName,
			Documents: docs,
			Count:     len(docs),
		})
	}

	// Sort folders by name
	sort.Slice(folders, func(i, j int) bool {
		// Put "root" folder first, then alphabetical
		if folders[i].Name == "root" {
			return true
		}
		if folders[j].Name == "root" {
			return false
		}
		return folders[i].Name < folders[j].Name
	})

	summary := DocumentSummary{
		TotalDocuments: len(allDocuments),
		Folders:        folders,
		AllDocuments:   allDocuments,
	}

	fmt.Printf("✅ [DocumentScanner] Scan complete: %d documents in %d folders\n", 
		len(allDocuments), len(folders))

	return summary, nil
}

// ListStaticDocumentsHandler returns all filesystem documents
func ListStaticDocumentsHandler(c *gin.Context) {
	fmt.Printf("📁 [ListStaticDocuments] Request received\n")
	
	baseDir := "./documents"
	summary, err := scanDocumentsDirectory(baseDir)
	if err != nil {
		fmt.Printf("❌ [ListStaticDocuments] Error scanning directory: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to scan documents directory",
			"error":   err.Error(),
		})
		return
	}

	fmt.Printf("✅ [ListStaticDocuments] Returning %d documents\n", summary.TotalDocuments)
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"summary":         summary,
		"total_documents": summary.TotalDocuments,
		"folders":         summary.Folders,
		"documents":       summary.AllDocuments,
	})
}

// GetDocumentsByFolderHandler returns documents from a specific folder
func GetDocumentsByFolderHandler(c *gin.Context) {
	folderName := c.Param("folder")
	fmt.Printf("📂 [GetDocumentsByFolder] Request for folder: %s\n", folderName)

	baseDir := "./documents"
	summary, err := scanDocumentsDirectory(baseDir)
	if err != nil {
		fmt.Printf("❌ [GetDocumentsByFolder] Error scanning directory: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to scan documents directory",
			"error":   err.Error(),
		})
		return
	}

	// Find the requested folder
	var targetFolder *DocumentFolder
	for _, folder := range summary.Folders {
		if folder.Name == folderName {
			targetFolder = &folder
			break
		}
	}

	if targetFolder == nil {
		fmt.Printf("❌ [GetDocumentsByFolder] Folder not found: %s\n", folderName)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("Folder '%s' not found", folderName),
		})
		return
	}

	fmt.Printf("✅ [GetDocumentsByFolder] Found %d documents in folder %s\n", 
		targetFolder.Count, folderName)
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"folder":    *targetFolder,
		"documents": targetFolder.Documents,
		"count":     targetFolder.Count,
	})
}

// SearchDocumentsHandler searches documents by name or type
func SearchDocumentsHandler(c *gin.Context) {
	query := c.Query("q")
	fileType := c.Query("type")
	folder := c.Query("folder")
	
	fmt.Printf("🔍 [SearchDocuments] Query: '%s', Type: '%s', Folder: '%s'\n", query, fileType, folder)

	baseDir := "./documents"
	summary, err := scanDocumentsDirectory(baseDir)
	if err != nil {
		fmt.Printf("❌ [SearchDocuments] Error scanning directory: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to scan documents directory",
			"error":   err.Error(),
		})
		return
	}

	var results []StaticDocument
	queryLower := strings.ToLower(query)
	fileTypeLower := strings.ToLower(fileType)
	folderLower := strings.ToLower(folder)

	for _, doc := range summary.AllDocuments {
		// Apply filters
		if folder != "" && strings.ToLower(doc.Folder) != folderLower {
			continue
		}
		
		if fileType != "" && strings.ToLower(doc.Type) != fileTypeLower {
			continue
		}
		
		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(doc.Name), queryLower)
			if !nameMatch {
				continue
			}
		}
		
		results = append(results, doc)
	}

	fmt.Printf("✅ [SearchDocuments] Found %d matching documents\n", len(results))
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"documents": results,
		"count":     len(results),
		"query":     query,
		"filters": gin.H{
			"type":   fileType,
			"folder": folder,
		},
	})
}

// GetDocumentStatsHandler returns statistics about documents
func GetDocumentStatsHandler(c *gin.Context) {
	fmt.Printf("📊 [GetDocumentStats] Request received\n")

	baseDir := "./documents"
	summary, err := scanDocumentsDirectory(baseDir)
	if err != nil {
		fmt.Printf("❌ [GetDocumentStats] Error scanning directory: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to scan documents directory",
			"error":   err.Error(),
		})
		return
	}

	// Calculate statistics
	typeCount := make(map[string]int)
	var totalSize int64
	
	for _, doc := range summary.AllDocuments {
		typeCount[doc.Type]++
		totalSize += doc.Size
	}

	stats := gin.H{
		"total_documents": summary.TotalDocuments,
		"total_folders":   len(summary.Folders),
		"total_size":      totalSize,
		"type_breakdown":  typeCount,
		"folders":         make([]gin.H, len(summary.Folders)),
	}

	// Add folder statistics
	for i, folder := range summary.Folders {
		var folderSize int64
		for _, doc := range folder.Documents {
			folderSize += doc.Size
		}
		stats["folders"].([]gin.H)[i] = gin.H{
			"name":  folder.Name,
			"count": folder.Count,
			"size":  folderSize,
		}
	}

	fmt.Printf("✅ [GetDocumentStats] Calculated stats for %d documents\n", summary.TotalDocuments)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
	})
}

// Database-based document handlers (for uploaded documents)

// GetDocumentsHandler handles requests to get all database documents
func GetDocumentsHandler(c *gin.Context, db *sql.DB) {
	fmt.Printf("🗄️ [GetDocuments] Request for database documents\n")
	
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
		fmt.Printf("❌ [GetDocuments] Database error: %v\n", err)
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
			&doc.ID, &doc.FileName, &fileDescription, &doc.FilePath,
			&doc.FileType, &doc.FileSize, &doc.UploadedBy, &uploaderName,
			&createdAt, &updatedAt, &doc.Status, &checksum, &doc.Version,
		)

		if err != nil {
			fmt.Printf("❌ [GetDocuments] Row scan error: %v\n", err)
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
		fmt.Printf("❌ [GetDocuments] Rows iteration error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Error iterating document rows",
			"error":   err.Error(),
		})
		return
	}

	fmt.Printf("✅ [GetDocuments] Retrieved %d database documents\n", len(documents))
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"documents": documents,
		"count":     len(documents),
	})
}

// UploadDocumentHandler handles document uploads
func UploadDocumentHandler(c *gin.Context, db *sql.DB) {
	fmt.Printf("📤 [UploadDocument] Request received\n")
	
	userIDStr := c.PostForm("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Missing user ID",
		})
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID format",
			"error":   err.Error(),
		})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No file uploaded or invalid file",
			"error":   err.Error(),
		})
		return
	}

	docID := uuid.New().String()
	fileDescription := c.PostForm("description")
	fileType := filepath.Ext(file.Filename)
	if fileType == "" {
		fileType = "unknown"
	} else {
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

	// Insert into database
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

	err = db.QueryRow(query, docID, file.Filename, fileDescription, filePath, 
		fileType, file.Size, userID).Scan(&createdAt, &updatedAt, &status, &version)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save document information to database",
			"error":   err.Error(),
		})
		return
	}

	fileURL := "/document-files/" + fileName
	fmt.Printf("✅ [UploadDocument] Successfully uploaded: %s\n", fileName)

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

// SetupDocumentRoutes registers all document-related routes
func SetupDocumentRoutes(router gin.IRouter, db *sql.DB) {
	fmt.Printf("🚀 [SetupDocumentRoutes] Registering document routes\n")
	
	// Create documents directory if it doesn't exist
	if _, err := os.Stat("./documents"); os.IsNotExist(err) {
		if err := os.MkdirAll("./documents", 0755); err != nil {
			fmt.Printf("❌ [SetupDocumentRoutes] Error creating documents directory: %v\n", err)
		} else {
			fmt.Printf("📁 [SetupDocumentRoutes] Created documents directory\n")
		}
	}

	// Static/filesystem document routes
	router.GET("/documents/static", ListStaticDocumentsHandler)
	router.GET("/documents/folder/:folder", GetDocumentsByFolderHandler) 
	router.GET("/documents/search", SearchDocumentsHandler)
	router.GET("/documents/stats", GetDocumentStatsHandler)

	// Database document routes
	router.GET("/documents", func(c *gin.Context) {
		GetDocumentsHandler(c, db)
	})
	
	router.POST("/documents", func(c *gin.Context) {
		UploadDocumentHandler(c, db)
	})

	// Individual document routes (database-based)
	router.GET("/documents/:id", func(c *gin.Context) {
		// This would be GetDocumentByIDHandler - implement if needed
		c.JSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"message": "Individual document retrieval not implemented yet",
		})
	})

	router.DELETE("/documents/:id", func(c *gin.Context) {
		// This would be DeleteDocumentHandler - implement if needed
		c.JSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"message": "Document deletion not implemented yet",
		})
	})

	fmt.Printf("✅ [SetupDocumentRoutes] All document routes registered successfully\n")
}
