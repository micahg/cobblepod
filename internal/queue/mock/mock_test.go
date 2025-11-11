package mock

import (
	"context"
	"strings"
	"testing"
	"time"

	"cobblepod/internal/queue"
)

func TestMockQueue_IsUserRunning(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueue()

	// Initially, no users should be running
	isRunning, err := mockQueue.IsUserRunning(ctx, "user1")
	if err != nil {
		t.Fatalf("IsUserRunning() unexpected error: %v", err)
	}
	if isRunning {
		t.Error("Expected user1 not to be running initially, but it was")
	}

	// Set user as running
	mockQueue.SetUserRunning("user1", true)

	// Now user should be running
	isRunning, err = mockQueue.IsUserRunning(ctx, "user1")
	if err != nil {
		t.Fatalf("IsUserRunning() unexpected error: %v", err)
	}
	if !isRunning {
		t.Error("Expected user1 to be running, but it wasn't")
	}

	// Remove user from running
	mockQueue.SetUserRunning("user1", false)

	// User should not be running anymore
	isRunning, err = mockQueue.IsUserRunning(ctx, "user1")
	if err != nil {
		t.Fatalf("IsUserRunning() unexpected error: %v", err)
	}
	if isRunning {
		t.Error("Expected user1 not to be running after removal, but it was")
	}
}

func TestMockQueue_StartJob(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueue()

	// First call should succeed
	added, err := mockQueue.StartJob(ctx, "user1")
	if err != nil {
		t.Fatalf("StartJob() unexpected error: %v", err)
	}
	if !added {
		t.Error("First StartJob should return true")
	}

	// Second call should fail (user already running)
	added, err = mockQueue.StartJob(ctx, "user1")
	if err != nil {
		t.Fatalf("StartJob() unexpected error: %v", err)
	}
	if added {
		t.Error("Second StartJob should return false")
	}

	// Verify user is running
	isRunning, err := mockQueue.IsUserRunning(ctx, "user1")
	if err != nil {
		t.Fatalf("IsUserRunning() unexpected error: %v", err)
	}
	if !isRunning {
		t.Error("Expected user1 to be running")
	}
}

func TestMockQueue_CompleteJob(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueue()

	// Start a job
	added, err := mockQueue.StartJob(ctx, "user1")
	if err != nil {
		t.Fatalf("StartJob() unexpected error: %v", err)
	}
	if !added {
		t.Error("StartJob should return true")
	}

	// Complete the job
	err = mockQueue.CompleteJob(ctx, "user1")
	if err != nil {
		t.Fatalf("CompleteJob() unexpected error: %v", err)
	}

	// User should no longer be running
	isRunning, err := mockQueue.IsUserRunning(ctx, "user1")
	if err != nil {
		t.Fatalf("IsUserRunning() unexpected error: %v", err)
	}
	if isRunning {
		t.Error("Expected user1 not to be running after job completion")
	}
}

func TestMockQueue_EnqueueDequeue(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueue()

	// Queue should be empty initially
	length, err := mockQueue.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength() unexpected error: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected queue length 0, got %d", length)
	}

	// Enqueue a job
	job1 := &queue.Job{
		ID:        "job1",
		FileID:    "file1",
		UserID:    "user1",
		CreatedAt: time.Now(),
	}
	err = mockQueue.Enqueue(ctx, job1)
	if err != nil {
		t.Fatalf("Enqueue() unexpected error: %v", err)
	}

	// Queue length should be 1
	length, err = mockQueue.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength() unexpected error: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected queue length 1, got %d", length)
	}

	// Enqueue another job
	job2 := &queue.Job{
		ID:        "job2",
		FileID:    "file2",
		UserID:    "user2",
		CreatedAt: time.Now(),
	}
	err = mockQueue.Enqueue(ctx, job2)
	if err != nil {
		t.Fatalf("Enqueue() unexpected error: %v", err)
	}

	// Queue length should be 2
	length, err = mockQueue.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength() unexpected error: %v", err)
	}
	if length != 2 {
		t.Errorf("Expected queue length 2, got %d", length)
	}

	// Dequeue should return first job (FIFO)
	dequeuedJob, err := mockQueue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue() unexpected error: %v", err)
	}
	if dequeuedJob == nil {
		t.Fatal("Dequeue() returned nil job")
	}
	if dequeuedJob.ID != "job1" {
		t.Errorf("Expected job ID 'job1', got '%s'", dequeuedJob.ID)
	}

	// Queue length should be 1
	length, err = mockQueue.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength() unexpected error: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected queue length 1, got %d", length)
	}

	// Dequeue second job
	dequeuedJob, err = mockQueue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue() unexpected error: %v", err)
	}
	if dequeuedJob == nil {
		t.Fatal("Dequeue() returned nil job")
	}
	if dequeuedJob.ID != "job2" {
		t.Errorf("Expected job ID 'job2', got '%s'", dequeuedJob.ID)
	}

	// Queue should be empty
	length, err = mockQueue.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength() unexpected error: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected queue length 0, got %d", length)
	}

	// Dequeue from empty queue should return nil
	dequeuedJob, err = mockQueue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue() unexpected error: %v", err)
	}
	if dequeuedJob != nil {
		t.Error("Expected nil job from empty queue")
	}
}

