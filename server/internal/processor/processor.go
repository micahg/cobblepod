package processor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"cobblepod/internal/audio"
	"cobblepod/internal/config"
	"cobblepod/internal/podcast"
	"cobblepod/internal/sources"
	"cobblepod/internal/state"
	"cobblepod/internal/storage"
)

// downloadReq represents a download request
type downloadReq struct {
	Idx int
	URL string
}

// downloadResult represents the result of a download
type downloadResult struct {
	Idx      int
	TempPath string
	Err      error
}

// ffmpegReq represents an FFmpeg processing request
type ffmpegReq struct {
	Idx      int
	Title    string
	Duration time.Duration
	URL      string
	UUID     string
	TempPath string
	Speed    float64
	Offset   time.Duration
}

// ffmpegResult represents the result of FFmpeg processing
type ffmpegResult struct {
	Result podcast.ProcessedEpisode
	Err    error
}

// GDriveDeleter interface for dependency injection
type GDriveDeleter interface {
	ExtractFileIDFromURL(url string) string
	DeleteFile(fileID string) error
}

// Processor handles the main processing logic
type Processor struct {
	storage *storage.GDrive
	state   *state.CobblepodStateManager
}

// NewProcessor creates a new processor with default dependencies
func NewProcessor(ctx context.Context) (*Processor, error) {
	storage, err := storage.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("error setting up Google Drive: %w", err)
	}

	state, err := state.NewStateManager(ctx)
	if err != nil {
		slog.Error("Failed to connect to state", "error", err)
		// Continue with nil state manager - we'll handle this in Run()
	}

	return &Processor{
		storage: storage,
		state:   state,
	}, nil
}

// NewProcessorWithDependencies creates a new processor with injected dependencies for testing
func NewProcessorWithDependencies(
	storage *storage.GDrive,
	state *state.CobblepodStateManager,
) *Processor {
	return &Processor{
		storage: storage,
		state:   state,
	}
}

// Run executes the main processing logic
func (p *Processor) Run(ctx context.Context) error {
	// Use the stored Google Drive servic

	m3u8src := sources.NewM3U8Source(p.storage)
	podcastAddictBackup := sources.NewPodcastAddictBackup(p.storage)

	audioProcessor := audio.NewProcessor()
	podcastProcessor := podcast.NewRSSProcessor("Playrun Addict Custom Feed", p.storage)

	// Use the stored state manager
	stateManager := p.state
	var appState *state.CobblepodState

	if stateManager != nil {
		var err error
		appState, err = stateManager.GetState()
		if err != nil {
			slog.Error("Failed to get state", "error", err)
			slog.Info("Assuming first run")
			appState = &state.CobblepodState{}
		} else {
			slog.Debug("State loaded", "last_run", appState.LastRun.Format(time.RFC3339))
		}
	} else {
		slog.Info("State manager not available, assuming first run")
		appState = &state.CobblepodState{}
	}

	// Get RSS feed and extract episode mapping
	rssFileID := podcastProcessor.GetRSSFeedID()
	episodeMapping := make(map[string]podcast.ExistingEpisode)
	if rssFileID != "" {
		rssContent, err := p.storage.DownloadFile(rssFileID)
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
			if err := stateManager.SaveState(&state.CobblepodState{LastRun: startTime}); err != nil {
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
	var entries []sources.AudioEntry
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

	reused, err := p.processEntries(ctx, entries, episodeMapping, p.storage, audioProcessor, podcastProcessor)
	if err != nil {
		return err
	}

	// Delete unused episodes from Google Drive
	p.deleteUnusedEpisodes(p.storage, episodeMapping, reused)

	return nil
}

// downloadWorker handles download requests
func downloadWorker(ctx context.Context, processor *audio.Processor, requests <-chan downloadReq, results chan<- downloadResult) {
	defer close(results)
	for req := range requests {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			results <- downloadResult{Idx: req.Idx, Err: ctx.Err()}
			return
		default:
		}

		tempPath, err := processor.DownloadFile(req.URL)
		results <- downloadResult{
			Idx:      req.Idx,
			TempPath: tempPath,
			Err:      err,
		}
	}
}

// ffmpegWorker handles FFmpeg processing requests
func ffmpegWorker(ctx context.Context, processor *audio.Processor, jobs <-chan ffmpegReq, results chan<- ffmpegResult) {
	fileCount := 0
	defer func() {
		slog.Info("FFmpeg worker completed", "processed_files", fileCount)
	}()

	for job := range jobs {
		fileCount++
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			results <- ffmpegResult{Err: ctx.Err()}
			return
		default:
		}

		slog.Info("Processing audio", "title", job.Title, "speed", job.Speed)
		outputPath, err := processor.ProcessAudio(job.TempPath, job.Speed, job.Offset)
		if err != nil {
			slog.Error("Error processing audio", "title", job.Title, "error", err)
			results <- ffmpegResult{Err: err}
			// Clean up temp file
			if cleanupErr := os.Remove(job.TempPath); cleanupErr != nil {
				slog.Warn("Failed to remove temp file", "path", job.TempPath, "error", cleanupErr)
			}
			continue
		}

		// Clean up input temp file
		if err := os.Remove(job.TempPath); err != nil {
			slog.Warn("Failed to remove temp file", "path", job.TempPath, "error", err)
		}

		newDuration := int64(float64(job.Duration.Milliseconds()) / job.Speed)
		result := podcast.ProcessedEpisode{
			Title:            job.Title,
			OriginalDuration: job.Duration.Milliseconds(),
			NewDuration:      newDuration,
			UUID:             job.UUID,
			Speed:            job.Speed,
			TempFile:         outputPath,
		}

		results <- ffmpegResult{Result: result, Err: nil}
	}
}

