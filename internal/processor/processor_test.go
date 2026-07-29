package processor

import (
	"context"
	"errors"
	"testing"

	"cobblepod/internal/auth"
	"cobblepod/internal/integration/playrun"
	"cobblepod/internal/podcast"
	"cobblepod/internal/queue"
	"cobblepod/internal/storage/mock"
)

// MockJobTracker is a mock implementation of the JobTracker interface
type MockJobTracker struct{}

func (m *MockJobTracker) SetJobItems(ctx context.Context, jobID string, items []queue.JobItem) error {
	return nil
}

func (m *MockJobTracker) UpdateJobItem(ctx context.Context, jobID string, item queue.JobItem) error {
	return nil
}

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

func (m *MockGDriveService) DeleteFile(ctx context.Context, fileID string) error {
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
			proc := NewProcessorWithDependencies(nil, &auth.MockTokenProvider{}, nil, &MockJobTracker{}, nil)
			proc.deleteUnusedEpisodes(context.Background(), mockService, tt.episodeMapping, tt.reused)

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
		proc := NewProcessorWithDependencies(nil, &auth.MockTokenProvider{}, nil, &MockJobTracker{}, nil)
		proc.deleteUnusedEpisodes(context.Background(), mockService, nil, nil)

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

		proc := NewProcessorWithDependencies(nil, &auth.MockTokenProvider{}, nil, &MockJobTracker{}, nil)
		proc.deleteUnusedEpisodes(context.Background(), mockService, episodeMapping, reused)

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

		proc := NewProcessorWithDependencies(nil, &auth.MockTokenProvider{}, nil, &MockJobTracker{}, nil)
		proc.deleteUnusedEpisodes(context.Background(), mockService, episodeMapping, reused)

		deletedFiles := mockService.GetDeletedFiles()
		if len(deletedFiles) != 0 {
			t.Errorf("Expected no deletions when episode is reused (regardless of data differences), got %d", len(deletedFiles))
		}
	})
}

func TestProcessor_Run_AuthFailure(t *testing.T) {
	mockTokenProvider := &auth.MockTokenProvider{
		Err: errors.New("auth failed"),
	}

	proc := NewProcessorWithDependencies(nil, mockTokenProvider, nil, &MockJobTracker{}, nil)

	job := &queue.Job{
		ID:     "job1",
		FileID: "file1",
		UserID: "user1",
	}

	err := proc.Run(context.Background(), job)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	expectedError := "failed to get Google access token for user user1: auth failed"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}

func TestProcessor_Run_StorageCreationFailure(t *testing.T) {
	mockTokenProvider := &auth.MockTokenProvider{
		Token: "valid-token",
	}

	expectedErr := errors.New("storage creation failed")
	mockStorageCreator := mock.NewMockStorageCreator(nil, expectedErr)

	proc := NewProcessorWithDependencies(nil, mockTokenProvider, mockStorageCreator, &MockJobTracker{}, nil)

	job := &queue.Job{
		ID:     "job1",
		FileID: "file1",
		UserID: "user1",
	}

	err := proc.Run(context.Background(), job)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	expectedErrorMsg := "failed to create storage service with user token: storage creation failed"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error %q, got %q", expectedErrorMsg, err.Error())
	}
}

// --- syncPlayrun tests ---

// mockPlayrunClient records every call made to it and returns canned responses.
type mockPlayrunClient struct {
	validateErr        error
	upsertUUID         string
	upsertErr          error
	playlist           []playrun.PlaylistEntry
	playlistErr        error
	subscribeErr       error            // default error for all Subscribe calls
	subscribeErrByUUID map[string]error // per-uuid override

	validateCalls    int
	upsertCalls      []string
	subscribeCalls   []playrun.PlaylistEntry
	unsubscribeCalls []mockUnsubCall
}

type mockUnsubCall struct {
	EpisodeUUID string
	PodcastUUID string
}

func (m *mockPlayrunClient) Validate(ctx context.Context) error {
	m.validateCalls++
	return m.validateErr
}

func (m *mockPlayrunClient) UpsertPodcast(ctx context.Context, feedURL string) (string, error) {
	m.upsertCalls = append(m.upsertCalls, feedURL)
	return m.upsertUUID, m.upsertErr
}

