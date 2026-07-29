package processor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"cobblepod/internal/audio"
	"cobblepod/internal/auth"
	"cobblepod/internal/config"
	"cobblepod/internal/integration/playrun"
	"cobblepod/internal/podcast"
	"cobblepod/internal/queue"
	"cobblepod/internal/sources"
	"cobblepod/internal/state"
	"cobblepod/internal/storage"
)

// Task represents a processing task for a single episode
type Task struct {
	Item     queue.JobItem
	TempPath string
	Result   podcast.ProcessedEpisode
	Err      error
}

// StorageDeleter interface for dependency injection
type StorageDeleter interface {
	ExtractFileIDFromURL(url string) string
	DeleteFile(ctx context.Context, fileID string) error
}

// JobTracker interface for tracking job progress
type JobTracker interface {
	SetJobItems(ctx context.Context, jobID string, items []queue.JobItem) error
	UpdateJobItem(ctx context.Context, jobID string, item queue.JobItem) error
}

// StorageCreator function type for creating storage service
type StorageCreator func(ctx context.Context, accessToken string) (storage.Storage, error)

// PlayrunClientFactory builds a Playrun client from a per-user JWT. The
// indirection lets tests inject a mock client without touching the network.
type PlayrunClientFactory func(jwt string) playrun.Client

// Processor handles the main processing logic
type Processor struct {
	state                state.CobblepodStateManager
	tokenProvider        auth.TokenProvider
	storageCreator       StorageCreator
	queue                JobTracker
	playrunClientFactory PlayrunClientFactory
}

// NewProcessor creates a new processor with default dependencies
func NewProcessor(ctx context.Context, q *queue.Queue) (*Processor, error) {
	state, err := state.NewStateManager(ctx)
	if err != nil {
		slog.Error("Failed to connect to state", "error", err)
		// Continue with nil state manager - we'll handle this in Run()
	}

	return &Processor{
		state:                state,
		tokenProvider:        &auth.DefaultTokenProvider{},
		storageCreator:       storage.NewServiceWithToken,
		queue:                q,
		playrunClientFactory: playrun.New,
	}, nil
}

// NewProcessorWithDependencies creates a new processor with injected dependencies for testing
func NewProcessorWithDependencies(
	state state.CobblepodStateManager,
	tokenProvider auth.TokenProvider,
	storageCreator StorageCreator,
	q JobTracker,
	playrunClientFactory PlayrunClientFactory,
) *Processor {
	if playrunClientFactory == nil {
		playrunClientFactory = playrun.New
	}
	return &Processor{
		state:                state,
		tokenProvider:        tokenProvider,
		storageCreator:       storageCreator,
		queue:                q,
		playrunClientFactory: playrunClientFactory,
	}
}

