package endpoints

import (
	"net/http"

	"cobblepod/internal/queue"

	"github.com/gin-gonic/gin"
)

// GetJobsResponse represents the response for the jobs endpoint
type GetJobsResponse struct {
	Waiting   []*queue.Job `json:"waiting,omitempty"`
	Running   []*queue.Job `json:"running,omitempty"`
	Completed []*queue.Job `json:"completed,omitempty"`
	Failed    []*queue.Job `json:"failed,omitempty"`
}

// HandleGetJobs returns a handler that retrieves jobs based on status
func HandleGetJobs(jobQueue *queue.Queue) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.Query("status")
		response := GetJobsResponse{}
		ctx := c.Request.Context()

		userID, err := GetUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// If status is "active" or empty, fetch waiting and running jobs
		if status == "active" || status == "" {
			response.Waiting, err = jobQueue.GetWaitingJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch waiting jobs"})
				return
			}

			response.Running, err = jobQueue.GetRunningJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch running jobs"})
				return
			}
		}

		// If status is "inactive" or empty, fetch completed and failed jobs
		if status == "inactive" || status == "" {
			response.Completed, err = jobQueue.GetCompletedJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch completed jobs"})
				return
			}

			response.Failed, err = jobQueue.GetFailedJobs(ctx, userID)
			if err != nil {
				if err == queue.ErrUserIDRequired {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch failed jobs"})
				return
			}
		}

		c.JSON(http.StatusOK, response)
	}
}
