package podcast

import (
	"testing"
	"time"

	"cobblepod/internal/sources"
	"cobblepod/internal/storage/mock"
)

func TestCanReuseEpisode(t *testing.T) {
	tests := []struct {
		name                    string
		newEpisode              sources.AudioEntry
		existingEpisode         ExistingEpisode
		speed                   float64
		extractFileIDResult     string
		fileExistsResult        bool
		fileExistsError         error
		expectedFileExistsCalls int
		expectedResult          bool
		description             string
	}{
		{
			name: "perfect_match_with_good_file_id",
			newEpisode: sources.AudioEntry{
				Title:    "Test Episode",
				Duration: 60 * time.Second, // 60 seconds
				Offset:   10 * time.Second, // 10 second offset
			},
			existingEpisode: ExistingEpisode{
				DownloadURL:      "https://example.com/file123",
				Duration:         50 * time.Second, // 50 seconds (60 - 10 offset, no speed change)
				OriginalDuration: 60 * time.Second, // 60 seconds original
			},
			speed:                   1.0, // Normal speed
			extractFileIDResult:     "valid-file-id-123",
			fileExistsResult:        true,
			expectedFileExistsCalls: 1,
			expectedResult:          true,
			description:             "Should return true when durations match and file exists with valid file ID",
		},
		{
			name: "empty_file_id_should_return_false",
			newEpisode: sources.AudioEntry{
				Title:    "Test Episode",
				Duration: 60 * time.Second,
				Offset:   10 * time.Second,
			},
			existingEpisode: ExistingEpisode{
				DownloadURL:      "https://example.com/invalid-url",
				Duration:         50 * time.Second,
				OriginalDuration: 60 * time.Second,
			},
			speed:                   1.0,
			extractFileIDResult:     "",    // Empty file ID
			fileExistsResult:        false, // FileExists will be called with empty string and should return false
			expectedFileExistsCalls: 0,

			expectedResult: false,
			description:    "Should return false when ExtractFileIDFromURL returns empty string",
		},
		{
			name: "valid_file_id_but_file_does_not_exist",
			newEpisode: sources.AudioEntry{
				Title:    "Test Episode",
				Duration: 60 * time.Second,
				Offset:   10 * time.Second,
			},
			existingEpisode: ExistingEpisode{
				DownloadURL:      "https://example.com/file456",
				Duration:         50 * time.Second,
				OriginalDuration: 60 * time.Second,
			},
			speed:                   1.0,
			expectedFileExistsCalls: 1,
			extractFileIDResult:     "valid-file-id-456",
			fileExistsResult:        false, // File doesn't exist
			expectedResult:          false,
			description:             "Should return false when file ID is valid but file doesn't exist",
		},
		{
			name: "duration_mismatch_with_good_file_id",
			newEpisode: sources.AudioEntry{
				Title:    "Test Episode",
				Duration: 70 * time.Second, // Different original duration
				Offset:   10 * time.Second,
			},
			existingEpisode: ExistingEpisode{
				DownloadURL:      "https://example.com/file789",
				Duration:         50 * time.Second,
				OriginalDuration: 60 * time.Second, // Different from new episode
			},
			speed:                   1.0,
			extractFileIDResult:     "valid-file-id-789",
			fileExistsResult:        true,
			expectedFileExistsCalls: 1,
			expectedResult:          false,
			description:             "Should return false when original durations don't match even with valid file ID",
		},
		{
			name: "processed_duration_mismatch_with_good_file_id",
			newEpisode: sources.AudioEntry{
				Title:    "Test Episode",
				Duration: 60 * time.Second,
				Offset:   20 * time.Second, // Different offset
			},
			existingEpisode: ExistingEpisode{
				DownloadURL:      "https://example.com/file101",
				Duration:         50 * time.Second, // This won't match new processed duration of 40s
				OriginalDuration: 60 * time.Second,
			},
			speed:                   1.0,
			extractFileIDResult:     "valid-file-id-101",
			fileExistsResult:        true,
			expectedFileExistsCalls: 1,
			expectedResult:          false,
			description:             "Should return false when processed durations don't match even with valid file ID",
		},
		{
			name: "speed_adjustment_with_good_file_id",
			newEpisode: sources.AudioEntry{
				Title:    "Test Episode",
				Duration: 60 * time.Second,
				Offset:   10 * time.Second,
			},
			existingEpisode: ExistingEpisode{
				DownloadURL:      "https://example.com/file202",
				Duration:         25 * time.Second, // 50 seconds / 2.0 speed = 25 seconds
				OriginalDuration: 60 * time.Second,
			},
			speed:                   2.0, // 2x speed
			extractFileIDResult:     "valid-file-id-202",
			fileExistsResult:        true,
			expectedFileExistsCalls: 1,
			expectedResult:          true,
			description:             "Should return true when durations match after speed adjustment with valid file ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock storage
			mockStorage := mock.NewMockStorage()

			// Configure the mock for ExtractFileIDFromURL
			mockStorage.ExtractFileIDFromURLFunc = func(url string) string {
				return tt.extractFileIDResult
			}

			// Configure the mock for FileExists
			mockStorage.FileExistsResult = tt.fileExistsResult
			mockStorage.FileExistsError = tt.fileExistsError

			// Create RSS processor with mock storage
			processor := NewRSSProcessor("Test Channel", mockStorage)

			// Test CanReuseEpisode
			result := processor.CanReuseEpisode(tt.newEpisode, tt.existingEpisode, tt.speed)

			// Verify result
			if result != tt.expectedResult {
				t.Errorf("CanReuseEpisode() = %v, want %v. %s", result, tt.expectedResult, tt.description)
			}

			// Verify ExtractFileIDFromURL was called with correct URL
			if len(mockStorage.ExtractFileIDFromURLCalls) != 1 {
				t.Errorf("Expected ExtractFileIDFromURL to be called once, got %d calls", len(mockStorage.ExtractFileIDFromURLCalls))
			} else if mockStorage.ExtractFileIDFromURLCalls[0] != tt.existingEpisode.DownloadURL {
				t.Errorf("ExtractFileIDFromURL called with %q, want %q", mockStorage.ExtractFileIDFromURLCalls[0], tt.existingEpisode.DownloadURL)
			}

			// Verify FileExists was called with the extracted file ID
			if len(mockStorage.FileExistsCalls) != tt.expectedFileExistsCalls {
				t.Errorf("Expected FileExists to be called once, got %d calls", len(mockStorage.FileExistsCalls))
			}
		})
	}
}
