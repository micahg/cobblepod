package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cobblepod/internal/config"

	"github.com/redis/go-redis/v9"
)

const (
	// QueueName is the Redis list key for job queue
	QueueName = "cobblepod:jobs"
	// BlockTimeout is how long BRPOP will wait for a job
	BlockTimeout = 5 * time.Second
)

// Job represents a backup processing job
type Job struct {
	ID        string    `json:"id"`
	FileID    string    `json:"file_id"`
	UserID    string    `json:"user_id,omitempty"`
	Filename  string    `json:"filename,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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

// Enqueue adds a job to the queue
func (q *Queue) Enqueue(ctx context.Context, job *Job) error {
	if q.client == nil {
		return fmt.Errorf("queue is not connected")
	}

	// Marshal job to JSON
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Push to left of list (LPUSH = append to queue)
	err = q.client.LPush(ctx, QueueName, jobJSON).Err()
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
	result, err := q.client.BRPop(ctx, BlockTimeout, QueueName).Result()
	if err != nil {
		// redis.Nil means timeout (no job available)
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	// BRPOP returns [key, value]
	if len(result) < 2 {
		return nil, fmt.Errorf("invalid BRPOP result: %v", result)
	}

	// Unmarshal the job
	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	slog.Info("Job dequeued", "job_id", job.ID, "file_id", job.FileID)
	return &job, nil
}

// QueueLength returns the number of jobs in the queue
func (q *Queue) QueueLength(ctx context.Context) (int64, error) {
	if q.client == nil {
		return 0, fmt.Errorf("queue is not connected")
	}

	length, err := q.client.LLen(ctx, QueueName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}

	return length, nil
}

// Close closes the queue connection
func (q *Queue) Close() error {
	if q.client != nil {
		return q.client.Close()
	}
	return nil
}
