package storage

import (
	"google.golang.org/api/drive/v3"
)

// Storage defines the interface for cloud storage operations.
// This interface abstracts cloud storage functionality to allow for
// different storage backend implementations while maintaining the same API.
// The current implementation uses Google Drive, but this interface allows
// for easy swapping to other storage providers like AWS S3, Azure Blob, etc.
type Storage interface {
	// File management operations
	GenerateDownloadURL(driveID string) string
	ExtractFileIDFromURL(url string) string
	GetFiles(query string, mostRecent bool) ([]*drive.File, error)
	GetMostRecentFile(files []*drive.File) *drive.File
	FileExists(fileID string) (bool, error)
	DeleteFile(fileID string) error

	// File content operations
	DownloadFile(fileID string) (string, error)
	DownloadFileToTemp(fileID string) (string, error)
	UploadFile(filePath, filename, mimeType string) (string, error)
	UploadString(content, filename, mimeType, fileID string) (string, error)
}