// uploadResults handles uploading processed audio files to Google Drive
func uploadResults(ctx context.Context, gdriveService *storage.GDrive, results []podcast.ProcessedEpisode) error {
	for i, result := range results {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping upload")
			return ctx.Err()
		default:
		}

		// Skip upload for reused files that already have download_url
		if downloadURL := result.DownloadURL; downloadURL != "" {
			slog.Info("Skipping upload for reused file", "title", result.Title)
			// Extract drive_file_id from download_url for consistency
			if driveFileID := gdriveService.ExtractFileIDFromURL(downloadURL); driveFileID != "" {
				results[i].DriveFileID = driveFileID
			}
			continue
		}

		slog.Info("Uploading to Google Drive", "title", result.Title)
		tempFile := result.TempFile
		filename := fmt.Sprintf("%s.mp3", result.Title)

		driveFileID, err := gdriveService.UploadFile(tempFile, filename, "audio/mpeg")
		if err != nil {
			return fmt.Errorf("failed to upload %s to Google Drive: %w", result.Title, err)
		}

		// Clean up temp file
		if err := os.Remove(tempFile); err != nil {
			slog.Warn("Failed to remove temp file", "path", tempFile, "error", err)
		}

		results[i].DriveFileID = driveFileID
	}

	return nil
}

// updateFeed creates and uploads the RSS XML feed and saves the application state
func updateFeed(podcastProcessor *podcast.RSSProcessor, gdriveService *storage.GDrive, results []podcast.ProcessedEpisode) error {
	// Create and upload RSS XML
	xmlFeed := podcastProcessor.CreateRSSXML(results)
	rssFileID, err := gdriveService.UploadString(xmlFeed, "playrun_addict.xml", "application/rss+xml", podcastProcessor.GetRSSFeedID())
	if err != nil {
		return fmt.Errorf("failed to upload RSS feed: %w", err)
	}

	rssDownloadURL := gdriveService.GenerateDownloadURL(rssFileID)
	slog.Info("RSS Feed created", "download_url", rssDownloadURL)

	return nil
}

// deleteUnusedEpisodes removes episodes from Google Drive that are no longer in the current playlist
func (p *Processor) deleteUnusedEpisodes(gdriveService GDriveDeleter, episodeMapping map[string]podcast.ExistingEpisode, reused map[string]podcast.ExistingEpisode) {
	// Delete episodes that are not reused
	for title, episode := range episodeMapping {
		if _, ok := reused[title]; ok {
			continue
		}
		driveId := gdriveService.ExtractFileIDFromURL(episode.DownloadURL)
		if driveId == "" {
			slog.Warn("Could not extract Drive file ID from URL", "url", episode.DownloadURL)
			continue
		}
		slog.Info("Deleting unused episode from Google Drive", "title", title, "drive_id", driveId)
		if err := gdriveService.DeleteFile(driveId); err != nil {
			slog.Error("Failed to delete file from Google Drive", "drive_id", driveId, "error", err)
		}
	}
}

