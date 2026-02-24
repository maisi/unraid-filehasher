package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maisi/unraid-filehasher/internal/db"
	"github.com/maisi/unraid-filehasher/internal/hasher"
	"github.com/maisi/unraid-filehasher/internal/scanner"
	"github.com/maisi/unraid-filehasher/internal/verifier"
)

// RunnerState represents the current operation state.
type RunnerState string

const (
	StateIdle      RunnerState = "idle"
	StateScanning  RunnerState = "scanning"
	StateVerifying RunnerState = "verifying"
)

// RunnerProgress holds current progress information.
type RunnerProgress struct {
	State   RunnerState `json:"state"`
	Phase   string      `json:"phase"`   // e.g., "walking", "hashing", "verifying"
	Done    int64       `json:"done"`    // files processed so far
	Total   int64       `json:"total"`   // total files (0 if unknown)
	Errors  int64       `json:"errors"`  // error count
	Started time.Time   `json:"started"` // when current op started
	Message string      `json:"message"` // latest status message
}

// Runner manages background scan and verify operations.
type Runner struct {
	db *db.DB

	mu       sync.RWMutex
	progress RunnerProgress

	// SSE subscribers
	subMu   sync.Mutex
	subs    map[uint64]chan RunnerProgress
	nextSub uint64
}

// NewRunner creates a Runner for background operations.
func NewRunner(database *db.DB) *Runner {
	return &Runner{
		db: database,
		progress: RunnerProgress{
			State: StateIdle,
		},
		subs: make(map[uint64]chan RunnerProgress),
	}
}

// Progress returns the current progress snapshot.
func (r *Runner) Progress() RunnerProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.progress
}

func (r *Runner) setProgress(p RunnerProgress) {
	r.mu.Lock()
	r.progress = p
	r.mu.Unlock()
	r.notify(p)
}

func (r *Runner) updateProgress(fn func(p *RunnerProgress)) {
	r.mu.Lock()
	fn(&r.progress)
	snap := r.progress
	r.mu.Unlock()
	r.notify(snap)
}

// Subscribe returns a channel that receives progress updates and an ID to unsubscribe.
func (r *Runner) Subscribe() (uint64, <-chan RunnerProgress) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	id := r.nextSub
	r.nextSub++
	ch := make(chan RunnerProgress, 16)
	r.subs[id] = ch
	return id, ch
}

// Unsubscribe removes a subscriber.
func (r *Runner) Unsubscribe(id uint64) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	if ch, ok := r.subs[id]; ok {
		close(ch)
		delete(r.subs, id)
	}
}

func (r *Runner) notify(p RunnerProgress) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	for _, ch := range r.subs {
		select {
		case ch <- p:
		default:
			// drop if subscriber is slow
		}
	}
}

// StartScan begins a background scan operation. Returns an error if already busy.
func (r *Runner) StartScan() error {
	r.mu.Lock()
	if r.progress.State != StateIdle {
		r.mu.Unlock()
		return fmt.Errorf("already running: %s", r.progress.State)
	}
	r.progress = RunnerProgress{
		State:   StateScanning,
		Phase:   "detecting disks",
		Started: time.Now(),
	}
	r.mu.Unlock()
	r.notify(r.Progress())

	go r.runScan()
	return nil
}

// StartVerify begins a background verify operation. Returns an error if already busy.
func (r *Runner) StartVerify() error {
	r.mu.Lock()
	if r.progress.State != StateIdle {
		r.mu.Unlock()
		return fmt.Errorf("already running: %s", r.progress.State)
	}
	r.progress = RunnerProgress{
		State:   StateVerifying,
		Phase:   "starting",
		Started: time.Now(),
	}
	r.mu.Unlock()
	r.notify(r.Progress())

	go r.runVerify()
	return nil
}

