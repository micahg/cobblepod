//go:build integration
// +build integration

package queue

import (
	"context"
	"testing"
	"time"
)

// Integration test - only runs when Redis is available
func TestQueueEnqueueDequeue(t *testing.T) {
	ctx := context.Background()

	// Try to create queue - skip if Redis not available
	q, err := NewQueue(ctx)
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
		return
	}
	defer q.Close()

	// Create test job
	job := &Job{
		ID:        "test-id-123",
		FileID:    "drive-file-123",
		UserID:    "user-123",
		Filename:  "test.backup",
		CreatedAt: time.Now(),
	}

	// Enqueue
	err = q.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Check queue length
	length, err := q.QueueLength(ctx)
	if err != nil {
		t.Fatalf("Failed to get queue length: %v", err)
	}
	if length < 1 {
		t.Errorf("Expected queue length >= 1, got %d", length)
	}

	// Dequeue
	dequeuedJob, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	// Verify job content
	if dequeuedJob == nil {
		t.Fatal("Dequeued job should not be nil")
	}
	if dequeuedJob.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, dequeuedJob.ID)
	}
	if dequeuedJob.FileID != job.FileID {
		t.Errorf("Expected file ID %s, got %s", job.FileID, dequeuedJob.FileID)
	}
}
