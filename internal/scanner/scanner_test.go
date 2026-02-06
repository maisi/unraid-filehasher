package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maisi/unraid-filehasher/internal/hasher"
)

func TestResolveDisk(t *testing.T) {
	tests := []struct {
		filePath string
		scanRoot string
		expected string
	}{
		{"/mnt/disk1/movies/test.mkv", "/mnt/disk1", "disk1"},
		{"/mnt/disk2/data/file.txt", "/mnt/disk2", "disk2"},
		{"/mnt/disk12/photos/img.jpg", "/mnt/disk12", "disk12"},
		{"/mnt/cache/appdata/test", "/mnt/cache", "cache"},
		{"/mnt/cache2/docker/file", "/mnt/cache2", "cache2"},
		// Non-Unraid paths use the scan root's base name
		{"/home/user/file.txt", "/home/user", "user"},
		{"/data/share/file.txt", "/data/share", "share"},
		// Edge cases
		{"/mnt/user/Movies/test.mkv", "/mnt/user", "user"},
	}

	for _, tt := range tests {
		got := ResolveDisk(tt.filePath, tt.scanRoot)
		if got != tt.expected {
			t.Errorf("ResolveDisk(%q, %q) = %q, want %q",
				tt.filePath, tt.scanRoot, got, tt.expected)
		}
	}
}

func TestDiskTypeString(t *testing.T) {
	tests := []struct {
		dt       DiskType
		expected string
	}{
		{DiskTypeHDD, "HDD"},
		{DiskTypeSSD, "SSD"},
		{DiskTypeUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.expected {
			t.Errorf("DiskType(%d).String() = %q, want %q", tt.dt, got, tt.expected)
		}
	}
}

func TestDiskTypeDefaultWorkers(t *testing.T) {
	tests := []struct {
		dt       DiskType
		expected int
	}{
		{DiskTypeHDD, 1},
		{DiskTypeSSD, 4},
		{DiskTypeUnknown, 2},
	}
	for _, tt := range tests {
		if got := tt.dt.DefaultWorkers(); got != tt.expected {
			t.Errorf("DiskType(%d).DefaultWorkers() = %d, want %d", tt.dt, got, tt.expected)
		}
	}
}

func TestNewScanner(t *testing.T) {
	// Valid patterns
	sc, err := New([]string{`\.tmp$`, `^/mnt/disk1/Trash`})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(sc.excludePatterns) != 2 {
		t.Errorf("got %d patterns, want 2", len(sc.excludePatterns))
	}

	// Invalid pattern
	_, err = New([]string{`[invalid`})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestNewScannerNilPatterns(t *testing.T) {
	sc, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(sc.excludePatterns) != 0 {
		t.Errorf("got %d patterns, want 0", len(sc.excludePatterns))
	}
}

func TestWalk(t *testing.T) {
	// Create a temp directory tree
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create files
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(dir, "file1.txt"), "hello"},
		{filepath.Join(dir, "file2.dat"), "world"},
		{filepath.Join(subdir, "nested.txt"), "nested content"},
	}

	for _, f := range files {
		if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
			t.Fatalf("write %s: %v", f.path, err)
		}
	}

	// Create a zero-byte file (should be skipped)
	emptyPath := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyPath, nil, 0644); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	sc, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch := make(chan hasher.FileInfo, 10)
	go func() {
		defer close(ch)
		if err := sc.Walk(dir, "testdisk", ch); err != nil {
			t.Errorf("Walk: %v", err)
		}
	}()

	var results []hasher.FileInfo
	for fi := range ch {
		results = append(results, fi)
	}

	// Should have 3 files (empty file skipped)
	if len(results) != 3 {
		t.Errorf("got %d files, want 3", len(results))
		for _, r := range results {
			t.Logf("  %s (size=%d)", r.Path, r.Size)
		}
	}

	// Verify all results have correct disk name and non-zero size/mtime
	for _, r := range results {
		if r.Disk != "testdisk" {
			t.Errorf("Disk = %q, want testdisk for %s", r.Disk, r.Path)
		}
		if r.Size <= 0 {
			t.Errorf("Size = %d for %s, want > 0", r.Size, r.Path)
		}
		if r.Mtime <= 0 {
			t.Errorf("Mtime = %d for %s, want > 0", r.Mtime, r.Path)
		}
	}
}

func TestWalkExclude(t *testing.T) {
	dir := t.TempDir()

	// Create files
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.tmp"), []byte("skip"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "also.tmp"), []byte("also skip"), 0644); err != nil {
		t.Fatal(err)
	}

	sc, err := New([]string{`\.tmp$`})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch := make(chan hasher.FileInfo, 10)
	go func() {
		defer close(ch)
		if err := sc.Walk(dir, "disk1", ch); err != nil {
			t.Errorf("Walk: %v", err)
		}
	}()

	var results []hasher.FileInfo
	for fi := range ch {
		results = append(results, fi)
	}

	if len(results) != 1 {
		t.Errorf("got %d files, want 1", len(results))
	}
	if len(results) > 0 && filepath.Base(results[0].Path) != "keep.txt" {
		t.Errorf("expected keep.txt, got %s", results[0].Path)
	}
}

func TestWalkExcludeDirectory(t *testing.T) {
	dir := t.TempDir()
	skipDir := filepath.Join(dir, "skipme")
	if err := os.MkdirAll(skipDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skipDir, "hidden.txt"), []byte("hidden"), 0644); err != nil {
		t.Fatal(err)
	}

	sc, err := New([]string{`skipme`})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch := make(chan hasher.FileInfo, 10)
	go func() {
		defer close(ch)
		if err := sc.Walk(dir, "disk1", ch); err != nil {
			t.Errorf("Walk: %v", err)
		}
	}()

	var results []hasher.FileInfo
	for fi := range ch {
		results = append(results, fi)
	}

	if len(results) != 1 {
		t.Errorf("got %d files, want 1", len(results))
	}
}
