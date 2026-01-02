package storage

import (
	"context"

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
	GetFiles(ctx context.Context, query string, mostRecent bool) ([]*drive.File, error)
	GetMostRecentFile(files []*drive.File) *drive.File
	FileExists(ctx context.Context, fileID string) (bool, error)
	DeleteFile(ctx context.Context, fileID string) error

	// File content operations
	DownloadFile(ctx context.Context, fileID string) (string, error)
	DownloadFileToTemp(ctx context.Context, fileID string) (string, error)
	UploadFile(ctx context.Context, filePath, filename, mimeType string) (string, error)
	UploadString(ctx context.Context, content, filename, mimeType, fileID string) (string, error)
}
