package gdrive

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"cobblepod/internal/config"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Service wraps the Google Drive API service
type Service struct {
	drive *drive.Service
}

// NewService creates a new Google Drive service
func NewService(ctx context.Context) (*Service, error) {
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

	log.Printf("Google Drive service initialized with project: %s", config.ProjectID)
	return &Service{drive: service}, nil
}

// GenerateDownloadURL converts a Google Drive file ID to a direct download URL
func (s *Service) GenerateDownloadURL(driveID string) string {
	return fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s&export=download&authuser=0&confirm=t", driveID)
}

// ExtractFileIDFromURL extracts the file ID from a Google Drive download URL
func (s *Service) ExtractFileIDFromURL(url string) string {
	re := regexp.MustCompile(`id=([a-zA-Z0-9_-]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// GetFiles searches for files matching the given query
func (s *Service) GetFiles(query string, mostRecent bool) ([]*drive.File, error) {
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
func (s *Service) GetMostRecentFile(files []*drive.File) *drive.File {
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
			log.Printf("Could not parse modifiedTime '%s' for file %s: %v", file.ModifiedTime, file.Name, err)
			continue
		}

		if mostRecent == nil || modifiedTime.After(mostRecentTime) {
			mostRecentTime = modifiedTime
			mostRecent = file
		}
	}

	return mostRecent
}

// DownloadFile downloads a file and returns its content as a string
func (s *Service) DownloadFile(fileID string) (string, error) {
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
func (s *Service) DownloadFileToTemp(fileID string) (string, error) {
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
func (s *Service) UploadFile(filePath, filename, mimeType string) (string, error) {
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

	log.Printf("File uploaded successfully: %s (ID: %s)", filename, createdFile.Id)

	// Set permissions
	if err := s.setFilePermissions(createdFile.Id, filename); err != nil {
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}

	return createdFile.Id, nil
}

// UploadString uploads a string as a file to Google Drive
func (s *Service) UploadString(content, filename, mimeType, fileID string) (string, error) {
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
func (s *Service) setFilePermissions(fileID, filename string) error {
	permission := &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}

	log.Printf("Setting permissions for %s (ID: %s)", filename, fileID)
	_, err := s.drive.Permissions.Create(fileID, permission).Do()
	return err
}
