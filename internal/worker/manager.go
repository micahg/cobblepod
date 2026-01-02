package worker

import (
	"context"
	"sync"
)

// Manager handles active job contexts for cancellation
type Manager struct {
	mu   sync.RWMutex
	jobs map[string]context.CancelFunc
}

// NewManager creates a new job manager
func NewManager() *Manager {
	return &Manager{
		jobs: make(map[string]context.CancelFunc),
	}
}

// Add registers a job with its cancel function
func (m *Manager) Add(jobID string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[jobID] = cancel
}

// Remove unregisters a job
func (m *Manager) Remove(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, jobID)
}

// Cancel cancels a specific job if it's running
func (m *Manager) Cancel(jobID string) {
	m.mu.RLock()
	cancel, ok := m.jobs[jobID]
	m.mu.RUnlock()

	if ok {
		cancel()
	}
}
