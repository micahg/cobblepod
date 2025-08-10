package audio

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"cobblepod/internal/config"
	"cobblepod/internal/gdrive"

	"github.com/google/uuid"
)

// ProcessingJob represents a single audio processing job
type ProcessingJob struct {
	ID             string                   `json:"id"`
	Status         string                   `json:"status"`
	M3U8FileID     string                   `json:"m3u8_file_id"`
	M3U8FileName   string                   `json:"m3u8_file_name"`
	Speed          float64                  `json:"speed"`
	CreatedAt      time.Time                `json:"created_at"`
	CompletedAt    *time.Time               `json:"completed_at,omitempty"`
	Error          string                   `json:"error,omitempty"`
	ProcessedFiles []map[string]interface{} `json:"processed_files"`
}

// AudioEntry represents an entry in an M3U8 playlist
type AudioEntry struct {
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	URL      string  `json:"url"`
	UUID     string  `json:"uuid"`
}

// Processor handles audio processing operations
type Processor struct {
	jobs           map[string]*ProcessingJob
	processedFiles map[string]bool
	mutex          sync.RWMutex
}

// NewProcessor creates a new audio processor
func NewProcessor() *Processor {
	return &Processor{
		jobs:           make(map[string]*ProcessingJob),
		processedFiles: make(map[string]bool),
	}
}

// CheckForNewM3U8Files checks for new M3U8 files and processes them
func (p *Processor) CheckForNewM3U8Files(ctx context.Context, driveService *gdrive.Service, oldEpisodes map[string]map[string]interface{}) ([]map[string]interface{}, error) {
	files, err := driveService.GetFiles(config.M3UQuery, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get M3U8 files: %w", err)
	}

	if len(files) == 0 {
		log.Println("No M3U8 files found")
		return nil, nil
	}

	mostRecentFile := gdrive.GetMostRecentFile(files)
	if mostRecentFile == nil {
		log.Println("No recent M3U8 files found")
		return nil, nil
	}

	fileID := mostRecentFile.Id
	fileName := mostRecentFile.Name

	p.mutex.Lock()
	if p.processedFiles[fileID] {
		p.mutex.Unlock()
		log.Printf("Most recent M3U8 file '%s' already processed", fileName)
		return nil, nil
	}
	p.mutex.Unlock()

	log.Printf("Found new M3U8 file: %s", fileName)

	jobID := uuid.New().String()
	job := &ProcessingJob{
		ID:           jobID,
		Status:       "pending",
		M3U8FileID:   fileID,
		M3U8FileName: fileName,
		Speed:        config.DefaultSpeed,
		CreatedAt:    time.Now().UTC(),
	}

	p.mutex.Lock()
	p.jobs[jobID] = job
	p.processedFiles[fileID] = true
	p.mutex.Unlock()

	results, err := p.processM3U8File(ctx, driveService, jobID, oldEpisodes)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// processM3U8File processes a single M3U8 file
func (p *Processor) processM3U8File(ctx context.Context, driveService *gdrive.Service, jobID string, oldEpisodes map[string]map[string]interface{}) ([]map[string]interface{}, error) {
	p.mutex.Lock()
	job := p.jobs[jobID]
	p.mutex.Unlock()

	if job == nil {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	job.Status = "processing"
	log.Printf("Processing job %s: %s", jobID, job.M3U8FileName)

	// Download M3U8 content
	m3u8Content, err := driveService.DownloadFile(job.M3U8FileID)
	if err != nil {
		job.Status = "failed"
		job.Error = fmt.Sprintf("failed to download M3U8 file: %v", err)
		return nil, fmt.Errorf("failed to download M3U8 file: %w", err)
	}

	// Parse M3U8 content
	audioEntries := p.parseM3U8(m3u8Content)
	if len(audioEntries) == 0 {
		job.Status = "failed"
		job.Error = "no audio files found in M3U8 playlist"
		return nil, fmt.Errorf("no audio files found in M3U8 playlist")
	}

	log.Printf("Found %d audio files to process", len(audioEntries))

	// Process entries
	var results []map[string]interface{}
	for _, entry := range audioEntries {
		result, err := p.processAudioEntry(ctx, entry, job.Speed, oldEpisodes)
		if err != nil {
			log.Printf("Error processing audio entry %s: %v", entry.Title, err)
			continue
		}
		results = append(results, result)
	}

	job.ProcessedFiles = results
	job.Status = "completed"
	completedAt := time.Now().UTC()
	job.CompletedAt = &completedAt

	log.Printf("Job %s completed with %d successful files", jobID, len(results))
	return results, nil
}

// processAudioEntry processes a single audio entry
func (p *Processor) processAudioEntry(ctx context.Context, entry AudioEntry, speed float64, oldEpisodes map[string]map[string]interface{}) (map[string]interface{}, error) {
	title := entry.Title
	duration := entry.Duration
	expectedNewDuration := int(duration / speed)

	// Check if we can reuse existing processed file
	if oldEp, exists := oldEpisodes[title]; exists {
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
					return result, nil
				}
			}
		}
	}

	// Create temp file for download
	tempFile, err := os.CreateTemp("", "audio_*.mp3")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFile.Close()

	// Download audio file
	start := time.Now()
	if err := p.downloadAudioFile(ctx, entry.URL, tempFile.Name()); err != nil {
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to download audio: %w", err)
	}
	downloadTime := time.Since(start)
	log.Printf("Downloaded %s in %.2f seconds", title, downloadTime.Seconds())

	// Create output temp file
	outputFile, err := os.CreateTemp("", "processed_*.mp3")
	if err != nil {
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to create output temp file: %w", err)
	}
	outputFile.Close()

	// Process with FFmpeg
	start = time.Now()
	if err := p.processAudioWithFFmpeg(ctx, tempFile.Name(), outputFile.Name(), speed); err != nil {
		os.Remove(tempFile.Name())
		os.Remove(outputFile.Name())
		return nil, fmt.Errorf("failed to process audio with FFmpeg: %w", err)
	}
	ffmpegTime := time.Since(start)
	log.Printf("FFmpeg processed %s in %.2f seconds", title, ffmpegTime.Seconds())

	// Clean up input file
	os.Remove(tempFile.Name())

	newDuration := int(duration / speed)

	return map[string]interface{}{
		"title":             title,
		"original_url":      entry.URL,
		"original_duration": duration,
		"new_duration":      newDuration,
		"uuid":              entry.UUID,
		"speed":             speed,
		"temp_file":         outputFile.Name(),
	}, nil
}

