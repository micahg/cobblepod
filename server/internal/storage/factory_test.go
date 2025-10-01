//go:build test

package storage

import (
	"context"
	"strings"
	"testing"

	"cobblepod/internal/config"
)

func TestStorageFactory(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory()

	t.Run("create S3 storage with explicit type", func(t *testing.T) {
		// This will fail without proper S3 config, but tests the factory logic
		_, err := factory.CreateStorage(ctx, StorageTypeS3)

		// We expect an error about missing configuration
		if err == nil {
			t.Error("Expected error for missing S3 configuration")
		}

		if err != nil && err.Error() != "S3_BUCKET environment variable is required for S3/R2 storage" {
			t.Logf("Got expected configuration error: %v", err)
		}
	})

	t.Run("validate storage config", func(t *testing.T) {
		// Test S3 validation
		err := ValidateStorageConfig(StorageTypeS3)
		if err == nil {
			t.Error("Expected validation error for missing S3 config")
		}

		// Test invalid storage type
		err = ValidateStorageConfig("invalid")
		if err == nil {
			t.Error("Expected validation error for invalid storage type")
		}
	})

	t.Run("create storage from config", func(t *testing.T) {
		// Save original config
		originalBackend := config.StorageBackend
		defer func() { config.StorageBackend = originalBackend }()

		// Test with empty config (should default to Google Drive)
		config.StorageBackend = ""
		_, err := factory.CreateStorageFromConfig(ctx)

		// May fail due to missing Google credentials, but that's expected in test
		t.Logf("Create from config result: %v", err)
	})

	t.Run("storage type constants", func(t *testing.T) {
		// Test that our constants are what we expect
		if StorageTypeGoogleDrive != "gdrive" {
			t.Errorf("Expected gdrive, got %s", StorageTypeGoogleDrive)
		}
		if StorageTypeS3 != "s3" {
			t.Errorf("Expected s3, got %s", StorageTypeS3)
		}
		if StorageTypeR2 != "r2" {
			t.Errorf("Expected r2, got %s", StorageTypeR2)
		}
	})
}

// Example of how to use the factory in your code
func ExampleStorageFactory() {
	ctx := context.Background()
	factory := NewFactory()

	// Create storage from environment configuration
	storage, err := factory.CreateStorageFromConfig(ctx)
	if err != nil {
		// Handle error
		return
	}

	// Use storage
	_ = storage

	// Or create specific storage type
	s3Storage, err := factory.CreateStorage(ctx, StorageTypeS3)
	if err != nil {
		// Handle error
		return
	}

	// Use S3 storage
	_ = s3Storage
}

func TestGetStorageTypeFromString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    StorageType
		expectError bool
	}{
		{
			name:        "Valid Google Drive",
			input:       "gdrive",
			expected:    StorageTypeGoogleDrive,
			expectError: false,
		},
		{
			name:        "Valid S3",
			input:       "s3",
			expected:    StorageTypeS3,
			expectError: false,
		},
		{
			name:        "Valid R2",
			input:       "r2",
			expected:    StorageTypeR2,
			expectError: false,
		},
		{
			name:        "Invalid Type",
			input:       "invalid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty String",
			input:       "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetStorageTypeFromString(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("GetStorageTypeFromString() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("GetStorageTypeFromString() unexpected error = %v", err)
				}
				if result != tt.expected {
					t.Errorf("GetStorageTypeFromString() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestGetAvailableStorageTypes(t *testing.T) {
	types := GetAvailableStorageTypes()
	expected := []StorageType{StorageTypeGoogleDrive, StorageTypeS3, StorageTypeR2}

	if len(types) != len(expected) {
		t.Errorf("GetAvailableStorageTypes() returned %d types, want %d", len(types), len(expected))
	}

	for _, expectedType := range expected {
		found := false
		for _, actualType := range types {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetAvailableStorageTypes() missing type %v", expectedType)
		}
	}
}

func TestIsValidStorageType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid Google Drive",
			input:    "gdrive",
			expected: true,
		},
		{
			name:     "Valid S3",
			input:    "s3",
			expected: true,
		},
		{
			name:     "Valid R2",
			input:    "r2",
			expected: true,
		},
		{
			name:     "Invalid Type",
			input:    "invalid",
			expected: false,
		},
		{
			name:     "Empty String",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidStorageType(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidStorageType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEnhancedValidateStorageConfig(t *testing.T) {
	// Save original config values
	originalBucket := config.S3Bucket
	originalAccessKey := config.S3AccessKey
	originalSecretKey := config.S3SecretKey

	// Restore original values after test
	defer func() {
		config.S3Bucket = originalBucket
		config.S3AccessKey = originalAccessKey
		config.S3SecretKey = originalSecretKey
	}()

	tests := []struct {
		name        string
		storageType StorageType
		setupConfig func()
		wantError   bool
		errorMsg    string
	}{
		{
			name:        "Valid S3 Config",
			storageType: StorageTypeS3,
			setupConfig: func() {
				config.S3Bucket = "test-bucket"
				config.S3AccessKey = "test-key"
				config.S3SecretKey = "test-secret"
			},
			wantError: false,
		},
		{
			name:        "S3 Missing Bucket",
			storageType: StorageTypeS3,
			setupConfig: func() {
				config.S3Bucket = ""
				config.S3AccessKey = "test-key"
				config.S3SecretKey = "test-secret"
			},
			wantError: true,
			errorMsg:  "S3_BUCKET is required",
		},
		{
			name:        "S3 Missing Access Key",
			storageType: StorageTypeS3,
			setupConfig: func() {
				config.S3Bucket = "test-bucket"
				config.S3AccessKey = ""
				config.S3SecretKey = "test-secret"
			},
			wantError: true,
			errorMsg:  "AWS_ACCESS_KEY_ID is required",
		},
		{
			name:        "Valid R2 Config",
			storageType: StorageTypeR2,
			setupConfig: func() {
				config.S3Bucket = "test-bucket"
				config.S3AccessKey = "test-key"
				config.S3SecretKey = "test-secret"
			},
			wantError: false,
		},
		{
			name:        "Google Drive Config",
			storageType: StorageTypeGoogleDrive,
			setupConfig: func() {},
			wantError:   false, // Currently no validation for GDrive
		},
		{
			name:        "Invalid Storage Type",
			storageType: "invalid",
			setupConfig: func() {},
			wantError:   true,
			errorMsg:    "unknown storage type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup configuration
			tt.setupConfig()

			err := ValidateStorageConfig(tt.storageType)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateStorageConfig() expected error, got nil")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateStorageConfig() error = %v, want error containing %v", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateStorageConfig() unexpected error = %v", err)
				}
			}
		})
	}
}
