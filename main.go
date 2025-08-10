package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cobblepod/internal/audio"
	"cobblepod/internal/gdrive"
	"cobblepod/internal/podcast"
)

func main() {
	// Initialize Google Drive service
	gdriveService, err := gdrive.NewService(context.Background())
	if err != nil {
		log.Fatalf("Error setting up Google Drive: %v", err)
	}

	processor := audio.NewProcessor()
	podcastProcessor := podcast.NewRSSProcessor("Playrun Addict Custom Feed")

	// Get RSS feed and extract episode mapping
	rssFileID := podcastProcessor.GetRSSFeedID(gdriveService)
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

	// Process M3U8 files
	results, err := processor.CheckForNewM3U8Files(context.Background(), gdriveService, episodeMapping)
	if err != nil {
		log.Fatalf("Error processing M3U8 files: %v", err)
	}

	if len(results) == 0 {
		log.Println("M3U8 resulted in no files")
		return
	}

	log.Printf("Processed %d audio files", len(results))

	// Upload processed files to Google Drive
	for i, result := range results {
		// Skip upload for reused files that already have download_url
		if downloadURL, exists := result["download_url"]; exists && downloadURL != "" {
			log.Printf("Skipping upload for reused file: %s", result["title"])
			// Extract drive_file_id from download_url for consistency
			if driveFileID := gdrive.ExtractFileIDFromURL(downloadURL.(string)); driveFileID != "" {
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

	rssDownloadURL := gdrive.GenerateDownloadURL(rssFileID)
	fmt.Printf("RSS Feed Download URL: %s\n", rssDownloadURL)
}
