package db

import (
	"database/sql"
	"fmt"
	"time"
)

// DownloadRecord represents a download in the database
type DownloadRecord struct {
	ID              string
	URL             string
	Filename        string
	Status          string
	Progress        float64
	BytesDownloaded int64
	TotalBytes      int64
	Speed           int64
	Error           string
	StartedAt       time.Time
	CompletedAt     *time.Time
	RetryCount      int
	SupportsResume  bool
}

// SaveDownload saves or updates a download in the database
func (s *Store) SaveDownload(d *DownloadRecord) error {
	query := `
		INSERT INTO downloads (
			id, url, filename, status, progress, bytes_downloaded, 
			total_bytes, speed, error, started_at, completed_at, 
			retry_count, supports_resume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			progress = excluded.progress,
			bytes_downloaded = excluded.bytes_downloaded,
			total_bytes = excluded.total_bytes,
			speed = excluded.speed,
			error = excluded.error,
			completed_at = excluded.completed_at,
			retry_count = excluded.retry_count,
			supports_resume = excluded.supports_resume
	`

	// Strip monotonic clock reading from timestamps for proper SQLite storage
	startedAt := d.StartedAt.Round(0)

	var completedAt interface{}
	if d.CompletedAt != nil {
		t := d.CompletedAt.Round(0)
		completedAt = t
	}

	_, err := s.db.Exec(query,
		d.ID, d.URL, d.Filename, d.Status, d.Progress, d.BytesDownloaded,
		d.TotalBytes, d.Speed, d.Error, startedAt, completedAt,
		d.RetryCount, d.SupportsResume,
	)

	if err != nil {
		return fmt.Errorf("failed to save download: %w", err)
	}

	return nil
}

// GetActiveDownloads retrieves all downloads that are in progress or pending retry
func (s *Store) GetActiveDownloads() ([]*DownloadRecord, error) {
	query := `
		SELECT id, url, filename, status, progress, bytes_downloaded, 
		       total_bytes, speed, error, started_at, completed_at, 
		       retry_count, supports_resume
		FROM downloads
		WHERE status IN ('downloading', 'retrying', 'pending')
		ORDER BY started_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query downloads: %w", err)
	}
	defer rows.Close()

	var downloads []*DownloadRecord
	for rows.Next() {
		var d DownloadRecord
		var completedAt sql.NullTime

		err := rows.Scan(
			&d.ID, &d.URL, &d.Filename, &d.Status, &d.Progress, &d.BytesDownloaded,
			&d.TotalBytes, &d.Speed, &d.Error, &d.StartedAt, &completedAt,
			&d.RetryCount, &d.SupportsResume,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download: %w", err)
		}

		if completedAt.Valid {
			d.CompletedAt = &completedAt.Time
		}

		downloads = append(downloads, &d)
	}

	return downloads, rows.Err()
}

// DeleteDownload removes a download from the database
func (s *Store) DeleteDownload(id string) error {
	_, err := s.db.Exec("DELETE FROM downloads WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete download: %w", err)
	}
	return nil
}

// CleanupOldDownloads removes completed/failed downloads older than the specified duration
func (s *Store) CleanupOldDownloads(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	query := `
		DELETE FROM downloads 
		WHERE status IN ('completed', 'failed') 
		AND started_at < ?
	`

	_, err := s.db.Exec(query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old downloads: %w", err)
	}

	return nil
}

// GetAllDownloads retrieves all downloads from the database
func (s *Store) GetAllDownloads() ([]*DownloadRecord, error) {
	query := `
		SELECT id, url, filename, status, progress, bytes_downloaded, 
		       total_bytes, speed, error, started_at, completed_at, 
		       retry_count, supports_resume
		FROM downloads
		ORDER BY started_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query downloads: %w", err)
	}
	defer rows.Close()

	var downloads []*DownloadRecord
	for rows.Next() {
		var d DownloadRecord
		var completedAt sql.NullTime

		err := rows.Scan(
			&d.ID, &d.URL, &d.Filename, &d.Status, &d.Progress, &d.BytesDownloaded,
			&d.TotalBytes, &d.Speed, &d.Error, &d.StartedAt, &completedAt,
			&d.RetryCount, &d.SupportsResume,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download: %w", err)
		}

		if completedAt.Valid {
			d.CompletedAt = &completedAt.Time
		}

		downloads = append(downloads, &d)
	}

	return downloads, rows.Err()
}

// GetDownload retrieves a single download by ID
func (s *Store) GetDownload(id string) (*DownloadRecord, error) {
	query := `
		SELECT id, url, filename, status, progress, bytes_downloaded, 
		       total_bytes, speed, error, started_at, completed_at, 
		       retry_count, supports_resume
		FROM downloads
		WHERE id = ?
	`

	var d DownloadRecord
	var completedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&d.ID, &d.URL, &d.Filename, &d.Status, &d.Progress, &d.BytesDownloaded,
		&d.TotalBytes, &d.Speed, &d.Error, &d.StartedAt, &completedAt,
		&d.RetryCount, &d.SupportsResume,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("download not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query download: %w", err)
	}

	if completedAt.Valid {
		d.CompletedAt = &completedAt.Time
	}

	return &d, nil
}
