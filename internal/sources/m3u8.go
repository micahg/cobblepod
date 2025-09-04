package sources

import (
	"cobblepod/internal/config"
	"cobblepod/internal/gdrive"
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/api/drive/v3"
)

// FileInfo contains metadata about a file (M3U8, backup, etc.)
type FileInfo struct {
	File         *drive.File
	FileName     string
	ModifiedTime time.Time
}

// AudioEntry represents an entry in an M3U8 playlist
type AudioEntry struct {
	Title    string `json:"title"`
	Duration int64  `json:"duration"`
	URL      string `json:"url"`
	UUID     string `json:"uuid"`
}

type M3U8Source struct {
	drive          *gdrive.Service
	mutex          sync.RWMutex
	processedFiles map[string]bool
}

// NewProcessor creates a new audio processor
func NewM3U8Source(driveService *gdrive.Service) *M3U8Source {
	return &M3U8Source{
		drive:          driveService,
		processedFiles: make(map[string]bool),
	}
}

// GetLatestM3U8File checks for the most recent M3U8 file and returns metadata
func (m *M3U8Source) GetLatestM3U8File(ctx context.Context) (*FileInfo, error) {
	files, err := m.drive.GetFiles(config.M3UQuery, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get M3U8 files: %w", err)
	}

	if len(files) == 0 {
		return nil, nil // No files found
	}

	mostRecentFile := m.drive.GetMostRecentFile(files)
	if mostRecentFile == nil {
		return nil, nil
	}

	// Parse the modified time
	modifiedTime, err := time.Parse(time.RFC3339, mostRecentFile.ModifiedTime)
	if err != nil {
		log.Printf("Warning: couldn't parse modified time for %s: %v", mostRecentFile.Name, err)
		modifiedTime = time.Time{} // Zero time as fallback
	}

	return &FileInfo{
		File:         mostRecentFile,
		ModifiedTime: modifiedTime,
		FileName:     mostRecentFile.Name,
	}, nil
}

// Process downloads and parses the M3U8 file
func (m *M3U8Source) Process(ctx context.Context, fileInfo *FileInfo) ([]AudioEntry, error) {
	fileID := fileInfo.File.Id

	// Mark as processed
	m.mutex.Lock()
	m.processedFiles[fileID] = true
	m.mutex.Unlock()

	// Download and parse
	m3u8Content, err := m.drive.DownloadFile(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to download M3U8 file: %w", err)
	}

	audioEntries := m.parseM3U8(m3u8Content)
	if len(audioEntries) == 0 {
		return nil, fmt.Errorf("no audio files found in M3U8 playlist")
	}

	log.Printf("Parsed %d audio entries from M3U8", len(audioEntries))
	return audioEntries, nil
}

// parseM3U8 parses M3U8 content and extracts audio entries
func (m *M3U8Source) parseM3U8(content string) []AudioEntry {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var entries []AudioEntry

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "#EXTINF:") {
			re := regexp.MustCompile(`^#EXTINF:([0-9.]+),(.+)$`)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				duration, err := strconv.ParseInt(matches[1], 10, 64)
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
