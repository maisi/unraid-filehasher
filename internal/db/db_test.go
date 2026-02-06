package db

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestOpenAndClose(t *testing.T) {
	database := openTestDB(t)
	// Verify the database is functional by running a simple query
	stats, err := database.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", stats.TotalFiles)
	}
}

func TestUpsertFileTxAndGetFilesByDisk(t *testing.T) {
	database := openTestDB(t)

	now := time.Now().Truncate(time.Second)
	record := &FileRecord{
		Path:         "/mnt/disk1/test.txt",
		Disk:         "disk1",
		Size:         1024,
		Mtime:        now.Unix(),
		SHA256:       "abc123def456",
		FirstSeen:    now,
		LastVerified: now,
		Status:       "ok",
	}

	tx, err := database.BeginBatch()
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := database.UpsertFileTx(tx, record); err != nil {
		tx.Rollback()
		t.Fatalf("UpsertFileTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Retrieve by disk
	files, err := database.GetFilesByDisk("disk1")
	if err != nil {
		t.Fatalf("GetFilesByDisk: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}

	f := files[0]
	if f.Path != record.Path {
		t.Errorf("Path = %q, want %q", f.Path, record.Path)
	}
	if f.Disk != record.Disk {
		t.Errorf("Disk = %q, want %q", f.Disk, record.Disk)
	}
	if f.Size != record.Size {
		t.Errorf("Size = %d, want %d", f.Size, record.Size)
	}
	if f.SHA256 != record.SHA256 {
		t.Errorf("SHA256 = %q, want %q", f.SHA256, record.SHA256)
	}
	if f.Status != "ok" {
		t.Errorf("Status = %q, want ok", f.Status)
	}
}

func TestUpsertFileTxUpdate(t *testing.T) {
	database := openTestDB(t)

	now := time.Now().Truncate(time.Second)
	record := &FileRecord{
		Path:         "/mnt/disk1/test.txt",
		Disk:         "disk1",
		Size:         1024,
		Mtime:        now.Unix(),
		SHA256:       "hash1",
		FirstSeen:    now,
		LastVerified: now,
		Status:       "ok",
	}

	// Insert
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, record)
	tx.Commit()

	// Update with new hash (upsert should update)
	record.SHA256 = "hash2"
	record.Size = 2048
	tx, _ = database.BeginBatch()
	database.UpsertFileTx(tx, record)
	tx.Commit()

	files, _ := database.GetFilesByDisk("disk1")
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1 (upsert should update, not duplicate)", len(files))
	}
	if files[0].SHA256 != "hash2" {
		t.Errorf("SHA256 = %q, want hash2", files[0].SHA256)
	}
	if files[0].Size != 2048 {
		t.Errorf("Size = %d, want 2048", files[0].Size)
	}
}

func TestGetStats(t *testing.T) {
	database := openTestDB(t)

	now := time.Now()

	// Insert files with different statuses
	tx, _ := database.BeginBatch()
	for i, status := range []string{"ok", "ok", "corrupted", "missing"} {
		r := &FileRecord{
			Path:         "/mnt/disk1/file" + string(rune('0'+i)) + ".txt",
			Disk:         "disk1",
			Size:         int64(100 * (i + 1)),
			Mtime:        now.Unix(),
			SHA256:       "hash" + string(rune('0'+i)),
			FirstSeen:    now,
			LastVerified: now,
			Status:       status,
		}
		database.UpsertFileTx(tx, r)
	}
	tx.Commit()

	stats, err := database.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	if stats.TotalFiles != 4 {
		t.Errorf("TotalFiles = %d, want 4", stats.TotalFiles)
	}
	if stats.OKFiles != 2 {
		t.Errorf("OKFiles = %d, want 2", stats.OKFiles)
	}
	if stats.CorruptedFiles != 1 {
		t.Errorf("CorruptedFiles = %d, want 1", stats.CorruptedFiles)
	}
	if stats.MissingFiles != 1 {
		t.Errorf("MissingFiles = %d, want 1", stats.MissingFiles)
	}
	// 100 + 200 + 300 + 400 = 1000
	if stats.TotalSize != 1000 {
		t.Errorf("TotalSize = %d, want 1000", stats.TotalSize)
	}
}

func TestGetDiskStats(t *testing.T) {
	database := openTestDB(t)

	now := time.Now()
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk1/a.txt", Disk: "disk1", Size: 100,
		Mtime: now.Unix(), SHA256: "h1", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk1/b.txt", Disk: "disk1", Size: 200,
		Mtime: now.Unix(), SHA256: "h2", FirstSeen: now, LastVerified: now, Status: "corrupted",
	})
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk2/c.txt", Disk: "disk2", Size: 300,
		Mtime: now.Unix(), SHA256: "h3", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	tx.Commit()

	diskStats, err := database.GetDiskStats()
	if err != nil {
		t.Fatalf("GetDiskStats: %v", err)
	}

	if len(diskStats) != 2 {
		t.Fatalf("got %d disk stats, want 2", len(diskStats))
	}

	// Results are ordered by disk name
	if diskStats[0].Disk != "disk1" {
		t.Errorf("first disk = %q, want disk1", diskStats[0].Disk)
	}
	if diskStats[0].TotalFiles != 2 {
		t.Errorf("disk1 TotalFiles = %d, want 2", diskStats[0].TotalFiles)
	}
	if diskStats[0].CorruptedFiles != 1 {
		t.Errorf("disk1 CorruptedFiles = %d, want 1", diskStats[0].CorruptedFiles)
	}
	if diskStats[1].Disk != "disk2" {
		t.Errorf("second disk = %q, want disk2", diskStats[1].Disk)
	}
}

