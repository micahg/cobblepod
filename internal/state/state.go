package state

import (
	"context"
	"log"
	"fmt"

	"cobblepod/internal/config"

	"github.com/redis/go-redis/v9"
)

type CobblepodState struct {
	value string
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

func (state *CobblepodStateManager) GetState() *CobblepodState {
	return &CobblepodState{
		value: state.client.Get(context.Background(), "state").Val(),
	}
}
