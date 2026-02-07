package db

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// FileRecord represents a single file entry in the catalog.
type FileRecord struct {
	ID           int64
	Path         string
	Disk         string
	Size         int64
	Mtime        int64
	SHA256       string
	FirstSeen    time.Time
	LastVerified time.Time
	Status       string // ok, corrupted, missing, new, moved
}

// Stats holds aggregate statistics for the catalog.
type Stats struct {
	TotalFiles     int64
	TotalSize      int64
	OKFiles        int64
	CorruptedFiles int64
	MissingFiles   int64
	NewFiles       int64
	LastScan       *time.Time
	LastVerify     *time.Time
}

// DiskStats holds per-disk statistics.
type DiskStats struct {
	Disk           string
	TotalFiles     int64
	TotalSize      int64
	CorruptedFiles int64
	MissingFiles   int64
	LastVerified   *time.Time
}

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// Open opens or creates the SQLite database at the given path.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Set pragmas for performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000", // 64MB cache
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		path          TEXT NOT NULL UNIQUE,
		disk          TEXT NOT NULL,
		size          INTEGER NOT NULL,
		mtime         INTEGER NOT NULL,
		sha256        TEXT NOT NULL,
		first_seen    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_verified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status        TEXT NOT NULL DEFAULT 'ok'
	);

	CREATE INDEX IF NOT EXISTS idx_files_disk ON files(disk);
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
	CREATE INDEX IF NOT EXISTS idx_files_sha256 ON files(sha256);

	CREATE TABLE IF NOT EXISTS scan_history (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		scan_type  TEXT NOT NULL,
		started_at TIMESTAMP NOT NULL,
		ended_at   TIMESTAMP,
		disks      TEXT,
		files_processed INTEGER DEFAULT 0,
		errors     INTEGER DEFAULT 0,
		status     TEXT NOT NULL DEFAULT 'running'
	);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// BeginBatch starts a transaction for batch operations.
func (db *DB) BeginBatch() (*sql.Tx, error) {
	return db.conn.Begin()
}

// UpsertFileTx inserts or updates a file record within a transaction.
func (db *DB) UpsertFileTx(tx *sql.Tx, f *FileRecord) error {
	_, err := tx.Exec(`
		INSERT INTO files (path, disk, size, mtime, sha256, first_seen, last_verified, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			disk = excluded.disk,
			size = excluded.size,
			mtime = excluded.mtime,
			sha256 = excluded.sha256,
			last_verified = excluded.last_verified,
			status = excluded.status
	`, f.Path, f.Disk, f.Size, f.Mtime, f.SHA256, f.FirstSeen, f.LastVerified, f.Status)
	return err
}

// QuickLookup holds minimal file info for incremental scan comparison.
type QuickLookup struct {
	Size   int64
	Mtime  int64
	SHA256 string
}

// LoadQuickLookupMap loads all file records into a map for fast path-based lookups.
// This is much more efficient than per-file queries when scanning large directories.
func (db *DB) LoadQuickLookupMap() (map[string]*QuickLookup, error) {
	rows, err := db.conn.Query(`SELECT path, size, mtime, sha256 FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]*QuickLookup)
	for rows.Next() {
		var path string
		var ql QuickLookup
		if err := rows.Scan(&path, &ql.Size, &ql.Mtime, &ql.SHA256); err != nil {
			return nil, err
		}
		m[path] = &ql
	}
	return m, rows.Err()
}

// GetFilesByDisk returns all file records on a given disk.
func (db *DB) GetFilesByDisk(disk string) ([]*FileRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, path, disk, size, mtime, sha256, first_seen, last_verified, status
		FROM files WHERE disk = ?
		ORDER BY path
	`, disk)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFileRows(rows)
}

// GetFilesByStatus returns all file records with a given status.
func (db *DB) GetFilesByStatus(status string) ([]*FileRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, path, disk, size, mtime, sha256, first_seen, last_verified, status
		FROM files WHERE status = ?
		ORDER BY path
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFileRows(rows)
}

