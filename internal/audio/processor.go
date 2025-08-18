package audio

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
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
	log.Printf("FFmpeg processing completed: %s", outputPath)

	return nil
}

// DownloadAudioForEntry is a thin wrapper exposing downloadAudioFile
func (p *Processor) DownloadAudioForEntry(ctx context.Context, url, outputPath string) error {
	return p.downloadAudioFile(ctx, url, outputPath)
}

// ProcessWithFFMPEG exposes processAudioWithFFmpeg for external orchestration
func (p *Processor) ProcessWithFFMPEG(ctx context.Context, inputPath, outputPath string, speed float64) error {
	return p.processAudioWithFFmpeg(ctx, inputPath, outputPath, speed)
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