func (r *Runner) runScan() {
	defer r.setProgress(RunnerProgress{State: StateIdle})

	// Detect Unraid disks
	disks, err := scanner.DetectUnraidDisks()
	if err != nil || len(disks) == 0 {
		msg := "no Unraid disks detected"
		if err != nil {
			msg = fmt.Sprintf("disk detection failed: %v", err)
		}
		r.updateProgress(func(p *RunnerProgress) {
			p.Phase = "error"
			p.Message = msg
		})
		return
	}

	r.updateProgress(func(p *RunnerProgress) {
		names := make([]string, len(disks))
		for i, d := range disks {
			names[i] = d.Name
		}
		p.Phase = "walking"
		p.Message = fmt.Sprintf("Scanning %d disks: %s", len(disks), strings.Join(names, ", "))
	})

	// Load lookup map for incremental scan
	lookupMap, err := r.db.LoadQuickLookupMap()
	if err != nil {
		r.updateProgress(func(p *RunnerProgress) {
			p.Phase = "error"
			p.Message = fmt.Sprintf("load lookup map: %v", err)
		})
		return
	}

	// Create scanner with no excludes (web scan uses defaults)
	sc, err := scanner.New(nil)
	if err != nil {
		r.updateProgress(func(p *RunnerProgress) {
			p.Phase = "error"
			p.Message = fmt.Sprintf("create scanner: %v", err)
		})
		return
	}

	// Record scan history
	var pathNames []string
	for _, d := range disks {
		pathNames = append(pathNames, d.Name)
	}
	scanID, _ := r.db.InsertScanHistory("scan", strings.Join(pathNames, ","))

	// Aggregate result channel
	results := make(chan hasher.Result, 256)

	var skipped int64
	var totalProcessed int64
	var totalErrors int64

	// Launch per-disk pipelines
	var pipelineWg sync.WaitGroup
	for _, d := range disks {
		workers := d.Type.DefaultWorkers()
		diskInput := make(chan hasher.FileInfo, workers*4)
		output := make(chan hasher.Result, workers*4)

		h := hasher.New(workers)

		pipelineWg.Add(1)
		go func() {
			defer pipelineWg.Done()
			for res := range output {
				results <- res
			}
		}()

		go h.HashFiles(diskInput, output)

		disk := d
		go func() {
			defer close(diskInput)

			scanned := make(chan hasher.FileInfo, workers*4)
			go func() {
				defer close(scanned)
				if err := sc.Walk(disk.Path, disk.Name, scanned); err != nil {
					r.updateProgress(func(p *RunnerProgress) {
						p.Message = fmt.Sprintf("error scanning %s: %v", disk.Path, err)
					})
				}
			}()

			for fi := range scanned {
				// Incremental check
				if lookupMap != nil {
					if existing, ok := lookupMap[fi.Path]; ok {
						if existing.Size == fi.Size && existing.Mtime == fi.Mtime {
							atomic.AddInt64(&skipped, 1)
							continue
						}
					}
				}
				diskInput <- fi
			}
		}()
	}

	go func() {
		pipelineWg.Wait()
		close(results)
	}()

	r.updateProgress(func(p *RunnerProgress) {
		p.Phase = "hashing"
	})

	// Process results
	tx, txErr := r.db.BeginBatch()
	if txErr != nil {
		r.updateProgress(func(p *RunnerProgress) {
			p.Phase = "error"
			p.Message = fmt.Sprintf("begin transaction: %v", txErr)
		})
		return
	}
	defer func() { tx.Rollback() }()

	batchSize := 1000
	batchCount := 0

	for result := range results {
		atomic.AddInt64(&totalProcessed, 1)

		if result.Err != nil {
			atomic.AddInt64(&totalErrors, 1)
			continue
		}

		now := time.Now()
		record := &db.FileRecord{
			Path:         result.Path,
			Disk:         result.Disk,
			Size:         result.Size,
			Mtime:        result.Mtime,
			SHA256:       result.SHA256,
			FirstSeen:    now,
			LastVerified: now,
			Status:       "ok",
		}

		// Move detection
		if lookupMap != nil {
			if _, ok := lookupMap[result.Path]; !ok {
				base := filepath.Base(result.Path)
				cands, err := r.db.FindMoveCandidates(base, result.Size, 20)
				if err == nil {
					for _, cand := range cands {
						if cand.Path == result.Path {
							continue
						}
						_, statErr := os.Stat(cand.Path)
						if statErr == nil || !os.IsNotExist(statErr) {
							continue
						}
						if cand.SHA256 == result.SHA256 {
							if err := r.db.MovePathTx(tx, cand.Path, result.Path, result.Disk, result.Size, result.Mtime); err != nil {
								atomic.AddInt64(&totalErrors, 1)
							}
							record = nil
							break
						}
						record.Status = "corrupted"
						break
					}
				}
			}
		}

		if record != nil {
			if err := r.db.UpsertFileTx(tx, record); err != nil {
				atomic.AddInt64(&totalErrors, 1)
			}
		}

		batchCount++
		if batchCount >= batchSize {
			if err := tx.Commit(); err != nil {
				r.updateProgress(func(p *RunnerProgress) {
					p.Phase = "error"
					p.Message = fmt.Sprintf("commit batch: %v", err)
				})
				return
			}
			tx, txErr = r.db.BeginBatch()
			if txErr != nil {
				r.updateProgress(func(p *RunnerProgress) {
					p.Phase = "error"
					p.Message = fmt.Sprintf("begin new batch: %v", txErr)
				})
				return
			}
			batchCount = 0
		}

		// Update progress periodically (every 50 files to reduce lock contention)
		processed := atomic.LoadInt64(&totalProcessed)
		if processed%50 == 0 {
			errors := atomic.LoadInt64(&totalErrors)
			skip := atomic.LoadInt64(&skipped)
			r.updateProgress(func(p *RunnerProgress) {
				p.Done = processed
				p.Errors = errors
				p.Message = fmt.Sprintf("Hashed %d files, skipped %d, %d errors", processed, skip, errors)
			})
		}
	}

	// Commit remaining
	if batchCount > 0 {
		if err := tx.Commit(); err != nil {
			r.updateProgress(func(p *RunnerProgress) {
				p.Phase = "error"
				p.Message = fmt.Sprintf("commit final batch: %v", err)
			})
			return
		}
	}

	finalProcessed := atomic.LoadInt64(&totalProcessed)
	finalErrors := atomic.LoadInt64(&totalErrors)
	finalSkipped := atomic.LoadInt64(&skipped)

	if scanID > 0 {
		r.db.CompleteScanHistory(scanID, int(finalProcessed), int(finalErrors))
	}

	elapsed := time.Since(r.Progress().Started)
	r.updateProgress(func(p *RunnerProgress) {
		p.Phase = "complete"
		p.Done = finalProcessed
		p.Errors = finalErrors
		p.Message = fmt.Sprintf("Scan complete: %d hashed, %d skipped, %d errors in %s",
			finalProcessed, finalSkipped, finalErrors, elapsed.Round(time.Second))
	})
}

