package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"cobblepod/internal/audio"
	"cobblepod/internal/config"
	"cobblepod/internal/gdrive"
	"cobblepod/internal/podcast"
	"cobblepod/internal/sources"
)

type downloadReq struct {
	Idx int
	URL string
}

// downloadResult represents the outcome of downloading a single URL
type downloadResult struct {
	Idx      int
	TempPath string
	Err      error
}

// ffmpegReq represents a request to process audio with FFmpeg
type ffmpegReq struct {
	Idx      int
	Title    string
	Duration int64
	URL      string
	UUID     string
	TempPath string
	Speed    float64
}

// ffmpegResult represents the result of FFmpeg processing
type ffmpegResult struct {
	Result podcast.ProcessedEpisode
	Err    error
}

// downloadWorker consumes URLs from jobs and emits downloadResult on results; closes results when done.
func downloadWorker(ctx context.Context, proc *audio.Processor, req <-chan downloadReq, res chan<- downloadResult) {
	defer close(res)
	for job := range req {
		// Create temp file for download
		tmp, err := os.CreateTemp("", "audio_*.mp3")
		if err != nil {
			res <- downloadResult{Idx: job.Idx, Err: fmt.Errorf("create temp: %w", err)}
			continue
		}
		tmp.Close()

		// Perform download
		if err := proc.DownloadAudioForEntry(ctx, job.URL, tmp.Name()); err != nil {
			os.Remove(tmp.Name())
			res <- downloadResult{Idx: job.Idx, Err: fmt.Errorf("download: %w", err)}
			continue
		}

		log.Printf("Downloaded %s to %s", job.URL, tmp.Name())
		res <- downloadResult{Idx: job.Idx, TempPath: tmp.Name()}
		log.Printf("Enqueued download result for index %d", job.Idx)
	}
}

// ffmpegWorker processes audio files with FFmpeg
func ffmpegWorker(ctx context.Context, proc *audio.Processor, jobs <-chan ffmpegReq, results chan<- ffmpegResult) {
	fileCount := 0
	for job := range jobs {
		// Create output file for processed audio
		outputFile, err := os.CreateTemp("", "processed_*.mp3")
		if err != nil {
			results <- ffmpegResult{Err: fmt.Errorf("failed to create output temp file: %w", err)}
			continue
		}
		outputFile.Close()

		// Process with FFmpeg
		if err := proc.ProcessWithFFMPEG(ctx, job.TempPath, outputFile.Name(), job.Speed); err != nil {
			os.Remove(job.TempPath)
			os.Remove(outputFile.Name())
			results <- ffmpegResult{Err: fmt.Errorf("ffmpeg failed for %s: %w", job.Title, err)}
			continue
		}

		// Clean up input temp file
		os.Remove(job.TempPath)

		// Create result struct
		newDuration := int64(float64(job.Duration) / job.Speed)
		result := podcast.ProcessedEpisode{
			Title:            job.Title,
			OriginalURL:      job.URL,
			OriginalDuration: job.Duration,
			NewDuration:      newDuration,
			UUID:             job.UUID,
			Speed:            job.Speed,
			TempFile:         outputFile.Name(),
		}

		results <- ffmpegResult{Result: result}
		fileCount++
	}
	log.Printf("FFmpeg worker completed processing %d jobs", fileCount)
}

