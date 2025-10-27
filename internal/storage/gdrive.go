package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"cobblepod/internal/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// GDrive wraps the Google Drive API service and implements the Storage interface
type GDrive struct {
	drive *drive.Service
	// For multi-user scenarios, store context needed to create per-user clients
	ctx context.Context
}

// NewServiceWithDefaultCredentials creates a new Google Drive service using default credentials
// This is used by the worker which runs with service account credentials
func NewServiceWithDefaultCredentials(ctx context.Context) (*GDrive, error) {
	credentials, err := google.FindDefaultCredentials(ctx, config.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}

	if config.ProjectID == "" {
		config.ProjectID = credentials.ProjectID
	}

	service, err := drive.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service: %w", err)
	}

	slog.Info("Google Drive service initialized", "project_id", config.ProjectID)
	return &GDrive{drive: service, ctx: ctx}, nil
}

// NewServiceWithToken creates a new Google Drive service using an OAuth2 token
// This creates a per-request client for a specific user
func NewServiceWithToken(ctx context.Context, accessToken string) (*GDrive, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("access token is required")
	}

	// Create an OAuth2 token
	token := &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	}

	// Create OAuth2 token source
	tokenSource := oauth2.StaticTokenSource(token)

	// Create Drive service with the token
	service, err := drive.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service with token: %w", err)
	}

	slog.Info("Google Drive service initialized with OAuth token")
	return &GDrive{drive: service, ctx: ctx}, nil
}

func NewServiceWithClient(client *drive.Service) *GDrive {
	return &GDrive{drive: client, ctx: context.Background()}
}

// GenerateDownloadURL converts a Google Drive file ID to a direct download URL
func (s *GDrive) GenerateDownloadURL(driveID string) string {
	return fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s&export=download&authuser=0&confirm=t", driveID)
}

// ExtractFileIDFromURL extracts the file ID from a Google Drive download URL
func (s *GDrive) ExtractFileIDFromURL(url string) string {
	re := regexp.MustCompile(`id=([a-zA-Z0-9_-]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// GetFiles searches for files matching the given query
func (s *GDrive) GetFiles(query string, mostRecent bool) ([]*drive.File, error) {
	call := s.drive.Files.List().Q(query).Fields("files(id, name, modifiedTime)")

	if mostRecent {
		call = call.OrderBy("modifiedTime desc").PageSize(1)
	}

	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return result.Files, nil
}

// GetMostRecentFile gets the most recently modified file from a list
func (s *GDrive) GetMostRecentFile(files []*drive.File) *drive.File {
	if len(files) == 0 {
		return nil
	}

	var mostRecent *drive.File
	var mostRecentTime time.Time

	for _, file := range files {
		if file.ModifiedTime == "" {
			continue
		}

		modifiedTime, err := time.Parse(time.RFC3339, file.ModifiedTime)
		if err != nil {
			slog.Warn("Could not parse modifiedTime", "time", file.ModifiedTime, "file", file.Name, "error", err)
			continue
		}

		if mostRecent == nil || modifiedTime.After(mostRecentTime) {
			mostRecentTime = modifiedTime
			mostRecent = file
		}
	}

	return mostRecent
}

// FileExists checks if a file with the given ID exists on Google Drive
func (s *GDrive) FileExists(fileID string) (bool, error) {
	if fileID == "" {
		return false, fmt.Errorf("file ID is empty")
	}

	_, err := s.drive.Files.Get(fileID).Fields("id").Do()
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "File not found") {
			return false, nil
		}
		// Return other errors (network issues, auth problems, etc.)
		return false, fmt.Errorf("failed to check if file exists: %w", err)
	}

	return true, nil
}

// DeleteFile deletes a file from Google Drive by ID
func (s *GDrive) DeleteFile(fileID string) error {
	if fileID == "" {
		return fmt.Errorf("file ID is empty")
	}

	err := s.drive.Files.Delete(fileID).Do()
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "File not found") {
			return fmt.Errorf("file not found: %s", fileID)
		}
		return fmt.Errorf("failed to delete file %s: %w", fileID, err)
	}

	return nil
}

// DownloadFile downloads a file and returns its content as a string
func (s *GDrive) DownloadFile(fileID string) (string, error) {
	resp, err := s.drive.Files.Get(fileID).Download()
	if err != nil {
		return "", fmt.Errorf("failed to download file %s: %w", fileID, err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(content), nil
}

// DownloadFileToTemp downloads a Drive file to a temporary file and returns the local path.
// Caller is responsible for removing the file when done.
func (s *GDrive) DownloadFileToTemp(fileID string) (string, error) {
	resp, err := s.drive.Files.Get(fileID).Download()
	if err != nil {
		return "", fmt.Errorf("failed to download file %s: %w", fileID, err)
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp("", "gdrive-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// UploadFile uploads a file to Google Drive
func (s *GDrive) UploadFile(filePath, filename, mimeType string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileMetadata := &drive.File{
		Name: filename,
	}

	// Create the file with content
	createdFile, err := s.drive.Files.Create(fileMetadata).Media(file).Fields("id").Do()
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	slog.Info("File uploaded successfully", "filename", filename, "id", createdFile.Id)

	// Set permissions
	if err := s.setFilePermissions(createdFile.Id, filename); err != nil {
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}

	return createdFile.Id, nil
}

// UploadString uploads a string as a file to Google Drive
func (s *GDrive) UploadString(content, filename, mimeType, fileID string) (string, error) {
	fileMetadata := &drive.File{
		Name: filename,
	}

	reader := strings.NewReader(content)

	var file *drive.File
	var err error

	if fileID != "" {
		// Update existing file
		file, err = s.drive.Files.Update(fileID, fileMetadata).Media(reader).Fields("id").Do()
	} else {
		// Create new file
		file, err = s.drive.Files.Create(fileMetadata).Media(reader).Fields("id").Do()
	}

	if err != nil {
		return "", fmt.Errorf("failed to upload string content: %w", err)
	}

	// Set permissions
	if err := s.setFilePermissions(file.Id, filename); err != nil {
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}

	return file.Id, nil
}

// setFilePermissions sets file permissions to be readable by anyone with the link
func (s *GDrive) setFilePermissions(fileID, filename string) error {
	permission := &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}

	slog.Info("Setting permissions", "filename", filename, "id", fileID)
	_, err := s.drive.Permissions.Create(fileID, permission).Do()
	return err
}
