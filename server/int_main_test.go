package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cobblepod/internal/processor"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// TODO move this to testing
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
func TestBigOne(t *testing.T) {
	fmt.Println("This is a placeholder test in int_main_test.go")
	t.Run("BIG ONE", func(t *testing.T) {
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

		ctx := context.Background()

		_, err := drive.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(mockServer.URL))
		if err != nil {
			t.Fatalf("Failed to create drive service: %v", err)
		}
		fmt.Println("Running BIG ONE test")

		proc, err := processor.NewProcessor(context.Background())
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		proc.Run(context.Background())
	})
}
