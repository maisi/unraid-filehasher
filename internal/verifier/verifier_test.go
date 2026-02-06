package verifier

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maisi/unraid-filehasher/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func writeTestFile(t *testing.T, path string, content []byte) string {
	t.Helper()
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func TestVerifyAllOK(t *testing.T) {
	database := setupTestDB(t)
	dir := t.TempDir()

	// Create a real file
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world\n")
	hash := writeTestFile(t, path, content)

	stat, _ := os.Stat(path)
	now := time.Now()

	// Insert into DB with correct hash
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &db.FileRecord{
		Path:         path,
		Disk:         "disk1",
		Size:         stat.Size(),
		Mtime:        stat.ModTime().Unix(),
		SHA256:       hash,
		FirstSeen:    now,
		LastVerified: now,
		Status:       "ok",
	})
	tx.Commit()

	v := New(database, 1, false)

	var results []VerifyResult
	summary, err := v.VerifyAll(func(r VerifyResult) {
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}

	if summary.TotalChecked != 1 {
		t.Errorf("TotalChecked = %d, want 1", summary.TotalChecked)
	}
	if summary.OK != 1 {
		t.Errorf("OK = %d, want 1", summary.OK)
	}
	if summary.Corrupted != 0 {
		t.Errorf("Corrupted = %d, want 0", summary.Corrupted)
	}
	if summary.Missing != 0 {
		t.Errorf("Missing = %d, want 0", summary.Missing)
	}
	if len(results) != 1 || results[0].Status != "ok" {
		t.Errorf("expected 1 result with status ok, got %v", results)
	}
}

func TestVerifyCorrupted(t *testing.T) {
	database := setupTestDB(t)
	dir := t.TempDir()

	path := filepath.Join(dir, "test.txt")
	content := []byte("original content\n")
	writeTestFile(t, path, content)

	stat, _ := os.Stat(path)
	now := time.Now()

	// Insert with WRONG hash to simulate corruption
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &db.FileRecord{
		Path:         path,
		Disk:         "disk1",
		Size:         stat.Size(),
		Mtime:        stat.ModTime().Unix(),
		SHA256:       "0000000000000000000000000000000000000000000000000000000000000000",
		FirstSeen:    now,
		LastVerified: now,
		Status:       "ok",
	})
	tx.Commit()

	v := New(database, 1, false)

	var results []VerifyResult
	summary, err := v.VerifyAll(func(r VerifyResult) {
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}

	if summary.Corrupted != 1 {
		t.Errorf("Corrupted = %d, want 1", summary.Corrupted)
	}
	if len(results) != 1 || results[0].Status != "corrupted" {
		t.Errorf("expected 1 corrupted result, got %v", results)
	}
}

func TestVerifyMissing(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now()

	// Insert a file that doesn't exist on disk
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &db.FileRecord{
		Path:         "/nonexistent/file/that/does/not/exist.txt",
		Disk:         "disk1",
		Size:         100,
		Mtime:        now.Unix(),
		SHA256:       "abc123",
		FirstSeen:    now,
		LastVerified: now,
		Status:       "ok",
	})
	tx.Commit()

	v := New(database, 1, false)

	var results []VerifyResult
	summary, err := v.VerifyAll(func(r VerifyResult) {
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}

	if summary.Missing != 1 {
		t.Errorf("Missing = %d, want 1", summary.Missing)
	}
	if len(results) != 1 || results[0].Status != "missing" {
		t.Errorf("expected 1 missing result, got %v", results)
	}
}

func TestVerifyQuickModeSkip(t *testing.T) {
	database := setupTestDB(t)
	dir := t.TempDir()

	path := filepath.Join(dir, "test.txt")
	content := []byte("test content\n")
	hash := writeTestFile(t, path, content)

	stat, _ := os.Stat(path)
	now := time.Now()

	// Insert with matching size and mtime
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &db.FileRecord{
		Path:         path,
		Disk:         "disk1",
		Size:         stat.Size(),
		Mtime:        stat.ModTime().Unix(),
		SHA256:       hash,
		FirstSeen:    now,
		LastVerified: now,
		Status:       "ok",
	})
	tx.Commit()

	// Verify with quick=true — should skip since mtime/size unchanged
	v := New(database, 1, true)

	summary, err := v.VerifyAll(func(r VerifyResult) {})
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}

	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	if summary.TotalChecked != 0 {
		t.Errorf("TotalChecked = %d, want 0 (skipped files don't count as checked)", summary.TotalChecked)
	}
}

func TestVerifyDisk(t *testing.T) {
	database := setupTestDB(t)
	dir := t.TempDir()

	// Create files on two "disks"
	path1 := filepath.Join(dir, "file1.txt")
	hash1 := writeTestFile(t, path1, []byte("disk1 content\n"))
	stat1, _ := os.Stat(path1)

	path2 := filepath.Join(dir, "file2.txt")
	writeTestFile(t, path2, []byte("disk2 content\n"))
	stat2, _ := os.Stat(path2)

	now := time.Now()
	tx, _ := database.BeginBatch()
	database.UpsertFileTx(tx, &db.FileRecord{
		Path: path1, Disk: "disk1", Size: stat1.Size(), Mtime: stat1.ModTime().Unix(),
		SHA256: hash1, FirstSeen: now, LastVerified: now, Status: "ok",
	})
	database.UpsertFileTx(tx, &db.FileRecord{
		Path: path2, Disk: "disk2", Size: stat2.Size(), Mtime: stat2.ModTime().Unix(),
		SHA256: "wrong_hash", FirstSeen: now, LastVerified: now, Status: "ok",
	})
	tx.Commit()

	// Verify only disk1 — should only see 1 file, and it should be OK
	v := New(database, 1, false)
	summary, err := v.VerifyDisk("disk1", func(r VerifyResult) {})
	if err != nil {
		t.Fatalf("VerifyDisk: %v", err)
	}

	if summary.TotalChecked != 1 {
		t.Errorf("TotalChecked = %d, want 1", summary.TotalChecked)
	}
	if summary.OK != 1 {
		t.Errorf("OK = %d, want 1", summary.OK)
	}
}

func TestNewVerifierDefaultWorkers(t *testing.T) {
	v := New(nil, 0, false)
	if v.workers != 4 {
		t.Errorf("workers = %d, want 4 (default)", v.workers)
	}

	v = New(nil, -1, false)
	if v.workers != 4 {
		t.Errorf("workers = %d, want 4 (negative input)", v.workers)
	}
}
