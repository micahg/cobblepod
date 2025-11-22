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
	LastRun time.Time
}

type CobblepodStateManager struct {
	client *redis.Client
}

// NewStateManager creates a new state connection using pure Go redis client
func NewStateManager(ctx context.Context) (*CobblepodStateManager, error) {
	addr := fmt.Sprintf("%s:%d", config.ValkeyHost, config.ValkeyPort)
	slog.Debug("Connecting to Valkey", "addr", addr)
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // Add to config if needed
		DB:       0,
	})

	sm := &CobblepodStateManager{client: client}

	// Test the connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		sm.client = nil
		return sm, fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	return sm, nil
}

func (sm *CobblepodStateManager) GetState() (*CobblepodState, error) {
	if sm.client == nil {
		return nil, fmt.Errorf("state manager is not connected")
	}

	stateStr, err := sm.client.Get(context.Background(), "state").Result()
	if err != nil {
		slog.Error("Error getting state", "error", err)
		return &CobblepodState{LastRun: time.Unix(0, 0)}, err
	}

	var state CobblepodState
	if err := json.Unmarshal([]byte(stateStr), &state); err != nil {
		slog.Error("Error unmarshalling state", "error", err)
		return nil, err
	}
	return &state, nil
}

func (sm *CobblepodStateManager) SaveState(state *CobblepodState) error {
	if sm.client == nil {
		return fmt.Errorf("state manager is not connected")
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	err = sm.client.Set(context.Background(), "state", stateJSON, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}