func TestLoadQuickLookupMap(t *testing.T) {
	database := openTestDB(t)

	now := time.Now()
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk1/a.txt", Disk: "disk1", Size: 100, Mtime: 1000,
		SHA256: "hash_a", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk1/b.txt", Disk: "disk1", Size: 200, Mtime: 2000,
		SHA256: "hash_b", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	tx.Commit()

	m, err := database.LoadQuickLookupMap()
	if err != nil {
		t.Fatalf("LoadQuickLookupMap: %v", err)
	}

	if len(m) != 2 {
		t.Fatalf("got %d entries, want 2", len(m))
	}

	ql := m["/mnt/disk1/a.txt"]
	if ql == nil {
		t.Fatal("missing entry for /mnt/disk1/a.txt")
	}
	if ql.Size != 100 {
		t.Errorf("Size = %d, want 100", ql.Size)
	}
	if ql.Mtime != 1000 {
		t.Errorf("Mtime = %d, want 1000", ql.Mtime)
	}
	if ql.SHA256 != "hash_a" {
		t.Errorf("SHA256 = %q, want hash_a", ql.SHA256)
	}
}

func TestUpdateStatusTx(t *testing.T) {
	database := openTestDB(t)

	now := time.Now()
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk1/a.txt", Disk: "disk1", Size: 100, Mtime: 1000,
		SHA256: "hash_a", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	tx.Commit()

	// Update status
	tx, _ = database.BeginBatch()
	if err := database.UpdateStatusTx(tx, "/mnt/disk1/a.txt", "corrupted"); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateStatusTx: %v", err)
	}
	tx.Commit()

	files, _ := database.GetFilesByStatus("corrupted")
	if len(files) != 1 {
		t.Fatalf("got %d corrupted files, want 1", len(files))
	}
	if files[0].Path != "/mnt/disk1/a.txt" {
		t.Errorf("Path = %q, want /mnt/disk1/a.txt", files[0].Path)
	}
}