// Run executes the main processing logic for the given job
func (p *Processor) Run(ctx context.Context, job *queue.Job) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	slog.Info("Processing job", "job_id", job.ID, "file_id", job.FileID, "user_id", job.UserID)

	// Get Google access token for the user
	googleToken, err := p.tokenProvider.GetGoogleAccessToken(ctx, job.UserID)
	if err != nil {
		return fmt.Errorf("failed to get Google access token for user %s: %w", job.UserID, err)
	}

	slog.Info("Successfully obtained Google access token for user", "user_id", job.UserID)

	// Create storage service with user's Google token
	userStorage, err := p.storageCreator(ctx, googleToken)
	if err != nil {
		return fmt.Errorf("failed to create storage service with user token: %w", err)
	}

	// TODO: Stop processing M3U8 files
	m3u8src := sources.NewM3U8Source(userStorage)
	podcastAddictBackup := sources.NewPodcastAddictBackup(userStorage)

	audioProcessor := audio.NewProcessor()
	podcastProcessor := podcast.NewRSSProcessor("Playrun Addict Custom Feed", userStorage)

	// Use the stored state manager
	stateManager := p.state
	var appState *state.CobblepodState

	if stateManager != nil {
		var err error
		appState, err = stateManager.GetState(job.UserID)
		if err != nil {
			slog.Error("Failed to get state", "error", err, "user_id", job.UserID)
			slog.Info("Assuming first run")
			appState = &state.CobblepodState{}
		} else {
			slog.Debug("State loaded", "last_run", appState.LastRun.Format(time.RFC3339), "user_id", job.UserID)
		}
	} else {
		slog.Info("State manager not available, assuming first run")
		appState = &state.CobblepodState{}
	}

	// Get RSS feed and extract episode mapping
	rssFileID := podcastProcessor.GetRSSFeedID(ctx)
	episodeMapping := make(map[string]podcast.ExistingEpisode)
	if rssFileID != "" {
		rssContent, err := userStorage.DownloadFile(ctx, rssFileID)
		if err != nil {
			slog.Error("Error downloading RSS feed", "error", err)
		} else {
			episodeMapping, err = podcastProcessor.ExtractEpisodeMapping(rssContent)
			if err != nil {
				slog.Error("Error extracting episode mapping", "error", err)
			}
		}
	}

	startTime := time.Now()
	defer func() {
		if stateManager != nil {
			if err := stateManager.SaveState(job.UserID, &state.CobblepodState{LastRun: startTime, PlayrunJWT: appState.PlayrunJWT}); err != nil {
				slog.Error("Failed to save state", "error", err)
			}
		}
	}()

	// Check for new M3U8 file
	m3u8File, err := m3u8src.GetLatest(ctx)
	if err != nil {
		return fmt.Errorf("error getting latest M3U8 file: %w", err)
	}

	newM3U8 := false
	if m3u8File != nil && (appState.LastRun.IsZero() || m3u8File.ModifiedTime.After(appState.LastRun)) {
		newM3U8 = true
	}

	// Check for new backup file
	backupFile, err := podcastAddictBackup.GetLatest(ctx)
	if err != nil {
		slog.Error("Error getting latest backup file", "error", err)
	}

	newBackup := false
	if backupFile != nil && (appState.LastRun.IsZero() || backupFile.ModifiedTime.After(appState.LastRun)) {
		newBackup = true
	}

	// Determine processing mode
	var entries []queue.JobItem
	if newM3U8 {
		slog.Info("Processing M3U8 file", "name", m3u8File.File.Name, "modified", m3u8File.ModifiedTime.Format(time.RFC3339))

		entries, err = m3u8src.Process(ctx, m3u8File)
		if err != nil {
			return fmt.Errorf("error processing M3U8 file: %w", err)
		}

		// Process M3U8 as before, including backup for offsets
		podcastAddictBackup.AddListeningProgress(ctx, entries)
	} else if newBackup {
		slog.Info("Processing backup independently", "name", backupFile.FileName, "modified", backupFile.ModifiedTime.Format(time.RFC3339))

		// Process backup independently
		entries, err = podcastAddictBackup.Process(ctx, backupFile)
		if err != nil {
			return fmt.Errorf("error processing backup independently: %w", err)
		}
	} else {
		slog.Debug("No new M3U8 or backup files found since last run")
		return nil
	}
	if len(entries) == 0 {
		slog.Info("No entries found in M3U8 file")
		return nil
	}

	// Populate job items
	if err := p.queue.SetJobItems(ctx, job.ID, entries); err != nil {
		slog.Error("Failed to set job items", "error", err)
	}
	job.Items = entries

	reused, err := p.processEntries(ctx, episodeMapping, userStorage, audioProcessor, podcastProcessor, job, appState.PlayrunJWT)
	if err != nil {
		return err
	}

	// The feed has been published and the playlist reconciled inside
	// processEntries (via finalize). Nothing left to do here.
	_ = reused

	return nil
}

// downloadWorker handles download requests
func downloadWorker(ctx context.Context, processor *audio.Processor, tasks <-chan Task, results chan<- Task, q JobTracker, jobID string) {
	defer close(results)
	for task := range tasks {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			task.Err = ctx.Err()
			results <- task
			return
		default:
		}

		// Update status
		task.Item.Status = queue.StatusDownloading
		if err := q.UpdateJobItem(ctx, jobID, task.Item); err != nil {
			slog.Error("Failed to update job item status", "error", err)
		}

		tempPath, err := processor.DownloadFile(ctx, task.Item.SourceURL)
		task.TempPath = tempPath
		task.Err = err

		if err != nil {
			task.Item.Status = queue.StatusFailed
			task.Item.Error = err.Error()
			if err := q.UpdateJobItem(ctx, jobID, task.Item); err != nil {
				slog.Error("Failed to update job item status", "error", err)
			}
		}

		results <- task
	}
}

