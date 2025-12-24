package endpoints

import (
	"context"
	"net/http"

	"cobblepod/internal/queue"

	"github.com/gin-gonic/gin"
)

// JobQueue defines the interface for job queue operations
type JobQueue interface {
	GetWaitingJobs(ctx context.Context, userID string) ([]*queue.Job, error)
	GetRunningJobs(ctx context.Context, userID string) ([]*queue.Job, error)
	GetFailedJobs(ctx context.Context, userID string) ([]*queue.Job, error)
	GetCompletedJobs(ctx context.Context, userID string) ([]*queue.Job, error)
	GetJobItems(ctx context.Context, jobID string) ([]queue.JobItem, error)
}

// GetJobsResponse represents the response for the jobs endpoint
type GetJobsResponse struct {
	Jobs []*queue.Job `json:"jobs"`
}

// HandleGetJobs returns a handler that retrieves jobs based on status
// @Summary      Get jobs
// @Description  Get a list of jobs for the authenticated user, optionally filtered by status
// @Tags         jobs
// @Produce      json
// @Param        status query string false "Job status filter"
// @Success      200  {object}  GetJobsResponse
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /jobs [get]
func HandleGetJobs(jobQueue JobQueue) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.Query("status")
		var jobs []*queue.Job
		ctx := c.Request.Context()

		userID, err := GetUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		if status == "" {
			waiting, err := jobQueue.GetWaitingJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch waiting jobs"})
				return
			}
			jobs = append(jobs, waiting...)

			running, err := jobQueue.GetRunningJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch running jobs"})
				return
			}
			jobs = append(jobs, running...)
		} else if status == "failed" {
			failed, err := jobQueue.GetFailedJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch failed jobs"})
				return
			}
			jobs = append(jobs, failed...)
		} else if status == "completed" {
			completed, err := jobQueue.GetCompletedJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch completed jobs"})
				return
			}
			jobs = append(jobs, completed...)
		}

		c.JSON(http.StatusOK, GetJobsResponse{Jobs: jobs})
	}
}

// GetJobItemsResponse represents the response for the job items endpoint
type GetJobItemsResponse struct {
	Items []queue.JobItem `json:"items"`
}

// HandleGetJobItems returns a handler that retrieves items for a specific job
// @Summary      Get job items
// @Description  Get a list of items for a specific job
// @Tags         jobs
// @Produce      json
// @Param        id path string true "Job ID"
// @Success      200  {object}  GetJobItemsResponse
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /jobs/{id}/items [get]
func HandleGetJobItems(jobQueue JobQueue) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("id")
		ctx := c.Request.Context()

		_, err := GetUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		items, err := jobQueue.GetJobItems(ctx, jobID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch job items"})
			return
		}

		c.JSON(http.StatusOK, GetJobItemsResponse{Items: items})
	}
}
