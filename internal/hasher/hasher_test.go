package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestHashFile(t *testing.T) {
	// Create a temp file with known content
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	result, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	// Compute expected SHA-256
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	if result.SHA256 != expected {
		t.Errorf("SHA256 = %q, want %q", result.SHA256, expected)
	}
	if result.Path != path {
		t.Errorf("Path = %q, want %q", result.Path, path)
	}
	if result.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", result.Size, len(content))
	}
}

func TestHashFileNotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestHashFileDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := HashFile(dir)
	if err == nil {
		t.Fatal("expected error for directory, got nil")
	}
}

func TestHashFileWithInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("test content for hashFileWithInfo\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	fi := FileInfo{
		Path:  path,
		Disk:  "testdisk",
		Size:  stat.Size(),
		Mtime: stat.ModTime().Unix(),
	}

	result, err := hashFileWithInfo(fi)
	if err != nil {
		t.Fatalf("hashFileWithInfo: %v", err)
	}

	// Verify it used the pre-existing info
	if result.Disk != "testdisk" {
		t.Errorf("Disk = %q, want %q", result.Disk, "testdisk")
	}
	if result.Size != fi.Size {
		t.Errorf("Size = %d, want %d", result.Size, fi.Size)
	}
	if result.Mtime != fi.Mtime {
		t.Errorf("Mtime = %d, want %d", result.Mtime, fi.Mtime)
	}

	// Verify hash matches HashFile
	result2, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if result.SHA256 != result2.SHA256 {
		t.Errorf("hashFileWithInfo SHA256 = %q, HashFile SHA256 = %q", result.SHA256, result2.SHA256)
	}
}

func TestHashFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 5 test files
	var expected []FileInfo
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, "file"+string(rune('a'+i))+".txt")
		content := []byte("content " + string(rune('0'+i)) + "\n")
		if err := os.WriteFile(name, content, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		stat, _ := os.Stat(name)
		expected = append(expected, FileInfo{
			Path:  name,
			Disk:  "disk1",
			Size:  stat.Size(),
			Mtime: stat.ModTime().Unix(),
		})
	}

	input := make(chan FileInfo, 10)
	output := make(chan Result, 10)
	h := New(2)

	go h.HashFiles(input, output)

	for _, fi := range expected {
		input <- fi
	}
	close(input)

	results := make(map[string]Result)
	for r := range output {
		results[r.Path] = r
	}

	if len(results) != 5 {
		t.Fatalf("got %d results, want 5", len(results))
	}

	for _, fi := range expected {
		r, ok := results[fi.Path]
		if !ok {
			t.Errorf("missing result for %s", fi.Path)
			continue
		}
		if r.Err != nil {
			t.Errorf("error for %s: %v", fi.Path, r.Err)
		}
		if r.SHA256 == "" {
			t.Errorf("empty SHA256 for %s", fi.Path)
		}
		if r.Disk != "disk1" {
			t.Errorf("Disk = %q, want disk1 for %s", r.Disk, fi.Path)
		}
	}
}

func TestHashFilesWithError(t *testing.T) {
	input := make(chan FileInfo, 1)
	output := make(chan Result, 1)
	h := New(1)

	go h.HashFiles(input, output)
	input <- FileInfo{Path: "/nonexistent/file.txt", Disk: "disk1"}
	close(input)

	r := <-output
	if r.Err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if r.Path != "/nonexistent/file.txt" {
		t.Errorf("Path = %q, want /nonexistent/file.txt", r.Path)
	}
	if r.Disk != "disk1" {
		t.Errorf("Disk = %q, want disk1", r.Disk)
	}
}
