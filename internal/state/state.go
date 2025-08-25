package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cobblepod/internal/config"

	"github.com/redis/go-redis/v9"
)

type CobblepodState struct {
	lastRun time.Time
}
type CobblepodStateManager struct {
	client *redis.Client
}

// NewStateManager creates a new state connection using pure Go redis client
func NewStateManager(ctx context.Context) (*CobblepodStateManager, error) {
	addr := fmt.Sprintf("%s:%d", config.ValkeyHost, config.ValkeyPort)
	log.Printf("Connecting to Valkey at %s", addr)
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // Add to config if needed
		DB:       0,
	})

	// Test the connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	return &CobblepodStateManager{client: client}, nil
}

func (sm *CobblepodStateManager) GetState() (*CobblepodState, error) {
	stateStr, err := sm.client.Get(context.Background(), "state").Result()
	if err != nil {
		log.Printf("Error getting state: %v", err)
		return nil, err
	}

	var state CobblepodState
	if err := json.Unmarshal([]byte(stateStr), &state); err != nil {
		log.Printf("Error unmarshalling state: %v", err)
		return nil, err
	}
	return &state, nil
}

func (sm *CobblepodStateManager) SaveState(state *CobblepodState) error {
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
