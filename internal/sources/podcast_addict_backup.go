package sources

// NOTE: Minimal Go translation scaffold of podcastaddict_backup.py.
// Full ZIP + SQLite parsing intentionally deferred to keep scope small.

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cobblepod/internal/gdrive"
)

// ListeningProgress represents a single episode listening offset.
type ListeningProgress struct {
	Podcast string
	Episode string
	Offset  int64
}

// PodcastAddictBackup handles extraction of listening progress from Podcast Addict backups.
type PodcastAddictBackup struct {
	drive *gdrive.Service
}

// NewPodcastAddictBackup constructs a new handler.
func NewPodcastAddictBackup(drive *gdrive.Service) *PodcastAddictBackup {
	return &PodcastAddictBackup{drive: drive}
}

// AddListeningProgress locates the most recent backup and will (later) augment epMap with offsets.
// Currently returns an empty slice as a placeholder.
func (p *PodcastAddictBackup) AddListeningProgress(ctx context.Context, epMap map[string]map[string]interface{}) ([]ListeningProgress, error) {
	if p.drive == nil {
		return nil, errors.New("drive service is nil")
	}

	query := "name contains 'PodcastAddict' and name contains '.backup'"
	files, err := p.drive.GetFiles(query, true)
	if err != nil {
		return nil, fmt.Errorf("querying backup files: %w", err)
	}
	if len(files) == 0 {
		return nil, errors.New("no PodcastAddict backup files found in Google Drive")
	}

	latest := files[0]
	log.Printf("Found PodcastAddict backup candidate: %s (modified %s)", latest.Name, latest.ModifiedTime)

	backup, err := p.drive.DownloadFileToTemp(latest.Id)
	if err != nil {
		return nil, fmt.Errorf("downloading backup file: %w", err)
	}
	defer os.Remove(backup)

	db, err := p.extractBackupDB(backup)
	if err != nil {
		return nil, fmt.Errorf("extracting backup archive: %w", err)
	}
	defer os.Remove(db)

	return nil, nil
}

// extractBackupDB creates extracts the ZIP-formatted
// Podcast Addict backup at backupPath database.
func (p *PodcastAddictBackup) extractBackupDB(backupPath string) (string, error) {
	r, err := zip.OpenReader(backupPath)
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	// find the db
	var dbFile *zip.File
	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".db") {
			continue
		}
		dbFile = f
		break
	}

	if dbFile == nil {
		return "", errors.New("no .db file found in Podcast Addict backup")
	}

	if dbFile.FileInfo().IsDir() {
		return "", fmt.Errorf("backup db is dir %s", dbFile.Name)
	}

	tempDB, err := os.CreateTemp("", "podcast_addict_backup_*")
	if err != nil {
		return "", fmt.Errorf("creating temp db: %w", err)
	}
	defer tempDB.Close()

	rc, err := dbFile.Open()
	if err != nil {
		return "", fmt.Errorf("opening file %s in zip: %w", dbFile.Name, err)
	}
	defer rc.Close()

	if _, err := io.Copy(tempDB, rc); err != nil {
		return "", fmt.Errorf("copying db file contents: %w", err)
	}

	return tempDB.Name(), nil
}

// updateEpisodeMap applies listening progress offsets into the provided episode map.
// Key format mirrors Python: "<podcast> - <episode>".
func (p *PodcastAddictBackup) updateEpisodeMap(progress []ListeningProgress, epMap map[string]map[string]interface{}) {
	for _, pr := range progress {
		key := fmt.Sprintf("%s - %s", pr.Podcast, pr.Episode)
		m := epMap[key]
		if m == nil {
			m = make(map[string]interface{})
			epMap[key] = m
		}
		m["offset"] = pr.Offset
	}
}

// isPodcastAddictBackupName returns true if filename looks like a PodcastAddict backup.
func isPodcastAddictBackupName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "podcastaddict") && filepath.Ext(lower) == ".backup"
}

// parseModifiedTime tries to parse an RFC3339 time, returns zero time on failure.
func parseModifiedTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
