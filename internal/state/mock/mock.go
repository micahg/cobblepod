package mock

import (
	"fmt"
	"sync"

	"cobblepod/internal/state"

	"github.com/redis/go-redis/v9"
)

// MockCobblepodStateManager is a mock implementation of the state manager for
// testing. It mirrors the public surface of *state.CobblepodStateManager so it
// can be injected anywhere the real one is used.
type MockCobblepodStateManager struct {
	mu     sync.RWMutex
	states map[string]*state.CobblepodState
	// ErrorMode lets tests force errors on specific operations.
	ErrorMode ErrorMode
}

// ErrorMode selects which operation the mock returns an error from.
type ErrorMode int

const (
	NoError ErrorMode = iota
	ErrorOnGetState
	ErrorOnSaveState
)

// NewMockCobblepodStateManager creates a new mock state manager.
func NewMockCobblepodStateManager() *MockCobblepodStateManager {
	return &MockCobblepodStateManager{
		states: make(map[string]*state.CobblepodState),
	}
}

// GetState returns the stored state for the user, or a zero state if none has
// been set (mirroring the "first run" behaviour of the real manager).
func (m *MockCobblepodStateManager) GetState(userID string) (*state.CobblepodState, error) {
	if m.ErrorMode == ErrorOnGetState {
		return nil, fmt.Errorf("mock error: GetState failed")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.states[userID]
	if !ok {
		// Mirror the real manager, which returns redis.Nil when the key
		// is missing so callers can distinguish "first run" from a
		// transient error.
		return nil, redis.Nil
	}
	// Return a copy to prevent tests mutating stored state directly.
	cp := *s
	return &cp, nil
}

// SaveState stores the state for the user.
func (m *MockCobblepodStateManager) SaveState(userID string, st *state.CobblepodState) error {
	if m.ErrorMode == ErrorOnSaveState {
		return fmt.Errorf("mock error: SaveState failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store a copy to prevent tests mutating stored state directly.
	cp := *st
	m.states[userID] = &cp
	return nil
}

// Close is a no-op for the mock.
func (m *MockCobblepodStateManager) Close() error {
	return nil
}

// SetState directly sets the state for a user (test setup helper).
func (m *MockCobblepodStateManager) SetState(userID string, st *state.CobblepodState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *st
	m.states[userID] = &cp
}

// GetSavedState returns the most recently saved state for a user (test
// assertion helper). Returns nil if none has been saved.
func (m *MockCobblepodStateManager) GetSavedState(userID string) *state.CobblepodState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.states[userID]
	if !ok {
		return nil
	}
	cp := *s
	return &cp
}

// Clear resets all stored state.
func (m *MockCobblepodStateManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = make(map[string]*state.CobblepodState)
}

// Compile-time check that MockCobblepodStateManager implements the
// state.CobblepodStateManager interface.
var _ state.CobblepodStateManager = (*MockCobblepodStateManager)(nil)