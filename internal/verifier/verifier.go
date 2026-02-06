package verifier

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/filehasher/filehasher/internal/db"
	"github.com/filehasher/filehasher/internal/hasher"
)

// VerifyResult represents the outcome of verifying a single file.
type VerifyResult struct {
	Path    string
	Status  string // ok, corrupted, missing
	OldHash string
	NewHash string
	Err     error
}

// Summary holds aggregated verification results.
type Summary struct {
	TotalChecked int
	OK           int
	Corrupted    int
	Missing      int
	Skipped      int
	Errors       int
	Duration     time.Duration
}

// Verifier checks files against their stored hashes.
type Verifier struct {
	db      *db.DB
	workers int
	quick   bool // only check files with changed mtime/size
}

// New creates a new Verifier.
func New(database *db.DB, workers int, quick bool) *Verifier {
	if workers <= 0 {
		workers = 4
	}
	return &Verifier{
		db:      database,
		workers: workers,
		quick:   quick,
	}
}

// VerifyAll verifies all tracked files and returns a summary.
func (v *Verifier) VerifyAll(resultCb func(VerifyResult)) (*Summary, error) {
	files, err := v.db.GetAllFiles()
	if err != nil {
		return nil, fmt.Errorf("get files: %w", err)
	}
	return v.verifyFiles(files, resultCb)
}

// VerifyDisk verifies all tracked files on a specific disk.
func (v *Verifier) VerifyDisk(disk string, resultCb func(VerifyResult)) (*Summary, error) {
	files, err := v.db.GetFilesByDisk(disk)
	if err != nil {
		return nil, fmt.Errorf("get files for disk %s: %w", disk, err)
	}
	return v.verifyFiles(files, resultCb)
}

func (v *Verifier) verifyFiles(files []*db.FileRecord, resultCb func(VerifyResult)) (*Summary, error) {
	start := time.Now()
	summary := &Summary{}

	// Set up the parallel hasher
	input := make(chan hasher.FileInfo, v.workers*2)
	output := make(chan hasher.Result, v.workers*2)

	h := hasher.New(v.workers)

	// Build a lookup map from path to stored record
	storedMap := make(map[string]*db.FileRecord, len(files))
	for _, f := range files {
		storedMap[f.Path] = f
	}

	// Start the hasher in a goroutine
	go h.HashFiles(input, output)

	// Track files the feeder determined are missing (avoids double stat later)
	var missingPaths []string
	var missingMu sync.Mutex
	var skippedCount atomic.Int64

	// Feed files to the hasher
	go func() {
		defer close(input)
		for _, f := range files {
			// Check if file still exists
			stat, err := os.Stat(f.Path)
			if err != nil {
				if os.IsNotExist(err) {
					// Track missing files for post-pipeline processing
					missingMu.Lock()
					missingPaths = append(missingPaths, f.Path)
					missingMu.Unlock()
					continue
				}
				continue
			}

			// In quick mode, skip files whose mtime and size haven't changed
			if v.quick && stat.ModTime().Unix() == f.Mtime && stat.Size() == f.Size {
				skippedCount.Add(1)
				continue
			}

			input <- hasher.FileInfo{Path: f.Path, Disk: f.Disk}
		}
	}()

	// Begin a transaction for batch updates
	tx, err := v.db.BeginBatch()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() // no-op after commit, prevents resource leak

	// Collect results
	for result := range output {
		summary.TotalChecked++

		stored := storedMap[result.Path]
		if stored == nil {
			continue
		}

		var vr VerifyResult
		vr.Path = result.Path
		vr.OldHash = stored.SHA256

		if result.Err != nil {
			vr.Status = "corrupted"
			vr.Err = result.Err
			summary.Errors++
			summary.Corrupted++
			if err := v.db.UpdateStatusTx(tx, result.Path, "corrupted"); err != nil {
				fmt.Fprintf(os.Stderr, "warning: update status for %s: %v\n", result.Path, err)
				summary.Errors++
			}
		} else {
			vr.NewHash = result.SHA256
			if result.SHA256 == stored.SHA256 {
				vr.Status = "ok"
				summary.OK++
				if err := v.db.UpdateStatusTx(tx, result.Path, "ok"); err != nil {
					fmt.Fprintf(os.Stderr, "warning: update status for %s: %v\n", result.Path, err)
					summary.Errors++
				}
			} else {
				vr.Status = "corrupted"
				summary.Corrupted++
				if err := v.db.UpdateStatusTx(tx, result.Path, "corrupted"); err != nil {
					fmt.Fprintf(os.Stderr, "warning: update status for %s: %v\n", result.Path, err)
					summary.Errors++
				}
			}
		}

		if resultCb != nil {
			resultCb(vr)
		}
	}

	// Process missing files identified by the feeder goroutine (no re-stat needed)
	missingMu.Lock()
	for _, path := range missingPaths {
		summary.TotalChecked++
		summary.Missing++
		if err := v.db.UpdateStatusTx(tx, path, "missing"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: update status for %s: %v\n", path, err)
			summary.Errors++
		}

		if resultCb != nil {
			stored := storedMap[path]
			oldHash := ""
			if stored != nil {
				oldHash = stored.SHA256
			}
			resultCb(VerifyResult{
				Path:    path,
				Status:  "missing",
				OldHash: oldHash,
			})
		}
	}
	missingMu.Unlock()

	// Assign atomic skipped count to summary (safe: feeder goroutine has finished by now)
	summary.Skipped = int(skippedCount.Load())

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	summary.Duration = time.Since(start)
	return summary, nil
}
