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
	runningUsers map[string]string // UserID -> JobID
	runningJobs  map[string]bool   // JobID -> bool
	waitingJobs  []*queue.Job
	failedJobs   []*queue.Job
}

// NewMockQueue creates a new mock queue
func NewMockQueue() *MockQueue {
	return &MockQueue{
		runningUsers: make(map[string]string),
		runningJobs:  make(map[string]bool),
		waitingJobs:  make([]*queue.Job, 0),
		failedJobs:   make([]*queue.Job, 0),
	}
}

// IsUserRunning checks if a user already has a job running
func (m *MockQueue) IsUserRunning(ctx context.Context, userID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.runningUsers[userID]
	return exists, nil
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
func (m *MockQueue) StartJob(ctx context.Context, userID string, jobID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.runningUsers[userID]; exists {
		return false, nil
	}

	m.runningUsers[userID] = jobID
	m.runningJobs[jobID] = true
	return true, nil
}

// CompleteJob marks a job as complete and removes user from running set
func (m *MockQueue) CompleteJob(ctx context.Context, userID string, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.runningUsers, userID)
	delete(m.runningJobs, jobID)
	return nil
}

// FailJob adds a job to the failed queue with a reason
func (m *MockQueue) FailJob(ctx context.Context, job *queue.Job, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job.FailReason = reason
	m.failedJobs = append(m.failedJobs, job)
	delete(m.runningJobs, job.ID)
	return nil
}

// QueueLength returns the number of jobs in the queue
func (m *MockQueue) QueueLength(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return int64(len(m.waitingJobs)), nil
}

// CleanupExpiredJobs removes expired jobs from sets (Mock implementation)
func (m *MockQueue) CleanupExpiredJobs(ctx context.Context) error {
	return nil
}

// Close closes the queue connection (no-op for mock)
func (m *MockQueue) Close() error {
	return nil
}

// GetJob retrieves a job by ID (Mock implementation)
func (m *MockQueue) GetJob(ctx context.Context, jobID string, includeItems bool) (*queue.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Search in waiting
	for _, job := range m.waitingJobs {
		if job.ID == jobID {
			if !includeItems {
				jobCopy := *job
				jobCopy.Items = nil
				return &jobCopy, nil
			}
			return job, nil
		}
	}
	// Search in failed
	for _, job := range m.failedJobs {
		if job.ID == jobID {
			if !includeItems {
				jobCopy := *job
				jobCopy.Items = nil
				return &jobCopy, nil
			}
			return job, nil
		}
	}
	return nil, nil
}

// GetUserJobs retrieves all jobs for a user (Mock implementation)
func (m *MockQueue) GetUserJobs(ctx context.Context, userID string) ([]*queue.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var jobs []*queue.Job
	for _, job := range m.waitingJobs {
		if job.UserID == userID {
			jobs = append(jobs, job)
		}
	}
	for _, job := range m.failedJobs {
		if job.UserID == userID {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

// Test helper methods

// SetUserRunning directly sets a user as running (for test setup)
func (m *MockQueue) SetUserRunning(userID string, running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if running {
		m.runningUsers[userID] = "mock-job-id"
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

	m.runningUsers = make(map[string]string)
	m.runningJobs = make(map[string]bool)
	m.waitingJobs = make([]*queue.Job, 0)
	m.failedJobs = make([]*queue.Job, 0)
}

// Compile-time check that MockQueue implements the same interface as Queue
var _ interface {
	IsUserRunning(ctx context.Context, userID string) (bool, error)
	Enqueue(ctx context.Context, job *queue.Job) error
	Dequeue(ctx context.Context) (*queue.Job, error)
	StartJob(ctx context.Context, userID string, jobID string) (bool, error)
	CompleteJob(ctx context.Context, userID string, jobID string) error
	FailJob(ctx context.Context, job *queue.Job, reason string) error
	CleanupExpiredJobs(ctx context.Context) error
	QueueLength(ctx context.Context) (int64, error)
	GetJob(ctx context.Context, jobID string, includeItems bool) (*queue.Job, error)
	GetUserJobs(ctx context.Context, userID string) ([]*queue.Job, error)
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
	ErrorOnCleanupExpiredJobs
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

func (m *MockQueueWithErrors) StartJob(ctx context.Context, userID string, jobID string) (bool, error) {
	if m.errorMode == ErrorOnStartJob {
		return false, fmt.Errorf("mock error: StartJob failed")
	}
	return m.MockQueue.StartJob(ctx, userID, jobID)
}

func (m *MockQueueWithErrors) CompleteJob(ctx context.Context, userID string, jobID string) error {
	if m.errorMode == ErrorOnCompleteJob {
		return fmt.Errorf("mock error: CompleteJob failed")
	}
	return m.MockQueue.CompleteJob(ctx, userID, jobID)
}

func (m *MockQueueWithErrors) FailJob(ctx context.Context, job *queue.Job, reason string) error {
	if m.errorMode == ErrorOnFailJob {
		return fmt.Errorf("mock error: FailJob failed")
	}
	return m.MockQueue.FailJob(ctx, job, reason)
}

func (m *MockQueueWithErrors) CleanupExpiredJobs(ctx context.Context) error {
	if m.errorMode == ErrorOnCleanupExpiredJobs {
		return fmt.Errorf("mock error: CleanupExpiredJobs failed")
	}
	return m.MockQueue.CleanupExpiredJobs(ctx)
}

func (m *MockQueueWithErrors) QueueLength(ctx context.Context) (int64, error) {
	if m.errorMode == ErrorOnQueueLength {
		return 0, fmt.Errorf("mock error: QueueLength failed")
	}
	return m.MockQueue.QueueLength(ctx)
}
