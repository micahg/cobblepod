package storage

import (
	"time"
)

// FileInfo represents a file in any storage backend
type FileInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ModifiedTime time.Time `json:"modified_time"`
	Size         int64     `json:"size,omitempty"`
	MimeType     string    `json:"mime_type,omitempty"`
}

// CommonStorage defines the interface for cloud storage operations using common types.
// This interface abstracts cloud storage functionality to allow for
// different storage backend implementations while maintaining the same API.
type CommonStorage interface {
	// File management operations
	GenerateDownloadURL(fileID string) string
	ExtractFileIDFromURL(url string) string
	GetFiles(query string, mostRecent bool) ([]*FileInfo, error)
	GetMostRecentFile(files []*FileInfo) *FileInfo
	FileExists(fileID string) (bool, error)
	DeleteFile(fileID string) error

	// File content operations
	DownloadFile(fileID string) (string, error)
	DownloadFileToTemp(fileID string) (string, error)
	UploadFile(filePath, filename, mimeType string) (string, error)
	UploadString(content, filename, mimeType, fileID string) (string, error)
}
