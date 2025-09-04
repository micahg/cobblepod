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
	"strings"
	"time"

	"cobblepod/internal/gdrive"
	"cobblepod/internal/podcast"

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

// TODO GetLatestBackupFile abd GetLatestM3U8File share a lot of code - normalize them.

// GetLatestBackupFile checks for the most recent backup file and returns metadata
func (p *PodcastAddictBackup) GetLatestBackupFile(ctx context.Context) (*FileInfo, error) {
	query := "name contains 'PodcastAddict' and name contains '.backup' and trashed = false"
	files, err := p.drive.GetFiles(query, true)
	if err != nil {
		return nil, fmt.Errorf("querying backup files: %w", err)
	}
	if len(files) == 0 {
		return nil, nil // No backup files found
	}

	mostRecentFile := p.drive.GetMostRecentFile(files)
	if mostRecentFile == nil {
		return nil, nil
	}

	modifiedTime, err := time.Parse(time.RFC3339, mostRecentFile.ModifiedTime)
	if err != nil {
		log.Printf("Error parsing backup modified time: %v", err)
		modifiedTime = time.Time{} // Zero time as fallback
	}

	backupInfo := &FileInfo{
		File:         mostRecentFile,
		FileName:     mostRecentFile.Name,
		ModifiedTime: modifiedTime,
	}

	return backupInfo, nil
}

// AddListeningProgress locates the most recent backup and will (later) augment epMap with offsets.
// Currently returns an empty slice as a placeholder.
func (p *PodcastAddictBackup) AddListeningProgress(ctx context.Context, epMap map[string]podcast.ExistingEpisode) ([]ListeningProgress, error) {
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

// Process locates the most recent backup and processes all episodes for independent processing.
// This is used when processing backup without M3U8 file.
func (p *PodcastAddictBackup) Process(ctx context.Context, backupFile *FileInfo) ([]podcast.ProcessedEpisode, error) {
	if p.drive == nil {
		return nil, errors.New("drive service is nil")
	}

	if backupFile == nil {
		return nil, errors.New("no backup file provided")
	}

	log.Printf("Processing PodcastAddict backup: %s (modified %s)", backupFile.FileName, backupFile.ModifiedTime)

	backup, err := p.drive.DownloadFileToTemp(backupFile.File.Id)
	if err != nil {
		return nil, fmt.Errorf("downloading backup file: %w", err)
	}
	defer os.Remove(backup)

	db, err := p.extractBackupDB(backup)
	if err != nil {
		return nil, fmt.Errorf("extracting backup archive: %w", err)
	}
	defer os.Remove(db)

	progress, err := p.queryAllEpisodes(db)
	if err != nil {
		return nil, fmt.Errorf("querying all episodes: %w", err)
	}

	// Convert listening progress to processed episodes
	var results []podcast.ProcessedEpisode
	for _, pr := range progress {
		result := podcast.ProcessedEpisode{
			Title:            pr.Episode,
			OriginalDuration: 0,   // Will be set during processing
			NewDuration:      0,   // Will be calculated
			UUID:             "",  // Will be generated
			Speed:            1.5, // Default speed from config
			// Note: TempFile and other fields will be set during actual processing
		}
		results = append(results, result)
	}

	return results, nil
}

// queryAllEpisodes opens the SQLite database at dbPath and returns all episodes
// without the position_to_resume > 0 filter for independent backup processing.
func (p *PodcastAddictBackup) queryAllEpisodes(dbPath string) ([]ListeningProgress, error) {
	// Open read-only using a proper file URI to avoid accidental writes.
	u := &url.URL{Scheme: "file", Path: dbPath, RawQuery: "mode=ro&_busy_timeout=5000"}
	dsn := u.String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	// Removed e.position_to_resume > 0 to get all episodes in the playlist
	const q = `
			SELECT 
				p.name as podcast,
				e.position_to_resume as offset,
				e.name as episode
			FROM episodes e
			JOIN podcasts p ON p._id = e.podcast_id
			JOIN ordered_list ol ON ol.id = e._id
			WHERE ol.type = 1`

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
// Key format mirrors Python: "<podcast> - <episode>".
func (p *PodcastAddictBackup) updateEpisodeMap(progress []ListeningProgress, epMap map[string]podcast.ExistingEpisode) {
	for _, pr := range progress {
		key := fmt.Sprintf("%s - %s", pr.Podcast, pr.Episode)
		episode := epMap[key]
		episode.Offset = pr.Offset
		epMap[key] = episode
	}
}