func (m *mockPlayrunClient) GetPlaylist(ctx context.Context) ([]playrun.PlaylistEntry, error) {
	return m.playlist, m.playlistErr
}

func (m *mockPlayrunClient) Subscribe(ctx context.Context, ep playrun.PlaylistEntry) error {
	m.subscribeCalls = append(m.subscribeCalls, ep)
	if m.subscribeErr != nil {
		return m.subscribeErr
	}
	if err, ok := m.subscribeErrByUUID[ep.UUID]; ok {
		return err
	}
	return nil
}

func (m *mockPlayrunClient) Unsubscribe(ctx context.Context, episodeUUID, podcastUUID string) error {
	m.unsubscribeCalls = append(m.unsubscribeCalls, mockUnsubCall{
		EpisodeUUID: episodeUUID,
		PodcastUUID: podcastUUID,
	})
	return nil
}

// recordingJobTracker captures every UpdateJobItem call so we can assert
// status transitions on individual items.
type recordingJobTracker struct {
	updates []queue.JobItem
}

func (r *recordingJobTracker) SetJobItems(ctx context.Context, jobID string, items []queue.JobItem) error {
	return nil
}

func (r *recordingJobTracker) UpdateJobItem(ctx context.Context, jobID string, item queue.JobItem) error {
	r.updates = append(r.updates, item)
	return nil
}

// statusByTitle returns the last status written for a given item title.
func (r *recordingJobTracker) statusByTitle(title string) (queue.JobItemStatus, bool) {
	var last queue.JobItemStatus
	found := false
	for _, u := range r.updates {
		if u.Title == title {
			last = u.Status
			found = true
		}
	}
	return last, found
}

// newSyncProc builds a Processor wired with a mock Playrun client.
func newSyncProc(mc *mockPlayrunClient) (*Processor, *recordingJobTracker) {
	q := &recordingJobTracker{}
	factory := func(jwt string) playrun.Client { return mc }
	proc := NewProcessorWithDependencies(nil, &auth.MockTokenProvider{}, nil, q, factory)
	return proc, q
}

func TestSyncPlayrun_NotLinked(t *testing.T) {
	mc := &mockPlayrunClient{}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
	}, storage, "https://feed.example/rss", "")

	if mc.validateCalls != 0 {
		t.Errorf("expected no API calls when jwt is empty, got %d validate calls", mc.validateCalls)
	}
	if len(q.updates) != 0 {
		t.Errorf("expected no status updates when jwt is empty, got %d", len(q.updates))
	}
}

func TestSyncPlayrun_EmptyFeedURL(t *testing.T) {
	mc := &mockPlayrunClient{}
	proc, _ := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
	}, storage, "", "jwt-token")

	if mc.validateCalls != 0 {
		t.Errorf("expected no API calls when feedURL is empty, got %d validate calls", mc.validateCalls)
	}
}

func TestSyncPlayrun_ValidateFails(t *testing.T) {
	mc := &mockPlayrunClient{validateErr: errors.New("bad token")}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
		{Title: "Ep2", Status: queue.StatusUploaded},
		{Title: "Ep3", Status: queue.StatusSkipped}, // should NOT be touched
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
		{Title: "Ep2", DownloadURL: "https://x/ep2.mp3"},
		{Title: "Ep3", DownloadURL: "https://x/ep3.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	if len(mc.upsertCalls) != 0 {
		t.Errorf("expected no upsert call when validate fails, got %d", len(mc.upsertCalls))
	}
	for _, title := range []string{"Ep1", "Ep2"} {
		if s, ok := q.statusByTitle(title); !ok || s != queue.StatusSyncFailed {
			t.Errorf("expected %s -> StatusSyncFailed, got %v (found=%v)", title, s, ok)
		}
	}
	if s, ok := q.statusByTitle("Ep3"); ok {
		t.Errorf("expected StatusSkipped item to remain untouched, got update to %v", s)
	}
}

func TestSyncPlayrun_UpsertFails(t *testing.T) {
	mc := &mockPlayrunClient{upsertErr: errors.New("upsert boom")}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	if s, ok := q.statusByTitle("Ep1"); !ok || s != queue.StatusSyncFailed {
		t.Errorf("expected Ep1 -> StatusSyncFailed, got %v (found=%v)", s, ok)
	}
}

