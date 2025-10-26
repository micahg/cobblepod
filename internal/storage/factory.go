package storage

import (
	"context"
	"fmt"
	"log/slog"

	"cobblepod/internal/config"
)

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeGoogleDrive StorageType = "gdrive"
	StorageTypeS3          StorageType = "s3"
	StorageTypeR2          StorageType = "r2"
)

// StorageFactory provides methods to create storage backends
type StorageFactory struct{}

// NewFactory creates a new storage factory
func NewFactory() *StorageFactory {
	return &StorageFactory{}
}

// CreateStorageFromConfig creates a storage instance based on the current config
// This is a convenience function for the most common use case
func CreateStorageFromConfig() (Storage, error) {
	storageType, err := GetStorageTypeFromString(config.StorageBackend)
	if err != nil {
		return nil, err
	}

	// Validate configuration first
	if err := ValidateStorageConfig(storageType); err != nil {
		return nil, fmt.Errorf("storage configuration validation failed: %w", err)
	}

	factory := NewFactory()
	return factory.CreateStorage(context.Background(), storageType)
}

// CreateStorage creates a new storage instance based on the given type
// This is a convenience function that uses the default factory
func CreateStorage(storageType StorageType) (Storage, error) {
	factory := NewFactory()
	return factory.CreateStorage(context.Background(), storageType)
}
func (f *StorageFactory) CreateStorage(ctx context.Context, storageType StorageType) (Storage, error) {
	switch storageType {
	case StorageTypeS3, StorageTypeR2:
		return f.createS3Storage(ctx)
	case StorageTypeGoogleDrive:
		return f.createGoogleDriveStorage(ctx)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", storageType)
	}
}

// CreateStorageFromConfig creates storage backend from environment configuration
func (f *StorageFactory) CreateStorageFromConfig(ctx context.Context) (Storage, error) {
	storageType := StorageType(config.StorageBackend)

	// Default to Google Drive for backwards compatibility
	if storageType == "" {
		storageType = StorageTypeGoogleDrive
		slog.Info("No storage backend specified, defaulting to Google Drive")
	}

	slog.Info("Creating storage backend", "type", storageType)
	return f.CreateStorage(ctx, storageType)
}

// createS3Storage creates an S3-compatible storage backend
func (f *StorageFactory) createS3Storage(ctx context.Context) (*S3Storage, error) {
	cfg := S3Config{
		Region:      config.S3Region,
		Bucket:      config.S3Bucket,
		AccessKey:   config.S3AccessKey,
		SecretKey:   config.S3SecretKey,
		EndpointURL: config.S3EndpointURL,
		BaseURL:     config.S3BaseURL,
		PublicRead:  config.S3PublicRead,
	}

	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET environment variable is required for S3/R2 storage")
	}

	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables are required for S3/R2 storage")
	}

	storage, err := NewS3Storage(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 storage: %w", err)
	}

	slog.Info("S3/R2 storage created successfully",
		"bucket", cfg.Bucket,
		"endpoint", cfg.EndpointURL,
		"public_read", cfg.PublicRead)

	return storage, nil
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

// NewStorage creates a storage backend based on configuration
// This is a convenience function that uses the default factory
func NewStorage(ctx context.Context) (Storage, error) {
	factory := NewFactory()
	return factory.CreateStorageFromConfig(ctx)
}

// NewStorageWithType creates a storage backend of a specific type
// This is useful for testing or when you want to override the config
func NewStorageWithType(ctx context.Context, storageType StorageType) (Storage, error) {
	factory := NewFactory()
	return factory.CreateStorage(ctx, storageType)
}

// ValidateStorageConfig validates the storage configuration for a given type
func ValidateStorageConfig(storageType StorageType) error {
	switch storageType {
	case StorageTypeS3, StorageTypeR2:
		if config.S3Bucket == "" {
			return fmt.Errorf("S3_BUCKET is required for %s storage", storageType)
		}
		if config.S3AccessKey == "" {
			return fmt.Errorf("AWS_ACCESS_KEY_ID is required for %s storage", storageType)
		}
		if config.S3SecretKey == "" {
			return fmt.Errorf("AWS_SECRET_ACCESS_KEY is required for %s storage", storageType)
		}
		return nil
	case StorageTypeGoogleDrive:
		// Google Drive validation would go here (checking for credentials file, etc.)
		return nil
	default:
		return fmt.Errorf("unknown storage type: %s", storageType)
	}
}

// GetAvailableStorageTypes returns a list of all supported storage types
func GetAvailableStorageTypes() []StorageType {
	return []StorageType{
		StorageTypeGoogleDrive,
		StorageTypeS3,
		StorageTypeR2,
	}
}

// IsValidStorageType checks if a storage type is supported
func IsValidStorageType(storageType string) bool {
	switch StorageType(storageType) {
	case StorageTypeGoogleDrive, StorageTypeS3, StorageTypeR2:
		return true
	default:
		return false
	}
}

// GetStorageTypeFromString converts a string to StorageType with validation
func GetStorageTypeFromString(s string) (StorageType, error) {
	storageType := StorageType(s)
	if !IsValidStorageType(s) {
		return "", fmt.Errorf("invalid storage type: %s. Valid types: %v", s, GetAvailableStorageTypes())
	}
	return storageType, nil
}
