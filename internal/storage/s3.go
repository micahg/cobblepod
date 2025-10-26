package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"google.golang.org/api/drive/v3"
)

// S3Storage implements the Storage interface using AWS S3 (or R2)
type S3Storage struct {
	client     *s3.Client
	bucket     string
	baseURL    string // For public URLs (e.g., R2 public URL)
	publicRead bool   // Whether to make uploaded files publicly readable
}

// S3Config holds configuration for S3 storage
type S3Config struct {
	Region      string
	Bucket      string
	AccessKey   string
	SecretKey   string
	EndpointURL string // For R2: https://account-id.r2.cloudflarestorage.com
	BaseURL     string // For public URLs: https://pub-bucket.r2.dev
	PublicRead  bool   // Whether to make files publicly readable
}

// NewS3Storage creates a new S3 storage implementation
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	var awsCfg aws.Config
	var err error

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		// Use explicit credentials
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKey, cfg.SecretKey, "",
			)),
			config.WithRegion(cfg.Region),
		)
	} else {
		// Use default credential chain
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Configure custom endpoint for R2
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
			o.UsePathStyle = true // R2 requires path-style addressing
		}
	})

	storage := &S3Storage{
		client:     client,
		bucket:     cfg.Bucket,
		baseURL:    cfg.BaseURL,
		publicRead: cfg.PublicRead,
	}

	// Test connection
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to access bucket %s: %w", cfg.Bucket, err)
	}

	slog.Info("S3/R2 storage initialized", "bucket", cfg.Bucket, "endpoint", cfg.EndpointURL)
	return storage, nil
}

// GenerateDownloadURL generates a public download URL for a file
func (s *S3Storage) GenerateDownloadURL(fileID string) string {
	if s.baseURL != "" {
		// Use public R2 URL if configured
		return fmt.Sprintf("%s/%s", strings.TrimRight(s.baseURL, "/"), fileID)
	}
	// Generate presigned URL (valid for 1 hour)
	presignClient := s3.NewPresignClient(s.client)
	request, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fileID),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Hour
	})
	if err != nil {
		slog.Error("Failed to generate presigned URL", "key", fileID, "error", err)
		return ""
	}
	return request.URL
}

// ExtractFileIDFromURL extracts file ID from URL
// For S3/R2, this is typically the last path segment
func (s *S3Storage) ExtractFileIDFromURL(url string) string {
	// Handle R2 public URLs
	if s.baseURL != "" && strings.HasPrefix(url, s.baseURL) {
		return strings.TrimPrefix(url, strings.TrimRight(s.baseURL, "/")+"/")
	}

	// Handle presigned URLs or other S3 URLs
	re := regexp.MustCompile(`[^/]+$`)
	matches := re.FindString(url)
	return matches
}

// GetFiles lists files matching a pattern (simplified for S3)
func (s *S3Storage) GetFiles(query string, mostRecent bool) ([]*drive.File, error) {
	ctx := context.TODO()

	// Convert Drive query to S3 prefix matching
	// This is a simplified implementation - real query parsing would be more complex
	var prefix string
	if strings.Contains(query, "name contains '.m3u'") {
		prefix = "" // List all, filter later
	} else if strings.Contains(query, "name = 'playrun_addict.xml'") {
		prefix = "playrun_addict.xml"
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	if mostRecent {
		input.MaxKeys = aws.Int32(1000) // Get many to find most recent
	}

	result, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	var files []*drive.File
	for _, obj := range result.Contents {
		// Filter based on query
		if strings.Contains(query, ".m3u") && !strings.Contains(*obj.Key, ".m3u") {
			continue
		}

		files = append(files, &drive.File{
			Id:           *obj.Key,
			Name:         *obj.Key,
			ModifiedTime: obj.LastModified.Format(time.RFC3339),
		})
	}

	// Sort by modified time if mostRecent requested
	if mostRecent && len(files) > 0 {
		// Return only the most recent
		mostRecentFile := s.GetMostRecentFile(files)
		if mostRecentFile != nil {
			return []*drive.File{mostRecentFile}, nil
		}
	}

	return files, nil
}

// GetMostRecentFile finds the most recently modified file
func (s *S3Storage) GetMostRecentFile(files []*drive.File) *drive.File {
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

// FileExists checks if a file exists in S3
func (s *S3Storage) FileExists(fileID string) (bool, error) {
	if fileID == "" {
		return false, fmt.Errorf("file ID is empty")
	}

	ctx := context.TODO()
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fileID),
	})

	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if file exists: %w", err)
	}

	return true, nil
}

// DeleteFile deletes a file from S3
func (s *S3Storage) DeleteFile(fileID string) error {
	if fileID == "" {
		return fmt.Errorf("file ID is empty")
	}

	ctx := context.TODO()
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fileID),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file %s: %w", fileID, err)
	}

	return nil
}

// DownloadFile downloads a file and returns its content as a string
func (s *S3Storage) DownloadFile(fileID string) (string, error) {
	ctx := context.TODO()
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fileID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to download file %s: %w", fileID, err)
	}
	defer result.Body.Close()

	content, err := io.ReadAll(result.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(content), nil
}

// DownloadFileToTemp downloads a file to a temporary file and returns the local path
func (s *S3Storage) DownloadFileToTemp(fileID string) (string, error) {
	ctx := context.TODO()
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fileID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to download file %s: %w", fileID, err)
	}
	defer result.Body.Close()

	tmpFile, err := os.CreateTemp("", "s3-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, result.Body); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// UploadFile uploads a file to S3
func (s *S3Storage) UploadFile(filePath, filename, mimeType string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return s.uploadReader(file, filename, mimeType)
}

// UploadString uploads a string as a file to S3
func (s *S3Storage) UploadString(content, filename, mimeType, fileID string) (string, error) {
	reader := strings.NewReader(content)

	// Use provided fileID or filename as the key
	key := filename
	if fileID != "" {
		key = fileID
	}

	return s.uploadReader(reader, key, mimeType)
}

// uploadReader handles the actual upload to S3
func (s *S3Storage) uploadReader(reader io.Reader, key, mimeType string) (string, error) {
	ctx := context.TODO()

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}

	if mimeType != "" {
		input.ContentType = aws.String(mimeType)
	}

	// Make file publicly readable if configured
	if s.publicRead {
		input.ACL = types.ObjectCannedACLPublicRead
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	slog.Info("File uploaded successfully", "key", key, "bucket", s.bucket)
	return key, nil
}

// Helper function to create S3Storage from environment variables
func NewS3StorageFromEnv(ctx context.Context) (*S3Storage, error) {
	cfg := S3Config{
		Region:      getEnv("AWS_REGION", "auto"),
		Bucket:      getEnv("S3_BUCKET", ""),
		AccessKey:   getEnv("AWS_ACCESS_KEY_ID", ""),
		SecretKey:   getEnv("AWS_SECRET_ACCESS_KEY", ""),
		EndpointURL: getEnv("AWS_ENDPOINT_URL", ""),
		BaseURL:     getEnv("S3_BASE_URL", ""),
		PublicRead:  getEnv("S3_PUBLIC_READ", "true") == "true",
	}

	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET environment variable is required")
	}

	return NewS3Storage(ctx, cfg)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