// GetAllFiles returns all file records for verification.
func (db *DB) GetAllFiles() ([]*FileRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, path, disk, size, mtime, sha256, first_seen, last_verified, status
		FROM files
		ORDER BY path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFileRows(rows)
}

// UpdateStatusTx updates the status and last_verified time within a transaction.
func (db *DB) UpdateStatusTx(tx *sql.Tx, path, status string) error {
	_, err := tx.Exec(`
		UPDATE files SET status = ?, last_verified = CURRENT_TIMESTAMP
		WHERE path = ?
	`, status, path)
	return err
}

// FindMoveCandidates looks up existing records that could correspond to a moved file.
// It matches by file basename (path suffix) + size, which is a reasonably strong heuristic
// without needing to hash the whole catalog.
func (db *DB) FindMoveCandidates(baseName string, size int64, limit int) ([]*FileRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.conn.Query(`
		SELECT id, path, disk, size, mtime, sha256, first_seen, last_verified, status
		FROM files
		WHERE size = ? AND path LIKE ?
		ORDER BY last_verified DESC
		LIMIT ?
	`, size, "%/"+baseName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFileRows(rows)
}

// MovePathTx re-keys a record from oldPath to newPath.
// This is used when a scan determines a file was moved but content stayed identical.
func (db *DB) MovePathTx(tx *sql.Tx, oldPath, newPath, newDisk string, newSize int64, newMtime int64) error {
	// If the destination path already exists (e.g., partial previous scan), remove it.
	if _, err := tx.Exec(`DELETE FROM files WHERE path = ?`, newPath); err != nil {
		return err
	}
	_, err := tx.Exec(`
		UPDATE files
		SET path = ?, disk = ?, size = ?, mtime = ?, last_verified = CURRENT_TIMESTAMP, status = 'ok'
		WHERE path = ?
	`, newPath, newDisk, newSize, newMtime, oldPath)
	return err
}

// GetStats returns aggregate statistics.
func (db *DB) GetStats() (*Stats, error) {
	s := &Stats{}

	err := db.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size),0) FROM files`).
		Scan(&s.TotalFiles, &s.TotalSize)
	if err != nil {
		return nil, err
	}

	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM files WHERE status = 'ok'`).Scan(&s.OKFiles); err != nil {
		return nil, fmt.Errorf("count ok files: %w", err)
	}
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM files WHERE status = 'corrupted'`).Scan(&s.CorruptedFiles); err != nil {
		return nil, fmt.Errorf("count corrupted files: %w", err)
	}
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM files WHERE status = 'missing'`).Scan(&s.MissingFiles); err != nil {
		return nil, fmt.Errorf("count missing files: %w", err)
	}
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM files WHERE status = 'new'`).Scan(&s.NewFiles); err != nil {
		return nil, fmt.Errorf("count new files: %w", err)
	}

	var lastScan, lastVerify sql.NullString
	if err := db.conn.QueryRow(`SELECT MAX(ended_at) FROM scan_history WHERE scan_type = 'scan' AND status = 'completed'`).
		Scan(&lastScan); err != nil {
		return nil, fmt.Errorf("query last scan: %w", err)
	}
	if err := db.conn.QueryRow(`SELECT MAX(ended_at) FROM scan_history WHERE scan_type = 'verify' AND status = 'completed'`).
		Scan(&lastVerify); err != nil {
		return nil, fmt.Errorf("query last verify: %w", err)
	}

	if lastScan.Valid {
		if t, err := parseTime(lastScan.String); err == nil {
			s.LastScan = &t
		}
	}
	if lastVerify.Valid {
		if t, err := parseTime(lastVerify.String); err == nil {
			s.LastVerify = &t
		}
	}

	return s, nil
}

// GetDiskStats returns per-disk statistics.
func (db *DB) GetDiskStats() ([]*DiskStats, error) {
	rows, err := db.conn.Query(`
		SELECT
			disk,
			COUNT(*) as total_files,
			COALESCE(SUM(size), 0) as total_size,
			COALESCE(SUM(CASE WHEN status = 'corrupted' THEN 1 ELSE 0 END), 0) as corrupted,
			COALESCE(SUM(CASE WHEN status = 'missing' THEN 1 ELSE 0 END), 0) as missing,
			MAX(last_verified) as last_verified
		FROM files
		GROUP BY disk
		ORDER BY disk
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*DiskStats
	for rows.Next() {
		ds := &DiskStats{}
		var lastVerified sql.NullString
		if err := rows.Scan(&ds.Disk, &ds.TotalFiles, &ds.TotalSize,
			&ds.CorruptedFiles, &ds.MissingFiles, &lastVerified); err != nil {
			return nil, err
		}
		if lastVerified.Valid {
			if t, err := parseTime(lastVerified.String); err == nil {
				ds.LastVerified = &t
			}
		}
		stats = append(stats, ds)
	}
	return stats, rows.Err()
}

