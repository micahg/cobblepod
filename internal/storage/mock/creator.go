package mock

import (
	"context"

	"cobblepod/internal/storage"
)

// NewMockStorageCreator returns a function that matches the StorageCreator signature
// but returns the provided storage and error.
func NewMockStorageCreator(s storage.Storage, err error) func(context.Context, string) (storage.Storage, error) {
	return func(ctx context.Context, accessToken string) (storage.Storage, error) {
		return s, err
	}
}
