package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

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
	var results []map[string]interface{}

	// Start a single downloader worker with separate job and result channels
	dlRequests := make(chan downloadReq, len(entries))
	dlResults := make(chan downloadResult, len(entries))
	go downloadWorker(context.Background(), processor, dlRequests, dlResults)

	speed := config.DefaultSpeed

	// First pass: reuse check; enqueue downloads for the rest
	for i, entry := range entries {
		title := entry.Title
		duration := entry.Duration
		expectedNewDuration := int(duration / speed)

		// Reuse check
		if oldEp, exists := episodeMapping[title]; exists {
			if origDur, ok := oldEp["original_duration"]; ok {
				if length, ok := oldEp["length"]; ok {
					origDurInt, _ := strconv.Atoi(fmt.Sprintf("%.0f", origDur))
					lengthInt, _ := strconv.Atoi(fmt.Sprintf("%.0f", length))
					if origDurInt == int(duration) && lengthInt == expectedNewDuration {
						log.Printf("Reusing existing processed file: %s", title)
						result := map[string]interface{}{
							"title":             title,
							"original_duration": int(duration),
							"new_duration":      expectedNewDuration,
							"uuid":              entry.UUID,
							"speed":             speed,
							"download_url":      oldEp["download_url"],
						}
						if guid, exists := oldEp["original_guid"]; exists {
							result["original_guid"] = guid
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

		outputFile := res.TempPath

		if err := processor.ProcessWithFFMPEG(context.Background(), res.TempPath, outputFile, speed); err != nil {
			os.Remove(res.TempPath)
			os.Remove(outputFile)
			log.Printf("ffmpeg failed %s: %v", title, err)
			continue
		}
		os.Remove(res.TempPath)
		newDuration := int(duration / speed)
		results = append(results, map[string]interface{}{
			"title":             title,
			"original_url":      url,
			"original_duration": duration,
			"new_duration":      newDuration,
			"uuid":              id,
			"speed":             speed,
			"temp_file":         res.TempPath,
		})
	}

	if len(results) == 0 {
		log.Println("No audio entries processed successfully")
		return
	}
	log.Printf("Processed %d audio files", len(results))

	// Upload processed files to Google Drive
	for i, result := range results {
		// Skip upload for reused files that already have download_url
		if downloadURL, exists := result["download_url"]; exists && downloadURL != "" {
			log.Printf("Skipping upload for reused file: %s", result["title"])
			// Extract drive_file_id from download_url for consistency
			if driveFileID := gdriveService.ExtractFileIDFromURL(downloadURL.(string)); driveFileID != "" {
				result["drive_file_id"] = driveFileID
			}
			continue
		}

		log.Printf("Uploading %s to Google Drive", result["title"])
		tempFile := result["temp_file"].(string)
		filename := fmt.Sprintf("%s.mp3", result["title"])

		driveFileID, err := gdriveService.UploadFile(tempFile, filename, "audio/mpeg")
		if err != nil {
			log.Fatalf("Failed to upload %s to Google Drive: %v", result["title"], err)
		}

		// Clean up temp file
		if err := os.Remove(tempFile); err != nil {
			log.Printf("Warning: failed to remove temp file %s: %v", tempFile, err)
		}

		results[i]["drive_file_id"] = driveFileID
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