func TestSyncPlayrun_GetPlaylistFails(t *testing.T) {
	mc := &mockPlayrunClient{
		upsertUUID:  "pod-uuid",
		playlistErr: errors.New("playlist boom"),
	}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	if s, ok := q.statusByTitle("Ep1"); !ok || s != queue.StatusSyncFailed {
		t.Errorf("expected Ep1 -> StatusSyncFailed, got %v (found=%v)", s, ok)
	}
}

func TestSyncPlayrun_SubscribesNewEpisodes(t *testing.T) {
	mc := &mockPlayrunClient{
		upsertUUID: "pod-uuid",
		playlist:   nil, // empty playlist
	}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
		{Title: "Ep2", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
		{Title: "Ep2", DownloadURL: "https://x/ep2.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	if len(mc.subscribeCalls) != 2 {
		t.Fatalf("expected 2 subscribe calls, got %d", len(mc.subscribeCalls))
	}
	for _, title := range []string{"Ep1", "Ep2"} {
		if s, ok := q.statusByTitle(title); !ok || s != queue.StatusSynced {
			t.Errorf("expected %s -> StatusSynced, got %v (found=%v)", title, s, ok)
		}
	}
}

func TestSyncPlayrun_AlreadyOnPlaylist(t *testing.T) {
	ep1 := playrun.MakeEpisode("Ep1", "https://x/ep1.mp3", "pod-uuid")
	mc := &mockPlayrunClient{
		upsertUUID: "pod-uuid",
		playlist:   []playrun.PlaylistEntry{ep1},
	}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	if len(mc.subscribeCalls) != 0 {
		t.Errorf("expected 0 subscribe calls (already on playlist), got %d", len(mc.subscribeCalls))
	}
	if s, ok := q.statusByTitle("Ep1"); !ok || s != queue.StatusSynced {
		t.Errorf("expected Ep1 -> StatusSynced, got %v (found=%v)", s, ok)
	}
}

func TestSyncPlayrun_PerItemSubscribeFails(t *testing.T) {
	bad := playrun.MakeEpisode("Ep2", "https://x/ep2.mp3", "pod-uuid")
	mc := &mockPlayrunClient{
		upsertUUID:         "pod-uuid",
		subscribeErrByUUID: map[string]error{bad.UUID: errors.New("subscribe boom")},
	}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
		{Title: "Ep2", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DownloadURL: "https://x/ep1.mp3"},
		{Title: "Ep2", DownloadURL: "https://x/ep2.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	if s, ok := q.statusByTitle("Ep1"); !ok || s != queue.StatusSynced {
		t.Errorf("expected Ep1 -> StatusSynced, got %v (found=%v)", s, ok)
	}
	if s, ok := q.statusByTitle("Ep2"); !ok || s != queue.StatusSyncFailed {
		t.Errorf("expected Ep2 -> StatusSyncFailed, got %v (found=%v)", s, ok)
	}
}

func TestSyncPlayrun_UnsubscribesStaleOnlyFromOurPodcast(t *testing.T) {
	// 'keep' is in desired (still published). 'stale' belongs to our podcast
	// but is no longer in desired → unsubscribe. 'foreign' belongs to a
	// different podcast → must NOT be unsubscribed.
	keep := playrun.MakeEpisode("Ep-Keep", "https://x/keep.mp3", "pod-uuid")
	stale := playrun.PlaylistEntry{
		UUID:    "stale-uuid",
		Title:   "Stale",
		URL:     "https://x/stale.mp3",
		Type:    "mp3",
		Podcast: playrun.PlaylistPodcast{UUID: "pod-uuid", Title: playrun.PodcastTitle},
	}
	foreign := playrun.PlaylistEntry{
		UUID:    "foreign-uuid",
		Title:   "Foreign",
		URL:     "https://x/foreign.mp3",
		Type:    "mp3",
		Podcast: playrun.PlaylistPodcast{UUID: "other-pod", Title: "Someone Else's Podcast"},
	}
	mc := &mockPlayrunClient{
		upsertUUID: "pod-uuid",
		playlist:   []playrun.PlaylistEntry{keep, stale, foreign},
	}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep-Keep", Status: queue.StatusUploaded},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep-Keep", DownloadURL: "https://x/keep.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	// Only 'stale' should have been unsubscribed.
	if len(mc.unsubscribeCalls) != 1 {
		t.Fatalf("expected 1 unsubscribe call, got %d: %+v", len(mc.unsubscribeCalls), mc.unsubscribeCalls)
	}
	if mc.unsubscribeCalls[0].EpisodeUUID != "stale-uuid" {
		t.Errorf("expected to unsubscribe 'stale-uuid', got %q", mc.unsubscribeCalls[0].EpisodeUUID)
	}
	if mc.unsubscribeCalls[0].PodcastUUID != "pod-uuid" {
		t.Errorf("expected unsubscribe to use pod-uuid, got %q", mc.unsubscribeCalls[0].PodcastUUID)
	}
	// 'keep' is already on playlist → no subscribe, but item moves to StatusSynced.
	if len(mc.subscribeCalls) != 0 {
		t.Errorf("expected 0 subscribe calls, got %d", len(mc.subscribeCalls))
	}
	if s, ok := q.statusByTitle("Ep-Keep"); !ok || s != queue.StatusSynced {
		t.Errorf("expected Ep-Keep -> StatusSynced, got %v (found=%v)", s, ok)
	}
}

func TestSyncPlayrun_StatusSkippedItemsLeftAlone(t *testing.T) {
	// Reused episode appears in both playlist and desired; it stays in
	// StatusSkipped (set by upstream) — syncPlayrun must not transition it.
	reused := playrun.MakeEpisode("Ep-Reused", "https://x/reused.mp3", "pod-uuid")
	mc := &mockPlayrunClient{
		upsertUUID: "pod-uuid",
		playlist:   []playrun.PlaylistEntry{reused},
	}
	proc, q := newSyncProc(mc)

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep-Reused", Status: queue.StatusSkipped},
	}}
	storage := mock.NewMockStorage()

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep-Reused", DownloadURL: "https://x/reused.mp3"},
	}, storage, "https://feed.example/rss", "jwt")

	if len(mc.subscribeCalls) != 0 {
		t.Errorf("expected 0 subscribe calls for reused episode, got %d", len(mc.subscribeCalls))
	}
	if len(mc.unsubscribeCalls) != 0 {
		t.Errorf("reused episode must not be unsubscribed, got %d calls", len(mc.unsubscribeCalls))
	}
	if s, ok := q.statusByTitle("Ep-Reused"); ok {
		t.Errorf("expected no status update for StatusSkipped item, got %v", s)
	}
}

