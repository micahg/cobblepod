package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cobblepod/internal/config"

	"github.com/redis/go-redis/v9"
)

type CobblepodState struct {
	LastRun    time.Time
	PlayrunJWT string
}

// CobblepodStateManager is the interface for persisting per-user application
// state. The production implementation is backed by Valkey/Redis; tests can
// inject a mock (see internal/state/mock).
type CobblepodStateManager interface {
	GetState(userID string) (*CobblepodState, error)
	SaveState(userID string, state *CobblepodState) error
	Close() error
}

// valkeyStateManager is the production CobblepodStateManager, backed by Valkey.
type valkeyStateManager struct {
	client *redis.Client
}

// NewStateManager creates a new state connection using pure Go redis client.
// On connection failure it returns (nil, err); callers should treat a nil
// manager as "state unavailable" and degrade gracefully.
func NewStateManager(ctx context.Context) (CobblepodStateManager, error) {
	addr := fmt.Sprintf("%s:%d", config.ValkeyHost, config.ValkeyPort)
	slog.Debug("Connecting to Valkey", "addr", addr)
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // Add to config if needed
		DB:       0,
	})

	// Test the connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		// Close the unused client to avoid leaking the connection.
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	return &valkeyStateManager{client: client}, nil
}

func (sm *valkeyStateManager) stateKey(userID string) string {
	return fmt.Sprintf("cobblepod:state:%s", userID)
}

func (sm *valkeyStateManager) GetState(userID string) (*CobblepodState, error) {
	if sm.client == nil {
		return nil, fmt.Errorf("state manager is not connected")
	}

	stateStr, err := sm.client.Get(context.Background(), sm.stateKey(userID)).Result()
	if err != nil {
		slog.Error("Error getting state", "error", err, "user_id", userID)
		return &CobblepodState{LastRun: time.Unix(0, 0)}, err
	}

	var state CobblepodState
	if err := json.Unmarshal([]byte(stateStr), &state); err != nil {
		slog.Error("Error unmarshalling state", "error", err)
		return nil, err
	}
	return &state, nil
}

func (sm *valkeyStateManager) SaveState(userID string, state *CobblepodState) error {
	if sm.client == nil {
		return fmt.Errorf("state manager is not connected")
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	err = sm.client.Set(context.Background(), sm.stateKey(userID), stateJSON, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

// Close closes the state manager connection
func (sm *valkeyStateManager) Close() error {
	if sm.client != nil {
		return sm.client.Close()
	}
	return nil
}