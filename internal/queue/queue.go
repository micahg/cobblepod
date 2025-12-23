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

	"errors"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrUserIDRequired is returned when a user ID is required but not provided
	ErrUserIDRequired = errors.New("user ID is required")
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

// QueueConfig holds the Redis keys configuration
type QueueConfig struct {
	WaitingQueue    string
	RunningUsersKey string
	RunningQueue    string
	SuccessSet      string
	FailedSet       string
	CleanupSet      string
	KeyPrefix       string
}

// DefaultConfig returns the default queue configuration
func DefaultConfig() QueueConfig {
	return QueueConfig{
		WaitingQueue:    WaitingQueue,
		RunningUsersKey: RunningUsersKey,
		RunningQueue:    RunningQueue,
		SuccessSet:      SuccessSet,
		FailedSet:       FailedSet,
		CleanupSet:      CleanupSet,
		KeyPrefix:       "cobblepod",
	}
}

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
	config QueueConfig
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
	return &Queue{
		client: client,
		config: DefaultConfig(),
	}, nil
}

// NewQueueWithClient creates a queue with an existing Redis client (for testing)
func NewQueueWithClient(client *redis.Client) *Queue {
	return &Queue{
		client: client,
		config: DefaultConfig(),
	}
}

// NewQueueWithConfig creates a queue with custom configuration (for testing)
func NewQueueWithConfig(client *redis.Client, config QueueConfig) *Queue {
	return &Queue{
		client: client,
		config: config,
	}
}

// jobKey returns the Redis key for a job
func (q *Queue) jobKey(jobID string) string {
	return fmt.Sprintf("%s:job:%s", q.config.KeyPrefix, jobID)
}

// jobItemsKey returns the Redis key for a job's items
func (q *Queue) jobItemsKey(jobID string) string {
	return fmt.Sprintf("%s:job:%s:items", q.config.KeyPrefix, jobID)
}

// userJobsKey returns the Redis key for a user's job set
// Deprecated: Use specific status keys instead
func (q *Queue) userJobsKey(userID string) string {
	return fmt.Sprintf("%s:user:%s:jobs", q.config.KeyPrefix, userID)
}

func (q *Queue) userWaitingKey(userID string) string {
	return fmt.Sprintf("%s:user:%s:waiting", q.config.KeyPrefix, userID)
}

func (q *Queue) userRunningKey(userID string) string {
	return fmt.Sprintf("%s:user:%s:running", q.config.KeyPrefix, userID)
}

func (q *Queue) userSuccessKey(userID string) string {
	return fmt.Sprintf("%s:user:%s:success", q.config.KeyPrefix, userID)
}

func (q *Queue) userFailedKey(userID string) string {
	return fmt.Sprintf("%s:user:%s:failed", q.config.KeyPrefix, userID)
}