func (r *Runner) runVerify() {
	defer r.setProgress(RunnerProgress{State: StateIdle})

	scanID, _ := r.db.InsertScanHistory("verify", "")
	v := verifier.New(r.db, 4, false)

	resultCb := func(vr verifier.VerifyResult) {
		// No-op for web — progress is tracked via progressCb
	}

	progressCb := func(done, total int) {
		r.updateProgress(func(p *RunnerProgress) {
			p.Phase = "verifying"
			p.Done = int64(done)
			p.Total = int64(total)
			p.Message = fmt.Sprintf("Verified %d / %d files", done, total)
		})
	}

	summary, err := v.VerifyAll(resultCb, progressCb)
	if err != nil {
		r.updateProgress(func(p *RunnerProgress) {
			p.Phase = "error"
			p.Message = fmt.Sprintf("verify failed: %v", err)
		})
		return
	}

	if scanID > 0 {
		r.db.CompleteScanHistory(scanID, summary.TotalChecked, summary.Errors)
	}

	r.updateProgress(func(p *RunnerProgress) {
		p.Phase = "complete"
		p.Done = int64(summary.TotalChecked)
		p.Total = int64(summary.TotalChecked)
		p.Errors = int64(summary.Errors)
		p.Message = fmt.Sprintf("Verify complete: %d checked, %d OK, %d corrupted, %d missing in %s",
			summary.TotalChecked, summary.OK, summary.Corrupted, summary.Missing,
			summary.Duration.Round(time.Second))
	})
}