func TestMockQueue_FailJob(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueue()

	job := &queue.Job{
		ID:        "job1",
		FileID:    "file1",
		UserID:    "user1",
		CreatedAt: time.Now(),
	}

	// Fail the job
	err := mockQueue.FailJob(ctx, job, "test error")
	if err != nil {
		t.Fatalf("FailJob() unexpected error: %v", err)
	}

	// Check failed jobs
	failedJobs := mockQueue.GetFailedJobs()
	if len(failedJobs) != 1 {
		t.Fatalf("Expected 1 failed job, got %d", len(failedJobs))
	}
	if failedJobs[0].ID != "job1" {
		t.Errorf("Expected job ID 'job1', got '%s'", failedJobs[0].ID)
	}
	if failedJobs[0].FailReason != "test error" {
		t.Errorf("Expected fail reason 'test error', got '%s'", failedJobs[0].FailReason)
	}
}

func TestMockQueue_Clear(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueue()

	// Add some state
	mockQueue.SetUserRunning("user1", true)
	job := &queue.Job{
		ID:        "job1",
		FileID:    "file1",
		UserID:    "user1",
		CreatedAt: time.Now(),
	}
	err := mockQueue.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Enqueue() unexpected error: %v", err)
	}
	err = mockQueue.FailJob(ctx, job, "test error")
	if err != nil {
		t.Fatalf("FailJob() unexpected error: %v", err)
	}

	// Verify state exists
	isRunning, err := mockQueue.IsUserRunning(ctx, "user1")
	if err != nil {
		t.Fatalf("IsUserRunning() unexpected error: %v", err)
	}
	if !isRunning {
		t.Error("Expected user1 to be running")
	}

	length, err := mockQueue.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength() unexpected error: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected queue length 1, got %d", length)
	}

	failedJobs := mockQueue.GetFailedJobs()
	if len(failedJobs) != 1 {
		t.Errorf("Expected 1 failed job, got %d", len(failedJobs))
	}

	// Clear the queue
	mockQueue.Clear()

	// Verify everything is cleared
	isRunning, err = mockQueue.IsUserRunning(ctx, "user1")
	if err != nil {
		t.Fatalf("IsUserRunning() unexpected error: %v", err)
	}
	if isRunning {
		t.Error("Expected user1 not to be running after clear")
	}

	length, err = mockQueue.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength() unexpected error: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected queue length 0 after clear, got %d", length)
	}

	failedJobs = mockQueue.GetFailedJobs()
	if len(failedJobs) != 0 {
		t.Errorf("Expected 0 failed jobs after clear, got %d", len(failedJobs))
	}
}

func TestMockQueueWithErrors_IsUserRunning(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueueWithErrors(ErrorOnIsUserRunning)

	_, err := mockQueue.IsUserRunning(ctx, "user1")
	if err == nil {
		t.Error("Expected error from IsUserRunning, got nil")
	}
	if !strings.Contains(err.Error(), "IsUserRunning failed") {
		t.Errorf("Expected error message to contain 'IsUserRunning failed', got '%s'", err.Error())
	}
}

func TestMockQueueWithErrors_Enqueue(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueueWithErrors(ErrorOnEnqueue)

	job := &queue.Job{ID: "job1", FileID: "file1"}
	err := mockQueue.Enqueue(ctx, job)
	if err == nil {
		t.Error("Expected error from Enqueue, got nil")
	}
	if !strings.Contains(err.Error(), "Enqueue failed") {
		t.Errorf("Expected error message to contain 'Enqueue failed', got '%s'", err.Error())
	}
}

func TestMockQueueWithErrors_Dequeue(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueueWithErrors(ErrorOnDequeue)

	_, err := mockQueue.Dequeue(ctx)
	if err == nil {
		t.Error("Expected error from Dequeue, got nil")
	}
	if !strings.Contains(err.Error(), "Dequeue failed") {
		t.Errorf("Expected error message to contain 'Dequeue failed', got '%s'", err.Error())
	}
}

func TestMockQueueWithErrors_StartJob(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueueWithErrors(ErrorOnStartJob)

	_, err := mockQueue.StartJob(ctx, "user1")
	if err == nil {
		t.Error("Expected error from StartJob, got nil")
	}
	if !strings.Contains(err.Error(), "StartJob failed") {
		t.Errorf("Expected error message to contain 'StartJob failed', got '%s'", err.Error())
	}
}

func TestMockQueueWithErrors_CompleteJob(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueueWithErrors(ErrorOnCompleteJob)

	err := mockQueue.CompleteJob(ctx, "user1")
	if err == nil {
		t.Error("Expected error from CompleteJob, got nil")
	}
	if !strings.Contains(err.Error(), "CompleteJob failed") {
		t.Errorf("Expected error message to contain 'CompleteJob failed', got '%s'", err.Error())
	}
}

func TestMockQueueWithErrors_FailJob(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueueWithErrors(ErrorOnFailJob)

	job := &queue.Job{ID: "job1", FileID: "file1"}
	err := mockQueue.FailJob(ctx, job, "reason")
	if err == nil {
		t.Error("Expected error from FailJob, got nil")
	}
	if !strings.Contains(err.Error(), "FailJob failed") {
		t.Errorf("Expected error message to contain 'FailJob failed', got '%s'", err.Error())
	}
}

func TestMockQueueWithErrors_QueueLength(t *testing.T) {
	ctx := context.Background()
	mockQueue := NewMockQueueWithErrors(ErrorOnQueueLength)

	_, err := mockQueue.QueueLength(ctx)
	if err == nil {
		t.Error("Expected error from QueueLength, got nil")
	}
	if !strings.Contains(err.Error(), "QueueLength failed") {
		t.Errorf("Expected error message to contain 'QueueLength failed', got '%s'", err.Error())
	}
}