// ffmpegWorker handles FFmpeg processing requests
func ffmpegWorker(ctx context.Context, processor *audio.Processor, tasks <-chan Task, results chan<- Task, speed float64, q JobTracker, jobID string) {
	fileCount := 0
	defer func() {
		slog.Info("FFmpeg worker completed", "processed_files", fileCount)
	}()

	for task := range tasks {
		fileCount++
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			task.Err = ctx.Err()
			results <- task
			return
		default:
		}

		// Update status
		task.Item.Status = queue.StatusProcessing
		if err := q.UpdateJobItem(ctx, jobID, task.Item); err != nil {
			slog.Error("Failed to update job item status", "error", err)
		}

		slog.Info("Processing audio", "title", task.Item.Title, "speed", speed)
		outputPath, err := processor.ProcessAudio(ctx, task.TempPath, speed, task.Item.Offset)
		if err != nil {
			slog.Error("Error processing audio", "title", task.Item.Title, "error", err)
			task.Err = err
			task.Item.Status = queue.StatusFailed
			task.Item.Error = err.Error()
			if err := q.UpdateJobItem(ctx, jobID, task.Item); err != nil {
				slog.Error("Failed to update job item status", "error", err)
			}

			// Clean up temp file
			if cleanupErr := os.Remove(task.TempPath); cleanupErr != nil {
				slog.Warn("Failed to remove temp file", "path", task.TempPath, "error", cleanupErr)
			}
			results <- task
			continue
		}

		// Clean up input temp file
		if err := os.Remove(task.TempPath); err != nil {
			slog.Warn("Failed to remove temp file", "path", task.TempPath, "error", err)
		}

		newDuration := time.Duration(float64((task.Item.Duration - task.Item.Offset).Nanoseconds()) / speed)
		result := podcast.ProcessedEpisode{
			Title:            task.Item.Title,
			OriginalDuration: task.Item.Duration,
			NewDuration:      newDuration,
			UUID:             task.Item.ID,
			Speed:            speed,
			TempFile:         outputPath,
		}

		task.Result = result
		results <- task
	}
}

// uploadResults handles uploading processed audio files to storage backend
func uploadResults(ctx context.Context, storageService storage.Storage, tasks []Task, q JobTracker, jobID string) ([]podcast.ProcessedEpisode, error) {
	var results []podcast.ProcessedEpisode
	for i, task := range tasks {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping upload")
			return nil, ctx.Err()
		default:
		}

		result := task.Result

		// Skip upload for reused files that already have download_url
		if downloadURL := result.DownloadURL; downloadURL != "" {
			slog.Info("Skipping upload for reused file", "title", result.Title)
			// Extract file_id from download_url for consistency
			if fileID := storageService.ExtractFileIDFromURL(downloadURL); fileID != "" {
				result.DriveFileID = fileID
			}
			results = append(results, result)
			continue
		}

		// Update status
		task.Item.Status = queue.StatusUploading
		if err := q.UpdateJobItem(ctx, jobID, task.Item); err != nil {
			slog.Error("Failed to update job item status", "error", err)
		}

		slog.Info("Uploading to storage backend", "title", result.Title)
		tempFile := result.TempFile
		filename := fmt.Sprintf("%s.mp3", result.Title)

		fileID, err := storageService.UploadFile(ctx, tempFile, filename, "audio/mpeg")
		if err != nil {
			task.Item.Status = queue.StatusFailed
			task.Item.Error = err.Error()
			q.UpdateJobItem(ctx, jobID, task.Item)
			return nil, fmt.Errorf("failed to upload %s to storage backend: %w", result.Title, err)
		}

		// Clean up temp file
		if err := os.Remove(tempFile); err != nil {
			slog.Warn("Failed to remove temp file", "path", tempFile, "error", err)
		}

		result.DriveFileID = fileID
		results = append(results, result)

		// Update status
		task.Item.Status = queue.StatusUploaded
		if err := q.UpdateJobItem(ctx, jobID, task.Item); err != nil {
			slog.Error("Failed to update job item status", "error", err)
		}
		tasks[i] = task // Update task in slice if needed
	}

	return results, nil
}

