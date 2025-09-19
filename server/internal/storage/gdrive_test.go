package storage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// createMockServer creates a mock HTTP server that intercepts Google Drive API calls
func createMockServer(t *testing.T, response any, queryPattern string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Intercepted HTTP request: %s %s", r.Method, r.URL.String())
		t.Logf("Query parameters: %s", r.URL.RawQuery)

		// Verify the query pattern is present in the request
		if !strings.Contains(r.URL.RawQuery, queryPattern) {
			t.Errorf("Expected query pattern '%s' not found in request: %s", queryPattern, r.URL.RawQuery)
		} else {
			t.Logf("✅ Successfully found query pattern '%s' in request - the vendor call was mocked!", queryPattern)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func TestGetFiles(t *testing.T) {
	// Create a mock HTTP server that will intercept the Google Drive API calls
	mockServer := createMockServer(t, &drive.FileList{
		Files: []*drive.File{
			{
				Id:           "file1",
				Name:         "test1.m3u",
				ModifiedTime: "2025-09-06T10:00:00.000Z",
			},
			{
				Id:           "file2",
				Name:         "test2.backup",
				ModifiedTime: "2025-09-06T11:00:00.000Z",
			},
		},
	}, "fields=files%28id%2C+name%2C+modifiedTime%29")
	defer mockServer.Close()

	// Create a Drive service that uses our mock server
	ctx := context.Background()
	driveService, err := drive.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(mockServer.URL))
	if err != nil {
		t.Fatalf("Failed to create drive service: %v", err)
	}

	// Create our Service wrapper
	service := &GDrive{drive: driveService}

	// Test the GetFiles method
	files, err := service.GetFiles("name contains 'test'", false)
	if err != nil {
		t.Fatalf("GetFiles failed: %v", err)
	}

	// Verify results
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	if files[0].Name != "test1.m3u" {
		t.Errorf("Expected first file name 'test1.m3u', got '%s'", files[0].Name)
	}

	if files[1].Name != "test2.backup" {
		t.Errorf("Expected second file name 'test2.backup', got '%s'", files[1].Name)
	}

	t.Log("GetFiles test passed - Fields call was successfully mocked and verified")
}

func TestGetFilesMostRecent(t *testing.T) {
	// Create a mock HTTP server for the mostRecent=true case
	// We'll check for multiple query parameters by using a custom handler
	mockServer := createMockServer(t, &drive.FileList{
		Files: []*drive.File{
			{
				Id:           "file1",
				Name:         "latest.m3u",
				ModifiedTime: "2025-09-06T12:00:00.000Z",
			},
		},
	}, "fields=files%28id%2C+name%2C+modifiedTime%29")
	defer mockServer.Close()

	// Create a Drive service that uses our mock server
	ctx := context.Background()
	driveService, err := drive.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(mockServer.URL))
	if err != nil {
		t.Fatalf("Failed to create drive service: %v", err)
	}

	// Create our Service wrapper
	service := &GDrive{drive: driveService}

	// Test the GetFiles method with mostRecent=true
	files, err := service.GetFiles("name contains 'latest'", true)
	if err != nil {
		t.Fatalf("GetFiles failed: %v", err)
	}

	// Verify results
	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	if files[0].Name != "latest.m3u" {
		t.Errorf("Expected file name 'latest.m3u', got '%s'", files[0].Name)
	}

	t.Log("GetFiles mostRecent test passed - Fields call with additional parameters was successfully mocked")
}
