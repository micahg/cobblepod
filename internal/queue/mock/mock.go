package mock

import (
	"context"
	"fmt"
	"sync"

	"cobblepod/internal/queue"
)

// MockQueue is a mock implementation of the Queue for testing
type MockQueue struct {
	mu           sync.RWMutex
	runningUsers map[string]bool
	waitingJobs  []*queue.Job
	failedJobs   []*queue.Job
}

// NewMockQueue creates a new mock queue
func NewMockQueue() *MockQueue {
	return &MockQueue{
		runningUsers: make(map[string]bool),
		waitingJobs:  make([]*queue.Job, 0),
		failedJobs:   make([]*queue.Job, 0),
	}
}

// IsUserRunning checks if a user already has a job running
func (m *MockQueue) IsUserRunning(ctx context.Context, userID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.runningUsers[userID], nil
}

// Enqueue adds a job to the queue
func (m *MockQueue) Enqueue(ctx context.Context, job *queue.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.waitingJobs = append(m.waitingJobs, job)
	return nil
}

// Dequeue removes and returns a job from the queue
func (m *MockQueue) Dequeue(ctx context.Context) (*queue.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.waitingJobs) == 0 {
		return nil, nil
	}

	// Pop from the front (FIFO) - Redis uses LPUSH/BRPOP which gives FIFO behavior
	job := m.waitingJobs[0]
	m.waitingJobs = m.waitingJobs[1:]
	return job, nil
}

// StartJob marks a user as having a running job
func (m *MockQueue) StartJob(ctx context.Context, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.runningUsers[userID] {
		return false, nil
	}

	m.runningUsers[userID] = true
	return true, nil
}

// CompleteJob marks a job as complete and removes user from running set
func (m *MockQueue) CompleteJob(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.runningUsers, userID)
	return nil
}

// FailJob adds a job to the failed queue with a reason
func (m *MockQueue) FailJob(ctx context.Context, job *queue.Job, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job.FailReason = reason
	m.failedJobs = append(m.failedJobs, job)
	return nil
}

// QueueLength returns the number of jobs in the queue
func (m *MockQueue) QueueLength(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return int64(len(m.waitingJobs)), nil
}

// Close closes the queue connection (no-op for mock)
func (m *MockQueue) Close() error {
	return nil
}

// Test helper methods

// SetUserRunning directly sets a user as running (for test setup)
func (m *MockQueue) SetUserRunning(userID string, running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if running {
		m.runningUsers[userID] = true
	} else {
		delete(m.runningUsers, userID)
	}
}

// GetFailedJobs returns all failed jobs (for test assertions)
func (m *MockQueue) GetFailedJobs() []*queue.Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*queue.Job, len(m.failedJobs))
	copy(jobs, m.failedJobs)
	return jobs
}

// GetWaitingJobs returns all waiting jobs (for test assertions)
func (m *MockQueue) GetWaitingJobs() []*queue.Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*queue.Job, len(m.waitingJobs))
	copy(jobs, m.waitingJobs)
	return jobs
}

// Clear resets the mock queue state
func (m *MockQueue) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.runningUsers = make(map[string]bool)
	m.waitingJobs = make([]*queue.Job, 0)
	m.failedJobs = make([]*queue.Job, 0)
}

// Compile-time check that MockQueue implements the same interface as Queue
// This will fail if Queue adds new methods that MockQueue doesn't implement
var _ interface {
	IsUserRunning(ctx context.Context, userID string) (bool, error)
	Enqueue(ctx context.Context, job *queue.Job) error
	Dequeue(ctx context.Context) (*queue.Job, error)
	StartJob(ctx context.Context, userID string) (bool, error)
	CompleteJob(ctx context.Context, userID string) error
	FailJob(ctx context.Context, job *queue.Job, reason string) error
	QueueLength(ctx context.Context) (int64, error)
	Close() error
} = (*MockQueue)(nil)

// ForceError can be set to make methods return errors (for error testing)
type ErrorMode int

const (
	NoError ErrorMode = iota
	ErrorOnIsUserRunning
	ErrorOnEnqueue
	ErrorOnDequeue
	ErrorOnStartJob
	ErrorOnCompleteJob
	ErrorOnFailJob
	ErrorOnQueueLength
)

// MockQueueWithErrors is a mock queue that can simulate errors
type MockQueueWithErrors struct {
	*MockQueue
	errorMode ErrorMode
}

// NewMockQueueWithErrors creates a mock queue that can return errors
func NewMockQueueWithErrors(errorMode ErrorMode) *MockQueueWithErrors {
	return &MockQueueWithErrors{
		MockQueue: NewMockQueue(),
		errorMode: errorMode,
	}
}

func (m *MockQueueWithErrors) IsUserRunning(ctx context.Context, userID string) (bool, error) {
	if m.errorMode == ErrorOnIsUserRunning {
		return false, fmt.Errorf("mock error: IsUserRunning failed")
	}
	return m.MockQueue.IsUserRunning(ctx, userID)
}

func (m *MockQueueWithErrors) Enqueue(ctx context.Context, job *queue.Job) error {
	if m.errorMode == ErrorOnEnqueue {
		return fmt.Errorf("mock error: Enqueue failed")
	}
	return m.MockQueue.Enqueue(ctx, job)
}

func (m *MockQueueWithErrors) Dequeue(ctx context.Context) (*queue.Job, error) {
	if m.errorMode == ErrorOnDequeue {
		return nil, fmt.Errorf("mock error: Dequeue failed")
	}
	return m.MockQueue.Dequeue(ctx)
}

func (m *MockQueueWithErrors) StartJob(ctx context.Context, userID string) (bool, error) {
	if m.errorMode == ErrorOnStartJob {
		return false, fmt.Errorf("mock error: StartJob failed")
	}
	return m.MockQueue.StartJob(ctx, userID)
}

func (m *MockQueueWithErrors) CompleteJob(ctx context.Context, userID string) error {
	if m.errorMode == ErrorOnCompleteJob {
		return fmt.Errorf("mock error: CompleteJob failed")
	}
	return m.MockQueue.CompleteJob(ctx, userID)
}

func (m *MockQueueWithErrors) FailJob(ctx context.Context, job *queue.Job, reason string) error {
	if m.errorMode == ErrorOnFailJob {
		return fmt.Errorf("mock error: FailJob failed")
	}
	return m.MockQueue.FailJob(ctx, job, reason)
}

func (m *MockQueueWithErrors) QueueLength(ctx context.Context) (int64, error) {
	if m.errorMode == ErrorOnQueueLength {
		return 0, fmt.Errorf("mock error: QueueLength failed")
	}
	return m.MockQueue.QueueLength(ctx)
}
