package sources

import (
	"cobblepod/internal/storage"
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/api/drive/v3"
)

// FileInfo contains metadata about a file (M3U8, backup, etc.)
type FileInfo struct {
	File         *drive.File
	FileName     string
	ModifiedTime time.Time
}

// AudioEntry represents an audio entry from various sources (M3U8 playlist, backup, etc.)
type AudioEntry struct {
	Title    string        `json:"title"`
	Duration time.Duration `json:"duration"`
	URL      string        `json:"url"`
	UUID     string        `json:"uuid"`
	Offset   time.Duration `json:"offset,omitempty"` // Listening offset
}

// GetLatestFile is a common function to get the most recent file matching a query
func GetLatestFile(ctx context.Context, drive *storage.GDrive, query string, fileTypeName string) (*FileInfo, error) {
	files, err := drive.GetFiles(query, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s files: %w", fileTypeName, err)
	}

	if len(files) == 0 {
		return nil, nil // No files found
	}

	mostRecentFile := drive.GetMostRecentFile(files)
	if mostRecentFile == nil {
		return nil, nil
	}

	// Parse the modified time
	modifiedTime, err := time.Parse(time.RFC3339, mostRecentFile.ModifiedTime)
	if err != nil {
		slog.Warn("Couldn't parse modified time", "file", mostRecentFile.Name, "error", err)
		modifiedTime = time.Time{} // Zero time as fallback
	}

	return &FileInfo{
		File:         mostRecentFile,
		ModifiedTime: modifiedTime,
		FileName:     mostRecentFile.Name,
	}, nil
}
