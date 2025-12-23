//go:build integration
// +build integration

package queue

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func setupTestQueue(t *testing.T) *Queue {
	ctx := context.Background()

	// Create a temporary client to check connection
	tempQ, err := NewQueue(ctx)
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
		return nil
	}
	client := tempQ.client

	// Use unique keys for testing to avoid interference from background workers
	suffix := time.Now().UnixNano()
	config := DefaultConfig()
	config.KeyPrefix = fmt.Sprintf("test:%d", suffix)
	config.WaitingQueue = fmt.Sprintf("%s:waiting", config.KeyPrefix)
	config.RunningUsersKey = fmt.Sprintf("%s:running-users", config.KeyPrefix)
	config.RunningQueue = fmt.Sprintf("%s:running", config.KeyPrefix)
	config.SuccessSet = fmt.Sprintf("%s:success", config.KeyPrefix)
	config.FailedSet = fmt.Sprintf("%s:failed", config.KeyPrefix)
	config.CleanupSet = fmt.Sprintf("%s:cleanup", config.KeyPrefix)

	return NewQueueWithConfig(client, config)
}

// Integration test - only runs when Redis is available
func TestQueueEnqueueDequeue(t *testing.T) {
	ctx := context.Background()

	// Try to create queue - skip if Redis not available
	q := setupTestQueue(t)
	if q == nil {
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
	err := q.Enqueue(ctx, job)
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

func TestQueueLifecycle(t *testing.T) {
	ctx := context.Background()

	// Try to create queue - skip if Redis not available
	q := setupTestQueue(t)
	if q == nil {
		return
	}
	defer q.Close()

	// Create test job
	jobID := "lifecycle-test-job"
	userID := "lifecycle-test-user"
	job := &Job{
		ID:        jobID,
		FileID:    "file-123",
		UserID:    userID,
		Filename:  "test.backup",
		CreatedAt: time.Now(),
	}

	// 1. Enqueue (Waiting)
	err := q.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Verify waiting
	waiting, err := q.GetWaitingJobs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get waiting jobs: %v", err)
	}
	if len(waiting) != 1 || waiting[0].ID != jobID {
		t.Errorf("Expected job in waiting queue, got %v", waiting)
	}

	// 2. Start Job (Running)
	started, err := q.StartJob(ctx, userID, jobID)
	if err != nil {
		t.Fatalf("Failed to start job: %v", err)
	}
	if !started {
		t.Fatal("Expected StartJob to return true")
	}

	// Verify running
	running, err := q.GetRunningJobs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get running jobs: %v", err)
	}
	if len(running) != 1 || running[0].ID != jobID {
		t.Errorf("Expected job in running queue, got %v", running)
	}

	// Verify NOT waiting anymore
	waiting, err = q.GetWaitingJobs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get waiting jobs: %v", err)
	}
	if len(waiting) != 0 {
		t.Errorf("Expected waiting queue to be empty, got %v", waiting)
	}

	// 3. Complete Job (Success)
	err = q.CompleteJob(ctx, userID, jobID)
	if err != nil {
		t.Fatalf("Failed to complete job: %v", err)
	}

	// Verify completed
	completed, err := q.GetCompletedJobs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get completed jobs: %v", err)
	}
	if len(completed) != 1 || completed[0].ID != jobID {
		t.Errorf("Expected job in completed queue, got %v", completed)
	}

	// Verify NOT running anymore
	running, err = q.GetRunningJobs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get running jobs: %v", err)
	}
	if len(running) != 0 {
		t.Errorf("Expected running queue to be empty, got %v", running)
	}

	// 4. Test Failure Path (New Job)
	failJobID := "fail-test-job"
	failJob := &Job{
		ID:        failJobID,
		FileID:    "file-456",
		UserID:    userID,
		CreatedAt: time.Now(),
	}

	err = q.Enqueue(ctx, failJob)
	if err != nil {
		t.Fatalf("Failed to enqueue fail job: %v", err)
	}

	// Start it
	_, err = q.StartJob(ctx, userID, failJobID)
	if err != nil {
		t.Fatalf("Failed to start fail job: %v", err)
	}

	// Fail it
	err = q.FailJob(ctx, failJob, "something went wrong")
	if err != nil {
		t.Fatalf("Failed to fail job: %v", err)
	}

	// Verify failed
	failed, err := q.GetFailedJobs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get failed jobs: %v", err)
	}
	if len(failed) != 1 || failed[0].ID != failJobID {
		t.Errorf("Expected job in failed queue, got %v", failed)
	}
	if failed[0].FailReason != "something went wrong" {
		t.Errorf("Expected fail reason 'something went wrong', got '%s'", failed[0].FailReason)
	}

	// Verify NOT running
	running, err = q.GetRunningJobs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get running jobs: %v", err)
	}
	if len(running) != 0 {
		t.Errorf("Expected running queue to be empty, got %v", running)
	}
}