// IsUserRunning checks if a user already has a running job
func (q *Queue) IsUserRunning(ctx context.Context, userID string) (bool, error) {
	if q.client == nil {
		return false, fmt.Errorf("queue is not connected")
	}

	// Check if user exists in running hash
	exists, err := q.client.HExists(ctx, q.config.RunningUsersKey, userID).Result()
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
	pipe.HSet(ctx, q.jobKey(job.ID), job)

	// 2. Store items if any
	if len(job.Items) > 0 {
		for _, item := range job.Items {
			itemJSON, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal item: %w", err)
			}
			pipe.HSet(ctx, q.jobItemsKey(job.ID), item.ID, itemJSON)
		}
	}

	// 3. Add to User's Waiting Set
	if job.UserID != "" {
		pipe.SAdd(ctx, q.userWaitingKey(job.UserID), job.ID)
	}

	// 4. Push ID to Waiting Queue
	pipe.LPush(ctx, q.config.WaitingQueue, job.ID)

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
	result, err := q.client.BRPop(ctx, BlockTimeout, q.config.WaitingQueue).Result()
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
	started, err := q.client.HSetNX(ctx, q.config.RunningUsersKey, userID, jobID).Result()
	if err != nil {
		return false, fmt.Errorf("failed to mark user as running: %w", err)
	}

	if started {
		pipe := q.client.Pipeline()
		// Update job status
		pipe.HSet(ctx, q.jobKey(jobID), "status", "running")
		// Add to running queue
		pipe.SAdd(ctx, q.config.RunningQueue, jobID)
		// Move from user waiting to user running
		pipe.SMove(ctx, q.userWaitingKey(userID), q.userRunningKey(userID), jobID)
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
	pipe.HDel(ctx, q.config.RunningUsersKey, userID)

	// Remove from running queue
	if jobID != "" {
		pipe.SRem(ctx, q.config.RunningQueue, jobID)
	}

	// Update job status
	if jobID != "" {
		pipe.HSet(ctx, q.jobKey(jobID), "status", "completed")
		pipe.Expire(ctx, q.jobKey(jobID), JobRetention)
		pipe.Expire(ctx, q.jobItemsKey(jobID), JobRetention)
		pipe.SAdd(ctx, q.config.SuccessSet, jobID)
		// Move from user running to user success
		pipe.SMove(ctx, q.userRunningKey(userID), q.userSuccessKey(userID), jobID)
		// Add to cleanup queue
		pipe.ZAdd(ctx, q.config.CleanupSet, redis.Z{
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
	pipe.HSet(ctx, q.jobKey(job.ID), map[string]interface{}{
		"status":      "failed",
		"fail_reason": reason,
	})

	// Push ID to failed set
	pipe.SAdd(ctx, q.config.FailedSet, job.ID)
	pipe.Expire(ctx, q.jobKey(job.ID), JobRetention)
	pipe.Expire(ctx, q.jobItemsKey(job.ID), JobRetention)

	// Move from user running (or waiting) to user failed
	// We try removing from both and adding to failed to be safe
	pipe.SRem(ctx, q.userRunningKey(job.UserID), job.ID)
	pipe.SRem(ctx, q.userWaitingKey(job.UserID), job.ID)
	pipe.SAdd(ctx, q.userFailedKey(job.UserID), job.ID)

	// Add to cleanup queue
	pipe.ZAdd(ctx, q.config.CleanupSet, redis.Z{
		Score:  float64(time.Now().Add(JobRetention).Unix()),
		Member: fmt.Sprintf("%s:%s", job.UserID, job.ID),
	})

	// Remove from running queue (if it was there)
	pipe.SRem(ctx, q.config.RunningQueue, job.ID)

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

	length, err := q.client.LLen(ctx, q.config.WaitingQueue).Result()
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
	err := q.client.HGetAll(ctx, q.jobKey(jobID)).Scan(&job)
	if err != nil {
		return nil, err
	}
	if job.ID == "" {
		return nil, nil // Not found
	}

	// Fetch items
	itemsMap, err := q.client.HGetAll(ctx, q.jobItemsKey(jobID)).Result()
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

	// Get all job IDs from all user sets
	jobIDs, err := q.client.SUnion(ctx,
		q.userWaitingKey(userID),
		q.userRunningKey(userID),
		q.userSuccessKey(userID),
		q.userFailedKey(userID),
	).Result()
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
	items, err := q.client.ZRangeByScore(ctx, q.config.CleanupSet, &redis.ZRangeBy{
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
				pipe.ZRem(ctx, q.config.CleanupSet, item)
				continue
			}
			userID, jobID := parts[0], parts[1]

			pipe.SRem(ctx, q.config.SuccessSet, jobID)
			pipe.SRem(ctx, q.config.FailedSet, jobID)
			// Remove from all possible user sets
			pipe.SRem(ctx, q.userWaitingKey(userID), jobID)
			pipe.SRem(ctx, q.userRunningKey(userID), jobID)
			pipe.SRem(ctx, q.userSuccessKey(userID), jobID)
			pipe.SRem(ctx, q.userFailedKey(userID), jobID)
			pipe.ZRem(ctx, q.config.CleanupSet, item)
			pipe.Del(ctx, q.jobKey(jobID))
			pipe.Del(ctx, q.jobItemsKey(jobID))
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
	pipe.Del(ctx, q.jobItemsKey(jobID)) // Clear existing items

	for _, item := range items {
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}
		pipe.HSet(ctx, q.jobItemsKey(jobID), item.ID, itemJSON)
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

	return q.client.HSet(ctx, q.jobItemsKey(jobID), item.ID, itemJSON).Err()
}

// getJobsFromIDs retrieves multiple jobs by their IDs
func (q *Queue) getJobsFromIDs(ctx context.Context, jobIDs []string) ([]*Job, error) {
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

// GetWaitingJobs returns all jobs currently in the waiting queue
func (q *Queue) GetWaitingJobs(ctx context.Context, userID string) ([]*Job, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}
	if q.client == nil {
		return nil, fmt.Errorf("queue is not connected")
	}

	jobIDs, err := q.client.SMembers(ctx, q.userWaitingKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get waiting jobs: %w", err)
	}

	jobs, err := q.getJobsFromIDs(ctx, jobIDs)
	if err != nil {
		return nil, err
	}

	// Since Sets are unordered, sort by CreatedAt to approximate queue order
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})

	return jobs, nil
}

// GetRunningJobs returns all jobs currently in the running set
func (q *Queue) GetRunningJobs(ctx context.Context, userID string) ([]*Job, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}
	if q.client == nil {
		return nil, fmt.Errorf("queue is not connected")
	}

	jobIDs, err := q.client.SMembers(ctx, q.userRunningKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get running jobs: %w", err)
	}

	return q.getJobsFromIDs(ctx, jobIDs)
}

// GetCompletedJobs returns all jobs in the success set
func (q *Queue) GetCompletedJobs(ctx context.Context, userID string) ([]*Job, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}
	if q.client == nil {
		return nil, fmt.Errorf("queue is not connected")
	}

	jobIDs, err := q.client.SMembers(ctx, q.userSuccessKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get completed jobs: %w", err)
	}

	return q.getJobsFromIDs(ctx, jobIDs)
}

// GetFailedJobs returns all jobs in the failed set
func (q *Queue) GetFailedJobs(ctx context.Context, userID string) ([]*Job, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}
	if q.client == nil {
		return nil, fmt.Errorf("queue is not connected")
	}

	jobIDs, err := q.client.SMembers(ctx, q.userFailedKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get failed jobs: %w", err)
	}

	return q.getJobsFromIDs(ctx, jobIDs)
}