func TestSyncPlayrun_ResolvesDriveFileIDWhenDownloadURLEmpty(t *testing.T) {
	// Uploaded-but-not-reused episodes have DriveFileID set and DownloadURL
	// empty (matches uploadResults in processor.go). syncPlayrun must resolve
	// the enclosure URL via storageService.GenerateDownloadURL.
	mc := &mockPlayrunClient{
		upsertUUID: "pod-uuid",
	}
	proc, q := newSyncProc(mc)

	storage := mock.NewMockStorage()
	storage.GenerateDownloadURLFunc = func(id string) string {
		return "https://resolved.example.com/" + id
	}

	job := &queue.Job{ID: "j1", UserID: "u1", Items: []queue.JobItem{
		{Title: "Ep1", Status: queue.StatusUploaded},
	}}

	proc.syncPlayrun(context.Background(), job, []podcast.ProcessedEpisode{
		{Title: "Ep1", DriveFileID: "fileID-123"}, // DownloadURL empty
	}, storage, "https://feed.example/rss", "jwt")

	if len(mc.subscribeCalls) != 1 {
		t.Fatalf("expected 1 subscribe call, got %d", len(mc.subscribeCalls))
	}
	wantURL := "https://resolved.example.com/fileID-123"
	if mc.subscribeCalls[0].URL != wantURL {
		t.Errorf("expected resolved URL %q, got %q", wantURL, mc.subscribeCalls[0].URL)
	}
	if mc.subscribeCalls[0].UUID != playrun.EpisodeUUID(wantURL) {
		t.Errorf("expected uuid to be sha1(resolved URL), got %q", mc.subscribeCalls[0].UUID)
	}
	if s, ok := q.statusByTitle("Ep1"); !ok || s != queue.StatusSynced {
		t.Errorf("expected Ep1 -> StatusSynced, got %v (found=%v)", s, ok)
	}
}