// updateFeed creates and uploads the RSS XML feed and saves the application state.
// It returns the public download URL of the feed so downstream steps (Playrun
// registration) can use it without recomputing.
func updateFeed(ctx context.Context, podcastProcessor *podcast.RSSProcessor, storageService storage.Storage, results []podcast.ProcessedEpisode) (string, error) {
	// Create and upload RSS XML
	xmlFeed := podcastProcessor.CreateRSSXML(results)
	rssFileID, err := storageService.UploadString(ctx, xmlFeed, "playrun_addict.xml", "application/rss+xml", podcastProcessor.GetRSSFeedID(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to upload RSS feed: %w", err)
	}

	rssDownloadURL := storageService.GenerateDownloadURL(rssFileID)
	slog.Info("RSS Feed created", "download_url", rssDownloadURL)

	return rssDownloadURL, nil
}

// deleteUnusedEpisodes removes episodes from storage backend that are no longer in the current playlist
func (p *Processor) deleteUnusedEpisodes(ctx context.Context, storageService StorageDeleter, episodeMapping map[string]podcast.ExistingEpisode, reused map[string]podcast.ExistingEpisode) {
	// Delete episodes that are not reused
	for title, episode := range episodeMapping {
		if _, ok := reused[title]; ok {
			continue
		}
		fileId := storageService.ExtractFileIDFromURL(episode.DownloadURL)
		if fileId == "" {
			slog.Warn("Could not extract file ID from URL", "url", episode.DownloadURL)
			continue
		}
		slog.Info("Deleting unused episode from storage backend", "title", title, "file_id", fileId)
		if err := storageService.DeleteFile(ctx, fileId); err != nil {
			slog.Error("Failed to delete file from storage backend", "file_id", fileId, "error", err)
		}
	}
}

// processEntries returns the reused episodes
func (p *Processor) processEntries(ctx context.Context, episodeMapping map[string]podcast.ExistingEpisode, storageService storage.Storage, audioProcessor *audio.Processor, podcastProcessor *podcast.RSSProcessor, job *queue.Job, playrunJWT string) (map[string]podcast.ExistingEpisode, error) {
	// Process entries locally
	var tasks []Task

	// Start a single downloader worker with separate job and result channels
	dlRequests := make(chan Task, len(job.Items))
	dlResults := make(chan Task, len(job.Items))
	go downloadWorker(ctx, audioProcessor, dlRequests, dlResults, p.queue, job.ID)

	speed := config.DefaultSpeed

	reused := make(map[string]podcast.ExistingEpisode)
	// First pass: reuse check; enqueue downloads for the rest
	for _, item := range job.Items {
		title := item.Title

		// Reuse check
		if oldEp, exists := episodeMapping[title]; exists {
			if podcastProcessor.CanReuseEpisode(ctx, item, oldEp, speed) {
				slog.Info("Reusing existing processed file", "title", title)
				reused[title] = oldEp
				result := podcast.ProcessedEpisode{
					Title:            title,
					OriginalDuration: item.Duration,
					NewDuration:      oldEp.Duration,
					UUID:             item.ID,
					Speed:            speed,
					DownloadURL:      oldEp.DownloadURL,
					OriginalGUID:     oldEp.OriginalGUID,
				}

				// Update status
				item.Status = queue.StatusSkipped
				if err := p.queue.UpdateJobItem(ctx, job.ID, item); err != nil {
					slog.Error("Failed to update job item status", "error", err)
				}

				tasks = append(tasks, Task{
					Item:   item,
					Result: result,
				})
				continue
			}
		}

		// Send request and wait for response
		slog.Info("Enqueuing download", "title", title, "url", item.SourceURL)
		dlRequests <- Task{
			Item: item,
		}
	}
	// all done sending jobs
	close(dlRequests)

	// Start FFmpeg worker
	var wg sync.WaitGroup
	ffmpegJobs := make(chan Task, len(job.Items))
	ffmpegResults := make(chan Task, len(job.Items))
	for i := 0; i < config.MaxFFMPEGWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ffmpegWorker(ctx, audioProcessor, ffmpegJobs, ffmpegResults, speed, p.queue, job.ID)
		}()
	}

	for res := range dlResults {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping processing")
			return nil, ctx.Err()
		default:
		}

		// Process the result
		if res.Err != nil {
			slog.Error("Download failed", "error", res.Err)
			// Add failed task to results so we don't lose it?
			// Or just skip ffmpeg
			continue
		}

		ffmpegJobs <- res
	}
	close(ffmpegJobs)
	wg.Wait()
	close(ffmpegResults)

	// Collect FFmpeg results
	var processedTasks []Task
	for ffmpegRes := range ffmpegResults {
		if ffmpegRes.Err != nil {
			slog.Error("FFmpeg processing failed", "error", ffmpegRes.Err)
			continue
		}
		processedTasks = append(processedTasks, ffmpegRes)
	}

	// Combine reused and processed tasks
	allTasks := append(tasks, processedTasks...)

	if len(allTasks) == 0 {
		slog.Info("Skipping uploads since no audio entries successfully processed")
		return reused, nil
	}
	slog.Info("Processing completed", "processed_files", len(allTasks))

	// Upload processed files to storage backend
	results, err := uploadResults(ctx, storageService, allTasks, p.queue, job.ID)
	if err != nil {
		return nil, err
	}

	// Create and upload RSS XML feed and save state
	feedURL, err := updateFeed(ctx, podcastProcessor, storageService, results)
	if err != nil {
		slog.Error("Failed to update feed", "error", err)
	}

	// Finalize: delete unused files from the storage backend and reconcile the
	// watch playlist with the freshly-published feed. Failures here do not
	// fail the job (the feed is the source of truth; we will retry next run).
	p.finalize(ctx, job, results, storageService, episodeMapping, reused, feedURL, playrunJWT)

	return reused, nil
}

