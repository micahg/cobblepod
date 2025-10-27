package endpoints

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cobblepod/internal/queue"
	"cobblepod/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BackupUploadRequest represents the file upload request
type BackupUploadRequest struct {
	File *os.File `json:"-"`
}

// BackupUploadResponse represents the upload response
type BackupUploadResponse struct {
	Success bool   `json:"success"`
	FileID  string `json:"file_id,omitempty"`
	JobID   string `json:"job_id,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleBackupUpload processes backup file upload
func HandleBackupUpload(c *gin.Context) {
	// Get user ID from context (set by Auth0Middleware)
	userID, err := RequireAuth0(c)
	if err != nil {
		slog.Error("Failed to get user ID from context", "error", err)
		c.JSON(http.StatusUnauthorized, BackupUploadResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	// Exchange Auth0 token for Google access token
	googleToken, err := GetGoogleAccessToken(c.Request.Context(), userID)
	if err != nil {
		slog.Error("Failed to get Google access token", "error", err, "user_id", userID)
		c.JSON(http.StatusUnauthorized, BackupUploadResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to authenticate with Google: %v", err),
		})
		return
	}

	slog.Info("Successfully exchanged Auth0 token for Google token", "user_id", userID)

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		slog.Error("Failed to get file from form", "error", err)
		c.JSON(http.StatusBadRequest, BackupUploadResponse{
			Success: false,
			Error:   "Failed to parse file upload",
		})
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".backup") {
		slog.Warn("Invalid file extension", "filename", header.Filename)
		c.JSON(http.StatusBadRequest, BackupUploadResponse{
			Success: false,
			Error:   "File must have .backup extension",
		})
		return
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "backup-*.backup")
	if err != nil {
		slog.Error("Failed to create temporary file", "error", err)
		c.JSON(http.StatusInternalServerError, BackupUploadResponse{
			Success: false,
			Error:   "Failed to create temporary file",
		})
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up temp file after upload

	// Copy uploaded file to temp file
	_, err = io.Copy(tmpFile, file)
	if err != nil {
		slog.Error("Failed to copy file content", "error", err)
		tmpFile.Close()
		c.JSON(http.StatusInternalServerError, BackupUploadResponse{
			Success: false,
			Error:   "Failed to save file",
		})
		return
	}
	tmpFile.Close()

	// Create Google Drive service with user's Google access token
	driveService, err := storage.NewServiceWithToken(c.Request.Context(), googleToken)
	if err != nil {
		slog.Error("Failed to create Drive service", "error", err)
		c.JSON(http.StatusInternalServerError, BackupUploadResponse{
			Success: false,
			Error:   "Failed to initialize storage service",
		})
		return
	}

	// Upload file to Google Drive
	fileID, err := driveService.UploadFile(tmpFile.Name(), filepath.Base(header.Filename), "application/octet-stream")
	if err != nil {
		slog.Error("Failed to upload file to Drive", "error", err, "filename", header.Filename)
		c.JSON(http.StatusInternalServerError, BackupUploadResponse{
			Success: false,
			Error:   "Failed to upload file to storage",
		})
		return
	}

	slog.Info("File uploaded successfully", "file_id", fileID, "filename", header.Filename)

	// Create and enqueue job to Redis
	jobQueue, err := queue.NewQueue(c.Request.Context())
	if err != nil {
		slog.Error("Failed to create queue", "error", err)
		// File uploaded but couldn't queue job - still return success with file ID
		c.JSON(http.StatusOK, BackupUploadResponse{
			Success: true,
			FileID:  fileID,
			Message: fmt.Sprintf("File %s uploaded but job queue failed", header.Filename),
		})
		return
	}
	defer jobQueue.Close()

	// Create job with unique ID
	jobID := uuid.New().String()
	job := &queue.Job{
		ID:        jobID,
		FileID:    fileID,
		UserID:    userID,
		Filename:  header.Filename,
		CreatedAt: time.Now(),
	}

	if err := jobQueue.Enqueue(c.Request.Context(), job); err != nil {
		slog.Error("Failed to enqueue job", "error", err, "job_id", jobID)
		// File uploaded but couldn't queue job
		c.JSON(http.StatusOK, BackupUploadResponse{
			Success: true,
			FileID:  fileID,
			Message: fmt.Sprintf("File %s uploaded but job queue failed", header.Filename),
		})
		return
	}

	c.JSON(http.StatusOK, BackupUploadResponse{
		Success: true,
		FileID:  fileID,
		JobID:   jobID,
		Message: fmt.Sprintf("File %s uploaded and queued for processing", header.Filename),
	})
}