// processEntries returns the reused episodes
func (p *Processor) processEntries(ctx context.Context, entries []sources.AudioEntry, episodeMapping map[string]podcast.ExistingEpisode, gdriveService *storage.GDrive, audioProcessor *audio.Processor, podcastProcessor *podcast.RSSProcessor) (map[string]podcast.ExistingEpisode, error) {
	// Process entries locally
	var results []podcast.ProcessedEpisode

	// Start a single downloader worker with separate job and result channels
	dlRequests := make(chan downloadReq, len(entries))
	dlResults := make(chan downloadResult, len(entries))
	go downloadWorker(ctx, audioProcessor, dlRequests, dlResults)

	speed := config.DefaultSpeed

	reused := make(map[string]podcast.ExistingEpisode)
	// First pass: reuse check; enqueue downloads for the rest
	for i, entry := range entries {
		title := entry.Title
		duration := entry.Duration
		// i don't think this is right either anymore - we're using duration (ns) now, no milliseconds
		expectedNewDuration := int64(float64(duration.Milliseconds()) / speed)

		// Reuse check
		if oldEp, exists := episodeMapping[title]; exists {
			if podcastProcessor.CanReuseEpisode(oldEp, duration.Milliseconds(), expectedNewDuration) {
				slog.Info("Reusing existing processed file", "title", title)
				reused[title] = oldEp
				result := podcast.ProcessedEpisode{
					Title:            title,
					OriginalDuration: duration.Milliseconds(),
					NewDuration:      expectedNewDuration,
					UUID:             entry.UUID,
					Speed:            speed,
					DownloadURL:      oldEp.DownloadURL,
					OriginalGUID:     oldEp.OriginalGUID,
				}
				results = append(results, result)
				continue
			}
		}

		// Send request and wait for response
		slog.Info("Enqueuing download", "title", title, "url", entry.URL)
		dlRequests <- downloadReq{Idx: i, URL: entry.URL}
	}
	// all done sending jobs
	close(dlRequests)

	// Start FFmpeg worker
	var wg sync.WaitGroup
	ffmpegJobs := make(chan ffmpegReq, len(entries))
	ffmpegResults := make(chan ffmpegResult, len(entries))
	for i := 0; i < config.MaxFFMPEGWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ffmpegWorker(ctx, audioProcessor, ffmpegJobs, ffmpegResults)
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
			continue
		}

		i := res.Idx
		ffmpegJobs <- ffmpegReq{
			Idx:      i,
			Title:    entries[i].Title,
			Duration: entries[i].Duration,
			URL:      entries[i].URL,
			UUID:     entries[i].UUID,
			TempPath: res.TempPath,
			Speed:    speed,
			Offset:   entries[i].Offset,
		}
	}
	close(ffmpegJobs)
	wg.Wait()
	close(ffmpegResults)

	// Collect FFmpeg results
	var newResults []podcast.ProcessedEpisode
	for ffmpegRes := range ffmpegResults {
		if ffmpegRes.Err != nil {
			slog.Error("FFmpeg processing failed", "error", ffmpegRes.Err)
			continue
		}
		newResults = append(newResults, ffmpegRes.Result)
	}

	if len(newResults) == 0 {
		slog.Info("Skipping uploads since no audio entries successfully processed")
		return reused, nil
	}
	results = append(results, newResults...)
	slog.Info("Processing completed", "processed_files", len(results))

	// Upload processed files to Google Drive
	if err := uploadResults(ctx, p.storage, results); err != nil {
		return nil, err
	}

	// Create and upload RSS XML feed and save state
	if err := updateFeed(podcastProcessor, p.storage, results); err != nil {
		slog.Error("Failed to update feed", "error", err)
	}

	return reused, nil
}