// InsertScanHistory records a scan/verify operation.
func (db *DB) InsertScanHistory(scanType, disks string) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO scan_history (scan_type, started_at, disks, status)
		VALUES (?, CURRENT_TIMESTAMP, ?, 'running')
	`, scanType, disks)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CompleteScanHistory marks a scan as completed.
func (db *DB) CompleteScanHistory(id int64, filesProcessed, errors int) error {
	_, err := db.conn.Exec(`
		UPDATE scan_history
		SET ended_at = CURRENT_TIMESTAMP, files_processed = ?, errors = ?, status = 'completed'
		WHERE id = ?
	`, filesProcessed, errors, id)
	return err
}

// SearchFiles searches for files by path pattern.
func (db *DB) SearchFiles(pattern string, limit int) ([]*FileRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.conn.Query(`
		SELECT id, path, disk, size, mtime, sha256, first_seen, last_verified, status
		FROM files WHERE path LIKE ?
		ORDER BY path
		LIMIT ?
	`, "%"+pattern+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFileRows(rows)
}

// GetScanHistory returns recent scan history entries.
func (db *DB) GetScanHistory(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(`
		SELECT id, scan_type, started_at, ended_at, disks, files_processed, errors, status
		FROM scan_history
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var id int64
		var filesProcessed, errCount int
		var scanType, disks, status string
		var startedAtStr string
		var endedAtStr sql.NullString

		if err := rows.Scan(&id, &scanType, &startedAtStr, &endedAtStr, &disks, &filesProcessed, &errCount, &status); err != nil {
			return nil, err
		}
		startedAt, err := parseTime(startedAtStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse started_at for scan %d: %v\n", id, err)
		}
		entry := map[string]interface{}{
			"id":              id,
			"scan_type":       scanType,
			"started_at":      startedAt.Format("2006-01-02 15:04:05"),
			"disks":           disks,
			"files_processed": filesProcessed,
			"errors":          errCount,
			"status":          status,
		}
		if endedAtStr.Valid {
			if t, err := parseTime(endedAtStr.String); err == nil {
				entry["ended_at"] = t.Format("2006-01-02 15:04:05")
			}
		}
		history = append(history, entry)
	}
	return history, rows.Err()
}

func scanFileRows(rows *sql.Rows) ([]*FileRecord, error) {
	var files []*FileRecord
	for rows.Next() {
		f := &FileRecord{}
		var firstSeen, lastVerified string
		if err := rows.Scan(&f.ID, &f.Path, &f.Disk, &f.Size, &f.Mtime, &f.SHA256,
			&firstSeen, &lastVerified, &f.Status); err != nil {
			return nil, err
		}
		var err error
		f.FirstSeen, err = parseTime(firstSeen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse first_seen for %s: %v\n", f.Path, err)
		}
		f.LastVerified, err = parseTime(lastVerified)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse last_verified for %s: %v\n", f.Path, err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// parseTime tries multiple formats that SQLite might return.
func parseTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}