func main() {
	// Initialize Google Drive service
	gdriveService, err := gdrive.NewService(context.Background())
	if err != nil {
		log.Fatalf("Error setting up Google Drive: %v", err)
	}

	m3u8src := sources.NewM3U8Source(gdriveService)
	podcastAddictBackup := sources.NewPodcastAddictBackup(gdriveService)

	processor := audio.NewProcessor()
	podcastProcessor := podcast.NewRSSProcessor("Playrun Addict Custom Feed", gdriveService)

	// Get RSS feed and extract episode mapping
	rssFileID := podcastProcessor.GetRSSFeedID()
	// todo use a struct (and use int64 so conversions are easier)
	var episodeMapping map[string]map[string]interface{}
	if rssFileID != "" {
		rssContent, err := gdriveService.DownloadFile(rssFileID)
		if err != nil {
			log.Printf("Error downloading RSS feed: %v", err)
			episodeMapping = make(map[string]map[string]interface{})
		} else {
			episodeMapping, err = podcastProcessor.ExtractEpisodeMapping(rssContent)
			if err != nil {
				log.Printf("Error extracting episode mapping: %v", err)
				episodeMapping = make(map[string]map[string]interface{})
			}
		}
	} else {
		episodeMapping = make(map[string]map[string]interface{})
	}

	podcastAddictBackup.AddListeningProgress(context.Background(), episodeMapping)

	// Discover new M3U8 (parse only)
	fileID, fileName, entries, err := m3u8src.CheckForNewM3U8Files(context.Background())
	if err != nil {
		log.Fatalf("Error checking M3U8 files: %v", err)
	}
	if fileID == "" || len(entries) == 0 {
		log.Println("No new M3U8 entries to process")
		return
	}
	log.Printf("Processing %d entries from %s", len(entries), fileName)

	// Process entries locally (moved from processor)
	var results []podcast.ProcessedEpisode

	// Start a single downloader worker with separate job and result channels
	dlRequests := make(chan downloadReq, len(entries))
	dlResults := make(chan downloadResult, len(entries))
	go downloadWorker(context.Background(), processor, dlRequests, dlResults)

	speed := config.DefaultSpeed

	// First pass: reuse check; enqueue downloads for the rest
	for i, entry := range entries {
		title := entry.Title
		duration := entry.Duration
		expectedNewDuration := int64(float64(duration) / speed)

		// Reuse check
		if oldEp, exists := episodeMapping[title]; exists {
			if origDur, ok := oldEp["original_duration"]; ok {
				if length, ok := oldEp["length"]; ok {
					origDurInt, _ := origDur.(int)
					lengthInt, _ := length.(int)
					if int64(origDurInt) == duration && int64(lengthInt) == expectedNewDuration {
						log.Printf("Reusing existing processed file: %s", title)
						result := podcast.ProcessedEpisode{
							Title:            title,
							OriginalDuration: duration,
							NewDuration:      expectedNewDuration,
							UUID:             entry.UUID,
							Speed:            speed,
							DownloadURL:      oldEp["download_url"].(string),
						}
						if guid, exists := oldEp["original_guid"]; exists {
							result.OriginalGUID = guid.(string)
						}
						results = append(results, result)
						continue
					}
				}
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
			ffmpegWorker(context.Background(), processor, ffmpegJobs, ffmpegResults)
		}()
	}

	for res := range dlResults {
		// Process the result
		if res.Err != nil {
			log.Printf("Download failed: %v", res.Err)
			continue
		}

		i := res.Idx
		title := entries[i].Title
		duration := entries[i].Duration
		url := entries[i].URL
		id := entries[i].UUID

		// Send to FFmpeg worker
		ffmpegJobs <- ffmpegReq{
			Idx:      i,
			Title:    title,
			Duration: duration,
			URL:      url,
			UUID:     id,
			TempPath: res.TempPath,
			Speed:    speed,
		}
	}
	close(ffmpegJobs)
	wg.Wait()
	close(ffmpegResults)
	// Collect FFmpeg results
	for ffmpegRes := range ffmpegResults {
		if ffmpegRes.Err != nil {
			log.Printf("FFmpeg processing failed: %v", ffmpegRes.Err)
			continue
		}
		results = append(results, ffmpegRes.Result)
	}

	if len(results) == 0 {
		log.Println("No audio entries processed successfully")
		return
	}
	log.Printf("Processed %d audio files", len(results))

	// Upload processed files to Google Drive
	for i, result := range results {
		// Skip upload for reused files that already have download_url
		if downloadURL := result.DownloadURL; downloadURL != "" {
			log.Printf("Skipping upload for reused file: %s", result.Title)
			// Extract drive_file_id from download_url for consistency
			if driveFileID := gdriveService.ExtractFileIDFromURL(downloadURL); driveFileID != "" {
				result.DriveFileID = driveFileID
			}
			continue
		}

		log.Printf("Uploading %s to Google Drive", result.Title)
		tempFile := result.TempFile
		filename := fmt.Sprintf("%s.mp3", result.Title)

		driveFileID, err := gdriveService.UploadFile(tempFile, filename, "audio/mpeg")
		if err != nil {
			log.Fatalf("Failed to upload %s to Google Drive: %v", result.Title, err)
		}

		// Clean up temp file
		if err := os.Remove(tempFile); err != nil {
			log.Printf("Warning: failed to remove temp file %s: %v", tempFile, err)
		}

		results[i].DriveFileID = driveFileID
	}

	// Create and upload RSS XML
	xmlFeed := podcastProcessor.CreateRSSXML(results)
	rssFileID, err = gdriveService.UploadString(xmlFeed, "playrun_addict.xml", "application/rss+xml", rssFileID)
	if err != nil {
		log.Fatalf("Failed to upload RSS feed: %v", err)
	}

	rssDownloadURL := gdriveService.GenerateDownloadURL(rssFileID)
	fmt.Printf("RSS Feed Download URL: %s\n", rssDownloadURL)
}
