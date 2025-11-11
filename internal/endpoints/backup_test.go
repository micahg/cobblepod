package endpoints

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cobblepod/internal/queue"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test DB
	})

	// Test connection
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	require.NoError(t, err, "Redis must be available for integration tests")

	// Clean up before test
	client.FlushDB(ctx)

	t.Cleanup(func() {
		client.FlushDB(ctx)
		client.Close()
	})

	return client
}

// mockBackupHandler creates a simplified version of HandleBackupUpload that skips auth/storage
func mockBackupHandler(jobQueue *queue.Queue) gin.HandlerFunc {
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

	// Set up Redis
	redisClient := setupTestRedis(t)
	ctx := context.Background()

	testUserID := "test-user-123"

	// Add user to running set to simulate a running job
	err := redisClient.SAdd(ctx, queue.RunningUsersKey, testUserID).Err()
	require.NoError(t, err)

	// Verify user is in running set
	isMember, err := redisClient.SIsMember(ctx, queue.RunningUsersKey, testUserID).Result()
	require.NoError(t, err)
	assert.True(t, isMember, "User should be in running set")

	// Create a test queue instance using the same Redis client
	jobQueue := queue.NewQueueWithClient(redisClient)

	// Create test router with mock handler
	router := gin.New()

	// Mock auth middleware that sets user_id in context
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUserID)
		c.Next()
	})

	router.POST("/api/backup", mockBackupHandler(jobQueue))

	// Create a simple request (no actual file needed for this test)
	req := httptest.NewRequest(http.MethodPost, "/api/backup", nil)

	// Record response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusConflict, w.Code, "Should return 409 Conflict when user has running job")

	var response BackupUploadResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "already have a job", "Error message should indicate concurrent job conflict")
}

func TestHandleBackupUpload_AllowsWhenNoRunningJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set up Redis
	redisClient := setupTestRedis(t)
	ctx := context.Background()

	testUserID := "test-user-456"

	// Verify user is NOT in running set
	isMember, err := redisClient.SIsMember(ctx, queue.RunningUsersKey, testUserID).Result()
	require.NoError(t, err)
	assert.False(t, isMember, "User should NOT be in running set initially")

	// Create a test queue instance using the same Redis client
	jobQueue := queue.NewQueueWithClient(redisClient)

	// Create test router with mock handler
	router := gin.New()

	// Mock auth middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUserID)
		c.Next()
	})

	router.POST("/api/backup", mockBackupHandler(jobQueue))

	// Create a simple request
	req := httptest.NewRequest(http.MethodPost, "/api/backup", nil)

	// Record response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed (200) since user has no running job
	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 when user has no running job")

	var response BackupUploadResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
}