// finalize runs the post-publish bookkeeping: removing unused episodes from the
// storage backend and reconciling the Playrun watch playlist with the freshly
// published feed. Both steps are best-effort; the published feed is the source
// of truth.
func (p *Processor) finalize(ctx context.Context, job *queue.Job, results []podcast.ProcessedEpisode, storageService storage.Storage, episodeMapping map[string]podcast.ExistingEpisode, reused map[string]podcast.ExistingEpisode, feedURL, playrunJWT string) {
	p.deleteUnusedEpisodes(ctx, storageService, episodeMapping, reused)
	p.syncPlayrun(ctx, job, results, storageService, feedURL, playrunJWT)
}

// syncPlayrun reconciles the watch playlist with the freshly published feed via
// the Playrun HTTP API. Per docs/playrun-playlist-api.md:
//  1. Upsert the custom feed (creates the podcast if needed); get its UUID.
//  2. Read the current watch playlist.
//  3. For each just-published episode whose UUID is not in the playlist, subscribe it.
//  4. For each playlist entry that belongs to our podcast but is not in the
//     just-published feed, unsubscribe it (don't touch other podcasts' entries).
//
// JobItem status transitions: items currently in StatusUploaded are moved to
// StatusSynced (success) or StatusSyncFailed (any failure on the
// validate/upsert/playlist-read path applies to all items; per-item Subscribe
// failures are scoped to that item). StatusSkipped items are left untouched,
// though they remain part of the desired playlist so we don't unsubscribe them.
func (p *Processor) syncPlayrun(ctx context.Context, job *queue.Job, results []podcast.ProcessedEpisode, storageService storage.Storage, feedURL, jwt string) {
	if jwt == "" {
		slog.Debug("Playrun not linked, skipping sync", "user_id", job.UserID)
		return
	}
	if feedURL == "" {
		slog.Debug("Feed URL empty (updateFeed likely failed), skipping sync", "user_id", job.UserID)
		return
	}

	client := p.playrunClientFactory(jwt)

	if err := client.Validate(ctx); err != nil {
		slog.Warn("Playrun token invalid, skipping sync", "error", err, "user_id", job.UserID)
		p.markUploadedItems(ctx, job, queue.StatusSyncFailed)
		return
	}

	podUUID, err := client.UpsertPodcast(ctx, feedURL)
	if err != nil {
		slog.Error("Failed to register custom feed with Playrun", "error", err)
		p.markUploadedItems(ctx, job, queue.StatusSyncFailed)
		return
	}

	current, err := client.GetPlaylist(ctx)
	if err != nil {
		slog.Error("Failed to read Playrun playlist", "error", err)
		p.markUploadedItems(ctx, job, queue.StatusSyncFailed)
		return
	}

	currentUUIDs := make(map[string]bool, len(current))
	for _, entry := range current {
		currentUUIDs[entry.UUID] = true
	}

	// Build desired from results (covers both freshly uploaded and reused
	// episodes — both are in the published feed).
	desired := make(map[string]playrun.PlaylistEntry, len(results))
	for _, ep := range results {
		enclosureURL := ep.DownloadURL
		if enclosureURL == "" && ep.DriveFileID != "" {
			enclosureURL = storageService.GenerateDownloadURL(ep.DriveFileID)
		}
		if enclosureURL == "" {
			slog.Warn("Missing enclosure URL; cannot sync episode to Playrun", "title", ep.Title)
			continue
		}
		desired[playrun.EpisodeUUID(enclosureURL)] = playrun.MakeEpisode(ep.Title, enclosureURL, podUUID)
	}

	// Subscribe desired not in current, then mark synced (or syncfailed).
	for uuid, ep := range desired {
		if currentUUIDs[uuid] {
			// Already on the watch; nothing to subscribe, but its JobItem can
			// move to StatusSynced.
			p.setItemStatus(ctx, job, ep.Title, queue.StatusSynced)
			continue
		}
		if err := client.Subscribe(ctx, ep); err != nil {
			slog.Error("Failed to subscribe episode to Playrun", "title", ep.Title, "error", err)
			p.setItemStatus(ctx, job, ep.Title, queue.StatusSyncFailed)
			continue
		}
		p.setItemStatus(ctx, job, ep.Title, queue.StatusSynced)
	}

	// Unsubscribe entries that belong to our podcast but are no longer in the
	// feed. These don't have matching JobItems in the current run, so there's no
	// per-item status to update — failures are just logged.
	for _, entry := range current {
		if entry.Podcast.UUID != podUUID {
			continue
		}
		if _, ok := desired[entry.UUID]; ok {
			continue
		}
		if err := client.Unsubscribe(ctx, entry.UUID, podUUID); err != nil {
			slog.Warn("Failed to unsubscribe stale entry from Playrun", "uuid", entry.UUID, "title", entry.Title, "error", err)
		}
	}
}

// setItemStatus updates the first job item whose Title matches and whose status
// is currently StatusUploaded. Only StatusUploaded items transition through the
// sync phase; StatusSkipped entries are intentionally left alone.
func (p *Processor) setItemStatus(ctx context.Context, job *queue.Job, title string, status queue.JobItemStatus) {
	for i, item := range job.Items {
		if item.Title != title {
			continue
		}
		if item.Status != queue.StatusUploaded {
			return
		}
		item.Status = status
		job.Items[i] = item
		if err := p.queue.UpdateJobItem(ctx, job.ID, item); err != nil {
			slog.Error("Failed to update job item status", "error", err, "title", title)
		}
		return
	}
}

// markUploadedItems transitions every StatusUploaded JobItem to status.
func (p *Processor) markUploadedItems(ctx context.Context, job *queue.Job, status queue.JobItemStatus) {
	for i, item := range job.Items {
		if item.Status != queue.StatusUploaded {
			continue
		}
		item.Status = status
		job.Items[i] = item
		if err := p.queue.UpdateJobItem(ctx, job.ID, item); err != nil {
			slog.Error("Failed to update job item status", "error", err, "title", item.Title)
		}
	}
}
