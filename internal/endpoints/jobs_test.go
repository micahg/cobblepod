package endpoints

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"cobblepod/internal/queue"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockJobQueue is a mock implementation of JobQueue
type MockJobQueue struct {
	mock.Mock
}

func (m *MockJobQueue) GetWaitingJobs(ctx context.Context, userID string) ([]*queue.Job, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*queue.Job), args.Error(1)
}

func (m *MockJobQueue) GetRunningJobs(ctx context.Context, userID string) ([]*queue.Job, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*queue.Job), args.Error(1)
}

func (m *MockJobQueue) GetFailedJobs(ctx context.Context, userID string) ([]*queue.Job, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*queue.Job), args.Error(1)
}

func (m *MockJobQueue) GetCompletedJobs(ctx context.Context, userID string) ([]*queue.Job, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*queue.Job), args.Error(1)
}

func TestHandleGetJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Unauthorized", func(t *testing.T) {
		mockQueue := new(MockJobQueue)
		router := gin.New()
		router.GET("/jobs", HandleGetJobs(mockQueue))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/jobs", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Success - All Jobs", func(t *testing.T) {
		mockQueue := new(MockJobQueue)
		router := gin.New()
		// Mock middleware to set user_id
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "test-user")
			c.Next()
		})
		router.GET("/jobs", HandleGetJobs(mockQueue))

		waitingJobs := []*queue.Job{{ID: "1", Status: "waiting"}}
		runningJobs := []*queue.Job{{ID: "2", Status: "running"}}

		mockQueue.On("GetWaitingJobs", mock.Anything, "test-user").Return(waitingJobs, nil)
		mockQueue.On("GetRunningJobs", mock.Anything, "test-user").Return(runningJobs, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/jobs", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response GetJobsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Jobs, 2)
		mockQueue.AssertExpectations(t)
	})

	t.Run("Success - Failed Jobs", func(t *testing.T) {
		mockQueue := new(MockJobQueue)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "test-user")
			c.Next()
		})
		router.GET("/jobs", HandleGetJobs(mockQueue))

		failedJobs := []*queue.Job{{ID: "3", Status: "failed"}}

		mockQueue.On("GetFailedJobs", mock.Anything, "test-user").Return(failedJobs, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/jobs?status=failed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response GetJobsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Jobs, 1)
		assert.Equal(t, "failed", response.Jobs[0].Status)
		mockQueue.AssertExpectations(t)
	})

	t.Run("Success - Completed Jobs", func(t *testing.T) {
		mockQueue := new(MockJobQueue)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "test-user")
			c.Next()
		})
		router.GET("/jobs", HandleGetJobs(mockQueue))

		completedJobs := []*queue.Job{{ID: "4", Status: "completed"}}

		mockQueue.On("GetCompletedJobs", mock.Anything, "test-user").Return(completedJobs, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/jobs?status=completed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response GetJobsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Jobs, 1)
		assert.Equal(t, "completed", response.Jobs[0].Status)
		mockQueue.AssertExpectations(t)
	})

	t.Run("Error - GetWaitingJobs", func(t *testing.T) {
		mockQueue := new(MockJobQueue)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "test-user")
			c.Next()
		})
		router.GET("/jobs", HandleGetJobs(mockQueue))

		mockQueue.On("GetWaitingJobs", mock.Anything, "test-user").Return([]*queue.Job{}, errors.New("db error"))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/jobs", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockQueue.AssertExpectations(t)
	})
}
