package processor

import (
	"testing"

	"cobblepod/internal/podcast"
)

// MockGDriveService is a mock implementation of the GDriveDeleter interface for testing
type MockGDriveService struct {
	deletedFiles []string
	urlToIDMap   map[string]string
	deleteError  error
}

func NewMockGDriveService() *MockGDriveService {
	return &MockGDriveService{
		deletedFiles: make([]string, 0),
		urlToIDMap:   make(map[string]string),
	}
}

func (m *MockGDriveService) ExtractFileIDFromURL(url string) string {
	if id, exists := m.urlToIDMap[url]; exists {
		return id
	}
	return ""
}

func (m *MockGDriveService) DeleteFile(fileID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deletedFiles = append(m.deletedFiles, fileID)
	return nil
}

// Helper method to set up URL to ID mapping for tests
func (m *MockGDriveService) SetURLToIDMapping(url, id string) {
	m.urlToIDMap[url] = id
}

// Helper method to get deleted files for assertions
func (m *MockGDriveService) GetDeletedFiles() []string {
	return m.deletedFiles
}

// Helper method to simulate delete errors
func (m *MockGDriveService) SetDeleteError(err error) {
	m.deleteError = err
}

func TestDeleteUnusedEpisodes(t *testing.T) {
	tests := []struct {
		name            string
		episodeMapping  map[string]podcast.ExistingEpisode
		reused          map[string]podcast.ExistingEpisode
		urlToIDMap      map[string]string
		expectedDeletes []string
	}{
		{
			name: "delete episodes not in reused map",
			episodeMapping: map[string]podcast.ExistingEpisode{
				"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1"},
				"Episode 2": {DownloadURL: "https://drive.google.com/file/d/file2"},
				"Episode 3": {DownloadURL: "https://drive.google.com/file/d/file3"},
			},
			reused: map[string]podcast.ExistingEpisode{
				"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1"},
			},
			urlToIDMap: map[string]string{
				"https://drive.google.com/file/d/file1": "file1",
				"https://drive.google.com/file/d/file2": "file2",
				"https://drive.google.com/file/d/file3": "file3",
			},
			expectedDeletes: []string{"file2", "file3"},
		},
		{
			name: "no deletions when all episodes are reused",
			episodeMapping: map[string]podcast.ExistingEpisode{
				"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1"},
				"Episode 2": {DownloadURL: "https://drive.google.com/file/d/file2"},
			},
			reused: map[string]podcast.ExistingEpisode{
				"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1"},
				"Episode 2": {DownloadURL: "https://drive.google.com/file/d/file2"},
			},
			urlToIDMap: map[string]string{
				"https://drive.google.com/file/d/file1": "file1",
				"https://drive.google.com/file/d/file2": "file2",
			},
			expectedDeletes: []string{},
		},
		{
			name: "delete all episodes when none are reused",
			episodeMapping: map[string]podcast.ExistingEpisode{
				"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1"},
				"Episode 2": {DownloadURL: "https://drive.google.com/file/d/file2"},
			},
			reused: map[string]podcast.ExistingEpisode{},
			urlToIDMap: map[string]string{
				"https://drive.google.com/file/d/file1": "file1",
				"https://drive.google.com/file/d/file2": "file2",
			},
			expectedDeletes: []string{"file1", "file2"},
		},
		{
			name: "skip episodes with invalid URLs",
			episodeMapping: map[string]podcast.ExistingEpisode{
				"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1"},
				"Episode 2": {DownloadURL: "invalid-url"},
				"Episode 3": {DownloadURL: "https://drive.google.com/file/d/file3"},
			},
			reused: map[string]podcast.ExistingEpisode{},
			urlToIDMap: map[string]string{
				"https://drive.google.com/file/d/file1": "file1",
				"https://drive.google.com/file/d/file3": "file3",
				// invalid-url is not mapped, so ExtractFileIDFromURL will return ""
			},
			expectedDeletes: []string{"file1", "file3"},
		},
		{
			name:            "empty episode mapping",
			episodeMapping:  map[string]podcast.ExistingEpisode{},
			reused:          map[string]podcast.ExistingEpisode{},
			urlToIDMap:      map[string]string{},
			expectedDeletes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock service
			mockService := NewMockGDriveService()
			for url, id := range tt.urlToIDMap {
				mockService.SetURLToIDMapping(url, id)
			}

			// Call the actual function using our mock
			proc := NewProcessorWithDependencies(nil, nil)
			proc.deleteUnusedEpisodes(mockService, tt.episodeMapping, tt.reused)

			// Check results
			deletedFiles := mockService.GetDeletedFiles()

			if len(deletedFiles) != len(tt.expectedDeletes) {
				t.Errorf("Expected %d deletions, got %d", len(tt.expectedDeletes), len(deletedFiles))
			}

			// Check that all expected files were deleted
			expectedMap := make(map[string]bool)
			for _, expected := range tt.expectedDeletes {
				expectedMap[expected] = true
			}

			for _, deleted := range deletedFiles {
				if !expectedMap[deleted] {
					t.Errorf("Unexpected file deleted: %s", deleted)
				}
				delete(expectedMap, deleted)
			}

			// Check that all expected deletions occurred
			for remaining := range expectedMap {
				t.Errorf("Expected file %s to be deleted, but it wasn't", remaining)
			}
		})
	}
}

func TestDeleteUnusedEpisodesEdgeCases(t *testing.T) {
	t.Run("nil maps", func(t *testing.T) {
		mockService := NewMockGDriveService()

		// This should not panic
		proc := NewProcessorWithDependencies(nil, nil)
		proc.deleteUnusedEpisodes(mockService, nil, nil)

		deletedFiles := mockService.GetDeletedFiles()
		if len(deletedFiles) != 0 {
			t.Errorf("Expected no deletions with nil maps, got %d", len(deletedFiles))
		}
	})

	t.Run("empty string URL", func(t *testing.T) {
		mockService := NewMockGDriveService()

		episodeMapping := map[string]podcast.ExistingEpisode{
			"Episode 1": {DownloadURL: ""},
		}
		reused := map[string]podcast.ExistingEpisode{}

		proc := NewProcessorWithDependencies(nil, nil)
		proc.deleteUnusedEpisodes(mockService, episodeMapping, reused)

		deletedFiles := mockService.GetDeletedFiles()
		if len(deletedFiles) != 0 {
			t.Errorf("Expected no deletions with empty URL, got %d", len(deletedFiles))
		}
	})

	t.Run("reused episode with different data but same title", func(t *testing.T) {
		mockService := NewMockGDriveService()
		mockService.SetURLToIDMapping("https://drive.google.com/file/d/file1", "file1")

		episodeMapping := map[string]podcast.ExistingEpisode{
			"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1", OriginalGUID: "guid1"},
		}
		reused := map[string]podcast.ExistingEpisode{
			"Episode 1": {DownloadURL: "https://drive.google.com/file/d/file1", OriginalGUID: "guid2"},
		}

		proc := NewProcessorWithDependencies(nil, nil)
		proc.deleteUnusedEpisodes(mockService, episodeMapping, reused)

		deletedFiles := mockService.GetDeletedFiles()
		if len(deletedFiles) != 0 {
			t.Errorf("Expected no deletions when episode is reused (regardless of data differences), got %d", len(deletedFiles))
		}
	})
}
