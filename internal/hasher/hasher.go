package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
)

// Result holds the hashing result for a single file.
type Result struct {
	Path   string
	Disk   string
	Size   int64
	Mtime  int64
	SHA256 string
	Err    error
}

// FileInfo is the input to the hasher.
type FileInfo struct {
	Path  string
	Disk  string
	Size  int64
	Mtime int64
}

// Hasher provides parallel file hashing.
type Hasher struct {
	workers int
}

// New creates a Hasher with the given number of workers.
func New(workers int) *Hasher {
	if workers <= 0 {
		workers = 1
	}
	return &Hasher{workers: workers}
}

// HashFile hashes a single file and returns the result.
// It stats the file to get size and mtime. For callers that already have
// this info, use hashFileWithInfo instead via HashFiles.
func HashFile(path string) (*Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}

	h := sha256.New()
	buf := make([]byte, 1*1024*1024) // 1MB buffer
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return nil, fmt.Errorf("hash %s: %w", path, err)
	}

	return &Result{
		Path:   path,
		Size:   stat.Size(),
		Mtime:  stat.ModTime().Unix(),
		SHA256: hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// hashFileWithInfo hashes a file using pre-existing size/mtime from FileInfo,
// avoiding a redundant stat syscall.
func hashFileWithInfo(fi FileInfo) (*Result, error) {
	f, err := os.Open(fi.Path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", fi.Path, err)
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 1*1024*1024) // 1MB buffer
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return nil, fmt.Errorf("hash %s: %w", fi.Path, err)
	}

	return &Result{
		Path:   fi.Path,
		Disk:   fi.Disk,
		Size:   fi.Size,
		Mtime:  fi.Mtime,
		SHA256: hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// HashFiles hashes multiple files in parallel and sends results to the results channel.
// The caller should close the input channel when done adding files.
// The results channel is closed when all workers finish.
// When FileInfo includes Size/Mtime (from a prior stat), the redundant stat is skipped.
func (h *Hasher) HashFiles(files <-chan FileInfo, results chan<- Result) {
	var wg sync.WaitGroup

	for i := 0; i < h.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range files {
				var result *Result
				var err error
				if fi.Size > 0 || fi.Mtime > 0 {
					// Pre-existing stat info available — skip redundant stat
					result, err = hashFileWithInfo(fi)
				} else {
					result, err = HashFile(fi.Path)
					if result != nil {
						result.Disk = fi.Disk
					}
				}
				if err != nil {
					results <- Result{Path: fi.Path, Disk: fi.Disk, Err: err}
					continue
				}
				results <- *result
			}
		}()
	}

	wg.Wait()
	close(results)
}
