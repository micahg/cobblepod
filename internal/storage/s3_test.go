//go:build integration

package storage

import (
	"context"
	"testing"
)

func TestS3StorageIntegration(t *testing.T) {
	// This test requires actual R2/S3 credentials
	// Set these environment variables to run:
	// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_ENDPOINT_URL, S3_BUCKET

	ctx := context.Background()
	storage, err := NewS3StorageFromEnv(ctx)
	if err != nil {
		t.Skipf("Skipping S3 integration test: %v", err)
	}

	// Test file operations
	t.Run("upload and download string", func(t *testing.T) {
		content := "Hello, R2!"
		filename := "test-file.txt"

		// Upload
		fileID, err := storage.UploadString(content, filename, "text/plain", "")
		if err != nil {
			t.Fatalf("Failed to upload string: %v", err)
		}

		// Download
		downloaded, err := storage.DownloadFile(fileID)
		if err != nil {
			t.Fatalf("Failed to download file: %v", err)
		}

		if downloaded != content {
			t.Errorf("Downloaded content mismatch: got %q, want %q", downloaded, content)
		}

		// Clean up
		err = storage.DeleteFile(fileID)
		if err != nil {
			t.Errorf("Failed to delete file: %v", err)
		}
	})

	t.Run("file exists", func(t *testing.T) {
		// Upload a test file
		fileID, err := storage.UploadString("test", "exists-test.txt", "text/plain", "")
		if err != nil {
			t.Fatalf("Failed to upload test file: %v", err)
		}
		defer storage.DeleteFile(fileID)

		// Check it exists
		exists, err := storage.FileExists(fileID)
		if err != nil {
			t.Fatalf("Failed to check file existence: %v", err)
		}

		if !exists {
			t.Error("File should exist but doesn't")
		}

		// Check non-existent file
		exists, err = storage.FileExists("non-existent-file")
		if err != nil {
			t.Fatalf("Failed to check non-existent file: %v", err)
		}

		if exists {
			t.Error("Non-existent file should not exist")
		}
	})

	t.Run("generate download URL", func(t *testing.T) {
		fileID := "test-file.txt"
		url := storage.GenerateDownloadURL(fileID)

		if url == "" {
			t.Error("Generated URL should not be empty")
		}

		t.Logf("Generated URL: %s", url)
	})
}
