package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"cobblepod/internal/config"

	"github.com/redis/go-redis/v9"
)

const (
	// WaitingQueue is the Redis list key for job queue (stores IDs)
	WaitingQueue = "cobblepod:waiting"
	// RunningUsersKey is the Redis hash key for users with running jobs (UserID -> JobID)
	RunningUsersKey = "cobblepod:running-users"
	// RunningQueue is the Redis set key for running job IDs
	RunningQueue = "cobblepod:running"
	// SuccessSet is the Redis set key for successful job IDs
	SuccessSet = "cobblepod:success"
	// FailedSet is the Redis set key for failed job IDs
	FailedSet = "cobblepod:failed"
	// CleanupSet is the Redis sorted set key for expiration tracking
	CleanupSet = "cobblepod:cleanup"
	// BlockTimeout is how long BRPOP will wait for a job
	BlockTimeout = 5 * time.Second
	// JobRetention is how long jobs are kept
	JobRetention = 7 * 24 * time.Hour
)

// JobItemStatus represents the state of a single item
type JobItemStatus string

const (
	StatusPending     JobItemStatus = "pending"
	StatusDownloading JobItemStatus = "downloading"
	StatusProcessing  JobItemStatus = "processing" // ffmpeg
	StatusUploading   JobItemStatus = "uploading"
	StatusCompleted   JobItemStatus = "completed"
	StatusSkipped     JobItemStatus = "skipped" // reused
	StatusFailed      JobItemStatus = "failed"
)

