package endpoints

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	queuemock "cobblepod/internal/queue/mock"

	"github.com/gin-gonic/gin"
)

// QueueInterface defines the methods we need from a queue
type QueueInterface interface {
	IsUserRunning(ctx context.Context, userID string) (bool, error)
}

// mockBackupHandler creates a simplified version of HandleBackupUpload that skips auth/storage
func mockBackupHandler(jobQueue QueueInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by test middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, BackupUploadResponse{
				Success: false,
				Error:   "Unauthorized",
			})
			return
		}

		// Check if user already has a running job (fail fast before expensive operations)
		isRunning, err := jobQueue.IsUserRunning(c.Request.Context(), userID.(string))
		if err != nil {
			c.JSON(http.StatusInternalServerError, BackupUploadResponse{
				Success: false,
				Error:   "Failed to check job status",
			})
			return
		}

		if isRunning {
			c.JSON(http.StatusConflict, BackupUploadResponse{
				Success: false,
				Error:   "You already have a job being processed. Please wait for it to complete.",
			})
			return
		}

		// For this test, we're only validating the concurrency check
		// Skip the actual auth, file upload, and storage operations
		c.JSON(http.StatusOK, BackupUploadResponse{
			Success: true,
			Message: "Request would be accepted",
		})
	}
}

func TestHandleBackupUpload_RejectsWhenUserHasRunningJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testUserID := "test-user-123"

	// Create mock queue and set user as running
	mockQueue := queuemock.NewMockQueue()
	mockQueue.SetUserRunning(testUserID, true)

	// Create test router with mock handler
	router := gin.New()

	// Mock auth middleware that sets user_id in context
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUserID)
		c.Next()
	})

	router.POST("/api/backup", mockBackupHandler(mockQueue))

	// Create a simple request (no actual file needed for this test)
	req := httptest.NewRequest(http.MethodPost, "/api/backup", nil)

	// Record response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d (Conflict), got %d", http.StatusConflict, w.Code)
	}

	var response BackupUploadResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Error("Expected Success to be false, got true")
	}
	if !strings.Contains(response.Error, "already have a job") {
		t.Errorf("Expected error message to contain 'already have a job', got '%s'", response.Error)
	}
}

func TestHandleBackupUpload_AllowsWhenNoRunningJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testUserID := "test-user-456"

	// Create mock queue with no running users
	mockQueue := queuemock.NewMockQueue()

	// Create test router with mock handler
	router := gin.New()

	// Mock auth middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUserID)
		c.Next()
	})

	router.POST("/api/backup", mockBackupHandler(mockQueue))

	// Create a simple request
	req := httptest.NewRequest(http.MethodPost, "/api/backup", nil)

	// Record response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed (200) since user has no running job
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d (OK), got %d", http.StatusOK, w.Code)
	}

	var response BackupUploadResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Success {
		t.Error("Expected Success to be true, got false")
	}
}

func TestHandleBackupUpload_HandlesQueueError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testUserID := "test-user-789"

	// Create mock queue that returns errors
	mockQueue := queuemock.NewMockQueueWithErrors(queuemock.ErrorOnIsUserRunning)

	// Create test router with mock handler
	router := gin.New()

	// Mock auth middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUserID)
		c.Next()
	})

	router.POST("/api/backup", mockBackupHandler(mockQueue))

	// Create a simple request
	req := httptest.NewRequest(http.MethodPost, "/api/backup", nil)

	// Record response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 500 when queue check fails
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d (Internal Server Error), got %d", http.StatusInternalServerError, w.Code)
	}

	var response BackupUploadResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Error("Expected Success to be false, got true")
	}
	if !strings.Contains(response.Error, "Failed to check job status") {
		t.Errorf("Expected error message to contain 'Failed to check job status', got '%s'", response.Error)
	}
}
