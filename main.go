package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"cobblepod/internal/audio"
	"cobblepod/internal/config"
	"cobblepod/internal/gdrive"
	"cobblepod/internal/podcast"
	"cobblepod/internal/sources"
	"cobblepod/internal/state"
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
	Duration int64
	URL      string
	UUID     string
	TempPath string
	Speed    float64
	Offset   int64
}

// ffmpegResult represents the result of FFmpeg processing
type ffmpegResult struct {
	Result podcast.ProcessedEpisode
	Err    error
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
		log.Printf("FFmpeg worker completed processing %d jobs", fileCount)
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

		log.Printf("Processing audio for %s (%.1fx speed)", job.Title, job.Speed)
		outputPath, err := processor.ProcessAudio(job.TempPath, job.Speed, job.Offset)
		if err != nil {
			log.Printf("Error processing audio for %s: %v", job.Title, err)
			results <- ffmpegResult{Err: err}
			// Clean up temp file
			if cleanupErr := os.Remove(job.TempPath); cleanupErr != nil {
				log.Printf("Warning: failed to remove temp file %s: %v", job.TempPath, cleanupErr)
			}
			continue
		}

		// Clean up input temp file
		if err := os.Remove(job.TempPath); err != nil {
			log.Printf("Warning: failed to remove temp file %s: %v", job.TempPath, err)
		}

		newDuration := int64(float64(job.Duration) / job.Speed)
		result := podcast.ProcessedEpisode{
			Title:            job.Title,
			OriginalDuration: job.Duration,
			NewDuration:      newDuration,
			UUID:             job.UUID,
			Speed:            job.Speed,
			TempFile:         outputPath,
		}

		results <- ffmpegResult{Result: result, Err: nil}
	}
}

// cobbleWorker handles processing job requests
func cobbleWorker(ctx context.Context, processingJobs <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Cobble worker shutting down...")
			return
		case _, ok := <-processingJobs:
			if !ok {
				// Channel closed
				log.Println("Processing jobs channel closed, worker exiting")
				return
			}

			if err := processRun(ctx); err != nil {
				if err == context.Canceled {
					log.Println("Processing cancelled")
					return
				} else {
					log.Printf("Error during processing: %v", err)
				}
			}
		}
	}
}

// uploadResults handles uploading processed audio files to Google Drive
func uploadResults(ctx context.Context, gdriveService *gdrive.Service, results []podcast.ProcessedEpisode) error {
	for i, result := range results {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping upload")
			return ctx.Err()
		default:
		}

		// Skip upload for reused files that already have download_url
		if downloadURL := result.DownloadURL; downloadURL != "" {
			log.Printf("Skipping upload for reused file: %s", result.Title)
			// Extract drive_file_id from download_url for consistency
			if driveFileID := gdriveService.ExtractFileIDFromURL(downloadURL); driveFileID != "" {
				results[i].DriveFileID = driveFileID
			}
			continue
		}

		log.Printf("Uploading %s to Google Drive", result.Title)
		tempFile := result.TempFile
		filename := fmt.Sprintf("%s.mp3", result.Title)

		driveFileID, err := gdriveService.UploadFile(tempFile, filename, "audio/mpeg")
		if err != nil {
			return fmt.Errorf("failed to upload %s to Google Drive: %w", result.Title, err)
		}

		// Clean up temp file
		if err := os.Remove(tempFile); err != nil {
			log.Printf("Warning: failed to remove temp file %s: %v", tempFile, err)
		}

		results[i].DriveFileID = driveFileID
	}

	return nil
}

// updateFeed creates and uploads the RSS XML feed and saves the application state
func updateFeed(podcastProcessor *podcast.RSSProcessor, gdriveService *gdrive.Service, results []podcast.ProcessedEpisode) error {
	// Create and upload RSS XML
	xmlFeed := podcastProcessor.CreateRSSXML(results)
	rssFileID, err := gdriveService.UploadString(xmlFeed, "playrun_addict.xml", "application/rss+xml", podcastProcessor.GetRSSFeedID())
	if err != nil {
		return fmt.Errorf("failed to upload RSS feed: %w", err)
	}

	rssDownloadURL := gdriveService.GenerateDownloadURL(rssFileID)
	log.Printf("RSS Feed Download URL: %s", rssDownloadURL)

	return nil
}