func TestGetFilesByStatus(t *testing.T) {
	database := openTestDB(t)

	now := time.Now()
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/a.txt", Disk: "disk1", Size: 100, Mtime: 1000,
		SHA256: "h1", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/b.txt", Disk: "disk1", Size: 200, Mtime: 2000,
		SHA256: "h2", FirstSeen: now, LastVerified: now, Status: "missing",
	})
	tx.Commit()

	ok, _ := database.GetFilesByStatus("ok")
	if len(ok) != 1 {
		t.Errorf("ok files = %d, want 1", len(ok))
	}
	missing, _ := database.GetFilesByStatus("missing")
	if len(missing) != 1 {
		t.Errorf("missing files = %d, want 1", len(missing))
	}
	corrupted, _ := database.GetFilesByStatus("corrupted")
	if len(corrupted) != 0 {
		t.Errorf("corrupted files = %d, want 0", len(corrupted))
	}
}

func TestSearchFiles(t *testing.T) {
	database := openTestDB(t)

	now := time.Now()
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk1/movies/test.mkv", Disk: "disk1", Size: 100, Mtime: 1000,
		SHA256: "h1", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk1/photos/test.jpg", Disk: "disk1", Size: 200, Mtime: 2000,
		SHA256: "h2", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	database.UpsertFileTx(tx, &FileRecord{
		Path: "/mnt/disk2/docs/readme.md", Disk: "disk2", Size: 300, Mtime: 3000,
		SHA256: "h3", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	tx.Commit()

	// Search for "test"
	results, err := database.SearchFiles("test", 100)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results for 'test', want 2", len(results))
	}

	// Search for "readme"
	results, err = database.SearchFiles("readme", 100)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results for 'readme', want 1", len(results))
	}

	// Search with limit
	results, err = database.SearchFiles("disk", 1)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results with limit 1, want 1", len(results))
	}
}

func TestScanHistory(t *testing.T) {
	database := openTestDB(t)

	id, err := database.InsertScanHistory("scan", "disk1,disk2")
	if err != nil {
		t.Fatalf("InsertScanHistory: %v", err)
	}
	if id <= 0 {
		t.Errorf("scan ID = %d, want > 0", id)
	}

	if err := database.CompleteScanHistory(id, 100, 2); err != nil {
		t.Fatalf("CompleteScanHistory: %v", err)
	}

	history, err := database.GetScanHistory(10)
	if err != nil {
		t.Fatalf("GetScanHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("got %d history entries, want 1", len(history))
	}

	entry := history[0]
	if entry["scan_type"] != "scan" {
		t.Errorf("scan_type = %v, want scan", entry["scan_type"])
	}
	if entry["files_processed"] != 100 {
		t.Errorf("files_processed = %v, want 100", entry["files_processed"])
	}
	if entry["errors"] != 2 {
		t.Errorf("errors = %v, want 2", entry["errors"])
	}
	if entry["status"] != "completed" {
		t.Errorf("status = %v, want completed", entry["status"])
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"2025-01-15 10:30:45", true},
		{"2025-01-15T10:30:45Z", true},
		{"2025-01-15T10:30:45", true},
		{"2025-01-15 10:30:45.000", true},
		{"2025-01-15T10:30:45+00:00", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		_, err := parseTime(tt.input)
		if tt.valid && err != nil {
			t.Errorf("parseTime(%q) returned error: %v, want success", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("parseTime(%q) returned nil error, want error", tt.input)
		}
	}
}

func TestGetAllFiles(t *testing.T) {
	database := openTestDB(t)

	now := time.Now()
	tx, _ := database.BeginBatch()
	for i := 0; i < 3; i++ {
		database.UpsertFileTx(tx, &FileRecord{
			Path: "/file" + string(rune('0'+i)), Disk: "disk1", Size: int64(i * 100),
			Mtime: now.Unix(), SHA256: "h" + string(rune('0'+i)),
			FirstSeen: now, LastVerified: now, Status: "ok",
		})
	}
	tx.Commit()

	files, err := database.GetAllFiles()
	if err != nil {
		t.Fatalf("GetAllFiles: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("got %d files, want 3", len(files))
	}
}
