package storage

import (
	"context"
	"fmt"
	"log/slog"
)

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeGoogleDrive StorageType = "gdrive"
)

// StorageFactory provides methods to create storage backends
type StorageFactory struct{}

// NewFactory creates a new storage factory
func NewFactory() *StorageFactory {
	return &StorageFactory{}
}

// CreateStorage creates a new storage instance (only Google Drive supported)
func (f *StorageFactory) CreateStorage(ctx context.Context) (Storage, error) {
	return f.createGoogleDriveStorage(ctx)
}

// createGoogleDriveStorage creates a Google Drive storage backend
func (f *StorageFactory) createGoogleDriveStorage(ctx context.Context) (*GDrive, error) {
	storage, err := NewServiceWithDefaultCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Drive storage: %w", err)
	}

	slog.Info("Google Drive storage created successfully")
	return storage, nil
}

// NewStorage creates a storage backend (Google Drive)
// This is a convenience function that uses the default factory
func NewStorage(ctx context.Context) (Storage, error) {
	factory := NewFactory()
	return factory.CreateStorage(ctx)
}