func processRun(ctx context.Context) error {
	// Initialize Google Drive service
	gdriveService, err := gdrive.NewService(ctx)
	if err != nil {
		return fmt.Errorf("error setting up Google Drive: %w", err)
	}

	m3u8src := sources.NewM3U8Source(gdriveService)
	podcastAddictBackup := sources.NewPodcastAddictBackup(gdriveService)

	processor := audio.NewProcessor()
	podcastProcessor := podcast.NewRSSProcessor("Playrun Addict Custom Feed", gdriveService)

	stateManager, err := state.NewStateManager(ctx)
	if err != nil {
		log.Printf("Failed to connect to state: %v", err)
	}

	appState, err := stateManager.GetState()
	if err != nil {
		log.Printf("Failed to get state: %v", err)
		log.Printf("Assuming first run")
		appState = &state.CobblepodState{}
	} else {
		log.Printf("Last run was at: %s", appState.LastRun.Format(time.RFC3339))
	}

	// Get RSS feed and extract episode mapping
	rssFileID := podcastProcessor.GetRSSFeedID()
	episodeMapping := make(map[string]podcast.ExistingEpisode)
	if rssFileID != "" {
		rssContent, err := gdriveService.DownloadFile(rssFileID)
		if err != nil {
			log.Printf("Error downloading RSS feed: %v", err)
		} else {
			episodeMapping, err = podcastProcessor.ExtractEpisodeMapping(rssContent)
			if err != nil {
				log.Printf("Error extracting episode mapping: %v", err)
			}
		}
	}

	startTime := time.Now()
	defer func() {
		if err := stateManager.SaveState(&state.CobblepodState{LastRun: startTime}); err != nil {
			log.Printf("Failed to save state: %v", err)
		}
	}()

	// Check for new M3U8 file
	m3u8File, err := m3u8src.GetLatestM3U8File(ctx)
	if err != nil {
		return fmt.Errorf("error getting latest M3U8 file: %w", err)
	}

	newM3U8 := false
	if m3u8File != nil && (appState.LastRun.IsZero() || m3u8File.ModifiedTime.After(appState.LastRun)) {
		newM3U8 = true
	}

	// Check for new backup file
	backupFile, err := podcastAddictBackup.GetLatestBackupFile(ctx)
	if err != nil {
		log.Printf("Error getting latest backup file: %v", err)
	}

	newBackup := false
	if backupFile != nil && (appState.LastRun.IsZero() || backupFile.ModifiedTime.After(appState.LastRun)) {
		newBackup = true
	}

	// Determine processing mode
	if newM3U8 {
		log.Printf("Processing M3U8 file: %s (modified: %s)", m3u8File.File.Name, m3u8File.ModifiedTime.Format(time.RFC3339))

		// Process M3U8 as before, including backup for offsets
		podcastAddictBackup.AddListeningProgress(ctx, episodeMapping)

		entries, err := m3u8src.Process(ctx, m3u8File)
		if err != nil {
			return fmt.Errorf("error processing M3U8 file: %w", err)
		}
		if len(entries) == 0 {
			log.Println("No entries found in M3U8 file")
			return nil
		}

		return processEntries(ctx, entries, episodeMapping, gdriveService, processor, podcastProcessor)

	} else if newBackup {
		log.Printf("Processing backup independently: %s (modified: %s)", backupFile.FileName, backupFile.ModifiedTime.Format(time.RFC3339))

		// Process backup independently
		newResults, err := podcastAddictBackup.Process(ctx, backupFile)
		if err != nil {
			return fmt.Errorf("error processing backup independently: %w", err)
		}
		if len(newResults) == 0 {
			log.Println("No episodes found in backup")
			return nil
		}

		// Convert to AudioEntry format for processing
		var entries []sources.AudioEntry
		for _, result := range newResults {
			entry := sources.AudioEntry{
				Title:    result.Title,
				Duration: result.OriginalDuration,
				URL:      "", // Will need to be filled from backup data
				UUID:     result.UUID,
			}
			entries = append(entries, entry)
		}

		return processEntries(ctx, entries, episodeMapping, gdriveService, processor, podcastProcessor)

	} else {
		log.Println("No new M3U8 or backup files found since last run")
		return nil
	}
}

