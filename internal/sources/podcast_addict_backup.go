package sources

// NOTE: Minimal Go translation scaffold of podcastaddict_backup.py.
// Full ZIP + SQLite parsing intentionally deferred to keep scope small.

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cobblepod/internal/gdrive"

	_ "modernc.org/sqlite"
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

	query := "name contains 'PodcastAddict' and name contains '.backup' and trashed = false"
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

	progress, err := p.queryListeningProgress(db)
	if err != nil {
		return nil, fmt.Errorf("querying listening progress: %w", err)
	}

	// Update the provided episode map with the offsets from progress
	p.updateEpisodeMap(progress, epMap)

	return progress, nil
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

// queryListeningProgress opens the SQLite database at dbPath in read-only mode
// and returns the rows from the listening progress query.
func (p *PodcastAddictBackup) queryListeningProgress(dbPath string) ([]ListeningProgress, error) {
	// Open read-only using a proper file URI to avoid accidental writes.
	u := &url.URL{Scheme: "file", Path: dbPath, RawQuery: "mode=ro&_busy_timeout=5000"}
	dsn := u.String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	// getting rid of e.position_to_resume > 0 gives the actual playlist
	// also, that order by is pretty useless (why do we need an order).
	const q = `
			SELECT 
				p.name as podcast,
				e.position_to_resume as offset,
				e.name as episode
			FROM episodes e
			JOIN podcasts p ON p._id = e.podcast_id
			JOIN ordered_list ol ON ol.id = e._id
			WHERE e.position_to_resume > 0 AND ol.type = 1`

	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	results := make([]ListeningProgress, 0, 64)
	for rows.Next() {
		var lp ListeningProgress
		if err := rows.Scan(&lp.Podcast, &lp.Offset, &lp.Episode); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		results = append(results, lp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return results, nil
}

// updateEpisodeMap applies listening progress offsets into the provided episode map.
// Key format mirrors Python: "<podcast> - <episode>".5
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