// JobItem represents a single item (episode) in a job
type JobItem struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Status    JobItemStatus `json:"status"`
	SourceURL string        `json:"source_url"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Offset    time.Duration `json:"offset,omitempty"`
}

// Job represents a backup processing job
type Job struct {
	ID         string    `json:"id" redis:"id"`
	FileID     string    `json:"file_id" redis:"file_id"`
	UserID     string    `json:"user_id,omitempty" redis:"user_id"`
	Filename   string    `json:"filename,omitempty" redis:"filename"`
	CreatedAt  time.Time `json:"created_at" redis:"created_at"`
	FailReason string    `json:"fail_reason,omitempty" redis:"fail_reason"` // Set when job fails
	Status     string    `json:"status" redis:"status"`                     // queued, running, completed, failed
	Items      []JobItem `json:"items" redis:"-"`                           // Items are stored in a separate hash
}

// Queue manages the Redis job queue
type Queue struct {
	client *redis.Client
}

// NewQueue creates a new queue connection
func NewQueue(ctx context.Context) (*Queue, error) {
	addr := fmt.Sprintf("%s:%d", config.ValkeyHost, config.ValkeyPort)
	slog.Debug("Connecting to Redis queue", "addr", addr)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // Add to config if needed
		DB:       0,
	})

	// Test the connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	slog.Info("Redis queue initialized", "addr", addr)
	return &Queue{client: client}, nil
}

// NewQueueWithClient creates a queue with an existing Redis client (for testing)
func NewQueueWithClient(client *redis.Client) *Queue {
	return &Queue{client: client}
}

// jobKey returns the Redis key for a job
func jobKey(jobID string) string {
	return fmt.Sprintf("cobblepod:job:%s", jobID)
}

// jobItemsKey returns the Redis key for a job's items
func jobItemsKey(jobID string) string {
	return fmt.Sprintf("cobblepod:job:%s:items", jobID)
}

// userJobsKey returns the Redis key for a user's job set
func userJobsKey(userID string) string {
	return fmt.Sprintf("cobblepod:user:%s:jobs", userID)
}

// IsUserRunning checks if a user already has a running job
func (q *Queue) IsUserRunning(ctx context.Context, userID string) (bool, error) {
	if q.client == nil {
		return false, fmt.Errorf("queue is not connected")
	}

	// Check if user exists in running hash
	exists, err := q.client.HExists(ctx, RunningUsersKey, userID).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check running users: %w", err)
	}

	return exists, nil
}

// Enqueue adds a job to the queue
func (q *Queue) Enqueue(ctx context.Context, job *Job) error {
	if q.client == nil {
		return fmt.Errorf("queue is not connected")
	}

	job.Status = "queued"
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}

	pipe := q.client.Pipeline()

	// 1. Store job data in Hash
	pipe.HSet(ctx, jobKey(job.ID), job)

	// 2. Store items if any
	if len(job.Items) > 0 {
		for _, item := range job.Items {
			itemJSON, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal item: %w", err)
			}
			pipe.HSet(ctx, jobItemsKey(job.ID), item.ID, itemJSON)
		}
	}

	// 3. Add to User's Job History Set
	if job.UserID != "" {
		pipe.SAdd(ctx, userJobsKey(job.UserID), job.ID)
	}

	// 4. Push ID to Waiting Queue
	pipe.LPush(ctx, WaitingQueue, job.ID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	slog.Info("Job enqueued", "job_id", job.ID, "file_id", job.FileID)
	return nil
}

// Dequeue removes and returns a job from the queue
// This blocks for up to BlockTimeout waiting for a job
func (q *Queue) Dequeue(ctx context.Context) (*Job, error) {
	if q.client == nil {
		return nil, fmt.Errorf("queue is not connected")
	}

	// Pop from right of list (BRPOP = blocking pop from end of queue)
	// Returns [key, value] where value is the job ID
	result, err := q.client.BRPop(ctx, BlockTimeout, WaitingQueue).Result()
	if err != nil {
		// redis.Nil means timeout (no job available)
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("invalid BRPOP result: %v", result)
	}

	jobID := result[1]

	return q.GetJob(ctx, jobID)
}

// StartJob marks a user as having a running job
// Returns false if user already has a running job (conflict)
func (q *Queue) StartJob(ctx context.Context, userID string, jobID string) (bool, error) {
	if q.client == nil {
		return false, fmt.Errorf("queue is not connected")
	}

	// HSETNX returns true if field was set, false if it already existed
	started, err := q.client.HSetNX(ctx, RunningUsersKey, userID, jobID).Result()
	if err != nil {
		return false, fmt.Errorf("failed to mark user as running: %w", err)
	}

	if started {
		pipe := q.client.Pipeline()
		// Update job status
		pipe.HSet(ctx, jobKey(jobID), "status", "running")
		// Add to running queue
		pipe.SAdd(ctx, RunningQueue, jobID)
		_, err := pipe.Exec(ctx)
		if err != nil {
			// If we fail here, we should probably try to undo the lock, but for now just log
			slog.Error("Failed to update job status or add to running queue", "error", err, "job_id", jobID)
		}
	}

	return started, nil
}

// CompleteJob marks a job as complete and removes user from running set
func (q *Queue) CompleteJob(ctx context.Context, userID string, jobID string) error {
	if q.client == nil {
		return fmt.Errorf("queue is not connected")
	}

	pipe := q.client.Pipeline()

	// Remove user from running hash
	pipe.HDel(ctx, RunningUsersKey, userID)

	// Remove from running queue
	if jobID != "" {
		pipe.SRem(ctx, RunningQueue, jobID)
	}

	// Update job status
	if jobID != "" {
		pipe.HSet(ctx, jobKey(jobID), "status", "completed")
		pipe.Expire(ctx, jobKey(jobID), JobRetention)
		pipe.Expire(ctx, jobItemsKey(jobID), JobRetention)
		pipe.SAdd(ctx, SuccessSet, jobID)
		// Add to cleanup queue
		pipe.ZAdd(ctx, CleanupSet, redis.Z{
			Score:  float64(time.Now().Add(JobRetention).Unix()),
			Member: fmt.Sprintf("%s:%s", userID, jobID),
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	return nil
}

// FailJob adds a job to the failed queue with a reason
func (q *Queue) FailJob(ctx context.Context, job *Job, reason string) error {
	if q.client == nil {
		return fmt.Errorf("queue is not connected")
	}

	pipe := q.client.Pipeline()

	// Update job status and reason
	pipe.HSet(ctx, jobKey(job.ID), map[string]interface{}{
		"status":      "failed",
		"fail_reason": reason,
	})

	// Push ID to failed set
	pipe.SAdd(ctx, FailedSet, job.ID)
	pipe.Expire(ctx, jobKey(job.ID), JobRetention)
	pipe.Expire(ctx, jobItemsKey(job.ID), JobRetention)

	// Add to cleanup queue
	pipe.ZAdd(ctx, CleanupSet, redis.Z{
		Score:  float64(time.Now().Add(JobRetention).Unix()),
		Member: fmt.Sprintf("%s:%s", job.UserID, job.ID),
	})

	// Remove from running queue (if it was there)
	pipe.SRem(ctx, RunningQueue, job.ID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to add job to failed queue: %w", err)
	}

	slog.Warn("Job failed", "job_id", job.ID, "user_id", job.UserID, "reason", reason)
	return nil
}

// QueueLength returns the number of jobs in the queue
func (q *Queue) QueueLength(ctx context.Context) (int64, error) {
	if q.client == nil {
		return 0, fmt.Errorf("queue is not connected")
	}

	length, err := q.client.LLen(ctx, WaitingQueue).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}

	return length, nil
}

// GetJob retrieves a job by ID
func (q *Queue) GetJob(ctx context.Context, jobID string) (*Job, error) {
	if q.client == nil {
		return nil, fmt.Errorf("queue is not connected")
	}

	var job Job
	err := q.client.HGetAll(ctx, jobKey(jobID)).Scan(&job)
	if err != nil {
		return nil, err
	}
	if job.ID == "" {
		return nil, nil // Not found
	}

	// Fetch items
	itemsMap, err := q.client.HGetAll(ctx, jobItemsKey(jobID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job items: %w", err)
	}

	for _, itemJSON := range itemsMap {
		var item JobItem
		if err := json.Unmarshal([]byte(itemJSON), &item); err != nil {
			slog.Error("Failed to unmarshal job item", "error", err)
			continue
		}
		job.Items = append(job.Items, item)
	}

	// Sort items by Title to be deterministic
	sort.Slice(job.Items, func(i, j int) bool {
		return job.Items[i].Title < job.Items[j].Title
	})

	return &job, nil
}

// GetUserJobs retrieves all jobs for a user
func (q *Queue) GetUserJobs(ctx context.Context, userID string) ([]*Job, error) {
	if q.client == nil {
		return nil, fmt.Errorf("queue is not connected")
	}

	// Get all job IDs
	jobIDs, err := q.client.SMembers(ctx, userJobsKey(userID)).Result()
	if err != nil {
		return nil, err
	}

	var jobs []*Job
	for _, id := range jobIDs {
		job, err := q.GetJob(ctx, id)
		if err != nil {
			slog.Error("Failed to fetch job", "job_id", id, "error", err)
			continue
		}
		if job != nil {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

// Close closes the queue connection
func (q *Queue) Close() error {
	if q.client != nil {
		return q.client.Close()
	}
	return nil
}

// CleanupExpiredJobs removes expired jobs from sets
func (q *Queue) CleanupExpiredJobs(ctx context.Context) error {
	if q.client == nil {
		return fmt.Errorf("queue is not connected")
	}

	// Get expired items
	now := float64(time.Now().Unix())
	items, err := q.client.ZRangeByScore(ctx, CleanupSet, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to get expired jobs: %w", err)
	}

	if len(items) == 0 {
		return nil
	}

	slog.Info("Cleaning up expired jobs", "count", len(items))

	// Process in batches of 100 to avoid blocking
	batchSize := 100
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]

		pipe := q.client.Pipeline()
		for _, item := range batch {
			// item is "userID:jobID"
			parts := strings.SplitN(item, ":", 2)
			if len(parts) != 2 {
				// Invalid format, just remove from cleanup
				pipe.ZRem(ctx, CleanupSet, item)
				continue
			}
			userID, jobID := parts[0], parts[1]

			pipe.SRem(ctx, SuccessSet, jobID)
			pipe.SRem(ctx, FailedSet, jobID)
			pipe.SRem(ctx, userJobsKey(userID), jobID)
			pipe.ZRem(ctx, CleanupSet, item)
			pipe.Del(ctx, jobKey(jobID))
			pipe.Del(ctx, jobItemsKey(jobID))
		}
		_, err := pipe.Exec(ctx)
		if err != nil {
			slog.Error("Failed to cleanup batch", "error", err)
		}
	}

	return nil
}

// SetJobItems replaces all items for a job
func (q *Queue) SetJobItems(ctx context.Context, jobID string, items []JobItem) error {
	if q.client == nil {
		return fmt.Errorf("queue is not connected")
	}

	pipe := q.client.Pipeline()
	pipe.Del(ctx, jobItemsKey(jobID)) // Clear existing items

	for _, item := range items {
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}
		pipe.HSet(ctx, jobItemsKey(jobID), item.ID, itemJSON)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// UpdateJobItem updates a single item in a job
func (q *Queue) UpdateJobItem(ctx context.Context, jobID string, item JobItem) error {
	if q.client == nil {
		return fmt.Errorf("queue is not connected")
	}

	itemJSON, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	return q.client.HSet(ctx, jobItemsKey(jobID), item.ID, itemJSON).Err()
}