// parseM3U8 parses M3U8 content and extracts audio entries
func (p *Processor) parseM3U8(content string) []AudioEntry {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var entries []AudioEntry

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "#EXTINF:") {
			re := regexp.MustCompile(`^#EXTINF:([0-9.]+),(.+)$`)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				duration, err := strconv.ParseFloat(matches[1], 64)
				if err != nil {
					continue
				}
				title := strings.TrimSpace(matches[2])

				if i+1 < len(lines) {
					url := strings.TrimSpace(lines[i+1])
					if url != "" && !strings.HasPrefix(url, "#") {
						entries = append(entries, AudioEntry{
							Title:    title,
							Duration: duration,
							URL:      url,
							UUID:     uuid.New().String(),
						})
						i++ // Skip the URL line
						continue
					}
				}
			}
		}
	}

	return entries
}

// downloadAudioFile downloads an audio file from URL to local path
func (p *Processor) downloadAudioFile(ctx context.Context, url, outputPath string) error {
	log.Printf("Downloading audio from: %s", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Minute, // Long timeout for large files
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download audio file: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// processAudioWithFFmpeg processes audio with FFmpeg
func (p *Processor) processAudioWithFFmpeg(ctx context.Context, inputPath, outputPath string, speed float64) error {
	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", inputPath,
		"-filter:a", fmt.Sprintf("atempo=%.1f", speed),
		"-y",
		outputPath,
	)

	log.Printf("Starting FFmpeg processing with %.1fx speed...", speed)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("FFmpeg error: %w, output: %s", err, string(output))
	}

	return nil
}

// GetJobStatus returns the status of a specific job
func (p *Processor) GetJobStatus(jobID string) *ProcessingJob {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.jobs[jobID]
}

// ListJobs returns all jobs
func (p *Processor) ListJobs() []*ProcessingJob {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	jobs := make([]*ProcessingJob, 0, len(p.jobs))
	for _, job := range p.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}