func processEntries(ctx context.Context, entries []sources.AudioEntry, episodeMapping map[string]podcast.ExistingEpisode, gdriveService *gdrive.Service, audioProcessor *audio.Processor, podcastProcessor *podcast.RSSProcessor) error {
	// Process entries locally
	var results []podcast.ProcessedEpisode

	// Start a single downloader worker with separate job and result channels
	dlRequests := make(chan downloadReq, len(entries))
	dlResults := make(chan downloadResult, len(entries))
	go downloadWorker(ctx, audioProcessor, dlRequests, dlResults)

	speed := config.DefaultSpeed

	// First pass: reuse check; enqueue downloads for the rest
	for i, entry := range entries {
		title := entry.Title
		duration := entry.Duration
		expectedNewDuration := int64(float64(duration) / speed)

		// Reuse check
		if oldEp, exists := episodeMapping[title]; exists {
			if podcastProcessor.CanReuseEpisode(oldEp, duration, expectedNewDuration) {
				log.Printf("Reusing existing processed file: %s", title)
				result := podcast.ProcessedEpisode{
					Title:            title,
					OriginalDuration: duration,
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
		log.Printf("Enqueuing download for %s (%s)", title, entry.URL)
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
			log.Printf("Context cancelled, stopping processing")
			return ctx.Err()
		default:
		}

		// Process the result
		if res.Err != nil {
			log.Printf("Download failed: %v", res.Err)
			continue
		}

		i := res.Idx
		req := ffmpegReq{
			Idx:      i,
			Title:    entries[i].Title,
			Duration: entries[i].Duration,
			URL:      entries[i].URL,
			UUID:     entries[i].UUID,
			TempPath: res.TempPath,
			Speed:    speed,
			Offset:   0,
		}

		// update the offset if we have one
		if ep, ok := episodeMapping[req.Title]; ok {
			req.Offset = ep.Offset
		}

		// Send to FFmpeg worker
		ffmpegJobs <- req
	}
	close(ffmpegJobs)
	wg.Wait()
	close(ffmpegResults)

	// Collect FFmpeg results
	var newResults []podcast.ProcessedEpisode
	for ffmpegRes := range ffmpegResults {
		if ffmpegRes.Err != nil {
			log.Printf("FFmpeg processing failed: %v", ffmpegRes.Err)
			continue
		}
		newResults = append(newResults, ffmpegRes.Result)
	}

	if len(newResults) == 0 {
		log.Println("Skipping uploads since no audio entries successfully processed")
		return nil
	}
	results = append(results, newResults...)
	log.Printf("Processed %d audio files", len(results))

	// Upload processed files to Google Drive
	if err := uploadResults(ctx, gdriveService, results); err != nil {
		return err
	}

	// Create and upload RSS XML feed and save state
	if err := updateFeed(podcastProcessor, gdriveService, results); err != nil {
		log.Printf("Failed to update feed: %v", err)
	}

	return nil
}

func main() {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a ticker for the polling interval
	ticker := time.NewTicker(time.Duration(config.PollInterval) * time.Second)
	defer ticker.Stop()

	// Channel for processing jobs - buffered to allow one pending job
	processingJobs := make(chan struct{})

	// Start the processing worker
	go cobbleWorker(ctx, processingJobs)

	log.Printf("Starting cobblepod with %d second polling interval", config.PollInterval)
	processingJobs <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down...")
			return
		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down gracefully...", sig)
			cancel()
			return
		case <-ticker.C:
			select {
			case processingJobs <- struct{}{}:
			default:
				log.Printf("Skipping processing - queue is full")
			}
		}
	}
}
