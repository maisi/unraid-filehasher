package web

import (
	"context"
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

// DiskProgress holds progress information for a single disk.
type DiskProgress struct {
	Disk       string `json:"disk"`
	Phase      string `json:"phase"`      // "walking", "hashing", "complete", "cancelled"
	FilesFound int64  `json:"filesFound"` // files discovered during walk (that need hashing)
	FilesDone  int64  `json:"filesDone"`  // files hashed/verified so far
	BytesTotal int64  `json:"bytesTotal"` // total bytes to hash (accumulated during walk)
	BytesDone  int64  `json:"bytesDone"`  // bytes hashed so far
}

// RunnerProgress holds current progress information.
type RunnerProgress struct {
	State   RunnerState    `json:"state"`
	Phase   string         `json:"phase"`   // e.g., "walking", "hashing", "verifying", "complete", "cancelled", "error"
	Done    int64          `json:"done"`    // files processed so far
	Total   int64          `json:"total"`   // total files (0 if unknown)
	Errors  int64          `json:"errors"`  // error count
	Started time.Time      `json:"started"` // when current op started
	Message string         `json:"message"` // latest status message
	Disks   []DiskProgress `json:"disks"`   // per-disk progress (nil when idle with no history)
}

// Runner manages background scan and verify operations.
type Runner struct {
	db *db.DB

	mu       sync.RWMutex
	progress RunnerProgress
	cancel   context.CancelFunc // cancel function for current operation

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

// Stop cancels the currently running operation.
func (r *Runner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.progress.State == StateIdle {
		return fmt.Errorf("no operation running")
	}
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

// StartScan begins a background scan operation. Returns an error if already busy.
func (r *Runner) StartScan() error {
	r.mu.Lock()
	if r.progress.State != StateIdle {
		r.mu.Unlock()
		return fmt.Errorf("already running: %s", r.progress.State)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.progress = RunnerProgress{
		State:   StateScanning,
		Phase:   "detecting disks",
		Started: time.Now(),
	}
	r.mu.Unlock()
	r.notify(r.Progress())

	go r.runScan(ctx)
	return nil
}

// StartVerify begins a background verify operation. Returns an error if already busy.
func (r *Runner) StartVerify() error {
	r.mu.Lock()
	if r.progress.State != StateIdle {
		r.mu.Unlock()
		return fmt.Errorf("already running: %s", r.progress.State)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.progress = RunnerProgress{
		State:   StateVerifying,
		Phase:   "starting",
		Started: time.Now(),
	}
	r.mu.Unlock()
	r.notify(r.Progress())

	go r.runVerify(ctx)
	return nil
}

// finishOperation sets the final progress state. Phase should be "complete", "cancelled", or "error".
// The state is set to idle but phase/message/disks are preserved so the frontend can display them.
func (r *Runner) finishOperation(phase string, done, total, errors int64, message string, disks []DiskProgress) {
	r.mu.Lock()
	r.cancel = nil
	r.progress = RunnerProgress{
		State:   StateIdle,
		Phase:   phase,
		Done:    done,
		Total:   total,
		Errors:  errors,
		Message: message,
		Disks:   disks,
	}
	snap := r.progress
	r.mu.Unlock()
	r.notify(snap)
}

func (r *Runner) runScan(ctx context.Context) {
	// Detect Unraid disks
	disks, err := scanner.DetectUnraidDisks()
	if err != nil || len(disks) == 0 {
		msg := "no Unraid disks detected"
		if err != nil {
			msg = fmt.Sprintf("disk detection failed: %v", err)
		}
		r.finishOperation("error", 0, 0, 0, msg, nil)
		return
	}

	// Initialize per-disk progress
	diskProgressMap := make(map[string]*DiskProgress, len(disks))
	diskProgressList := make([]DiskProgress, len(disks))
	for i, d := range disks {
		diskProgressList[i] = DiskProgress{
			Disk:  d.Name,
			Phase: "walking",
		}
		diskProgressMap[d.Name] = &diskProgressList[i]
	}

	names := make([]string, len(disks))
	for i, d := range disks {
		names[i] = d.Name
	}

	r.updateProgress(func(p *RunnerProgress) {
		p.Phase = "walking"
		p.Message = fmt.Sprintf("Scanning %d disks: %s", len(disks), strings.Join(names, ", "))
		p.Disks = cloneDiskProgress(diskProgressList)
	})

	// Check for cancellation
	select {
	case <-ctx.Done():
		r.finishOperation("cancelled", 0, 0, 0, "Scan cancelled during disk detection", cloneDiskProgress(diskProgressList))
		return
	default:
	}

	// Load lookup map for incremental scan
	lookupMap, err := r.db.LoadQuickLookupMap()
	if err != nil {
		r.finishOperation("error", 0, 0, 0, fmt.Sprintf("load lookup map: %v", err), nil)
		return
	}

	// Create scanner with no excludes (web scan uses defaults)
	sc, err := scanner.New(nil)
	if err != nil {
		r.finishOperation("error", 0, 0, 0, fmt.Sprintf("create scanner: %v", err), nil)
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

		go h.HashFilesContext(ctx, diskInput, output)

		disk := d
		dp := diskProgressMap[disk.Name]
		go func() {
			defer close(diskInput)

			scanned := make(chan hasher.FileInfo, workers*4)
			go func() {
				defer close(scanned)
				if err := sc.WalkContext(ctx, disk.Path, disk.Name, scanned); err != nil {
					r.updateProgress(func(p *RunnerProgress) {
						p.Message = fmt.Sprintf("error scanning %s: %v", disk.Path, err)
					})
				}
			}()

			for fi := range scanned {
				// Check for cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Incremental check
				if lookupMap != nil {
					if existing, ok := lookupMap[fi.Path]; ok {
						if existing.Size == fi.Size && existing.Mtime == fi.Mtime {
							atomic.AddInt64(&skipped, 1)
							continue
						}
					}
				}

				// Track per-disk walk progress
				atomic.AddInt64(&dp.FilesFound, 1)
				atomic.AddInt64(&dp.BytesTotal, fi.Size)

				diskInput <- fi
			}

			// Mark disk walk as complete, transition to hashing
			dp.Phase = "hashing"
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
		r.finishOperation("error", 0, 0, 0, fmt.Sprintf("begin transaction: %v", txErr), nil)
		return
	}
	defer func() { tx.Rollback() }()

	batchSize := 1000
	batchCount := 0

	cancelled := false
	for result := range results {
		// Check for cancellation
		select {
		case <-ctx.Done():
			cancelled = true
			// Drain remaining results to unblock goroutines
			for range results {
			}
			break
		default:
		}
		if cancelled {
			break
		}

		atomic.AddInt64(&totalProcessed, 1)

		if result.Err != nil {
			atomic.AddInt64(&totalErrors, 1)
			continue
		}

		// Track per-disk hash progress
		if dp, ok := diskProgressMap[result.Disk]; ok {
			atomic.AddInt64(&dp.FilesDone, 1)
			atomic.AddInt64(&dp.BytesDone, result.Size)
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
				r.finishOperation("error", atomic.LoadInt64(&totalProcessed), 0, atomic.LoadInt64(&totalErrors),
					fmt.Sprintf("commit batch: %v", err), cloneDiskProgress(diskProgressList))
				return
			}
			tx, txErr = r.db.BeginBatch()
			if txErr != nil {
				r.finishOperation("error", atomic.LoadInt64(&totalProcessed), 0, atomic.LoadInt64(&totalErrors),
					fmt.Sprintf("begin new batch: %v", txErr), cloneDiskProgress(diskProgressList))
				return
			}
			batchCount = 0
		}

		// Update progress periodically (every 50 files to reduce lock contention)
		processed := atomic.LoadInt64(&totalProcessed)
		if processed%50 == 0 {
			errors := atomic.LoadInt64(&totalErrors)
			skip := atomic.LoadInt64(&skipped)

			// Mark disks as complete if all their files are done
			for i := range diskProgressList {
				dp := &diskProgressList[i]
				if dp.Phase == "hashing" && dp.FilesFound > 0 && dp.FilesDone >= dp.FilesFound {
					dp.Phase = "complete"
				}
			}

			r.updateProgress(func(p *RunnerProgress) {
				p.Done = processed
				p.Errors = errors
				p.Disks = cloneDiskProgress(diskProgressList)
				p.Message = fmt.Sprintf("Hashed %d files, skipped %d, %d errors", processed, skip, errors)
			})
		}
	}

	// Handle cancellation
	if cancelled {
		// Commit what we have so far
		if batchCount > 0 {
			tx.Commit()
		}
		finalProcessed := atomic.LoadInt64(&totalProcessed)
		finalErrors := atomic.LoadInt64(&totalErrors)
		finalSkipped := atomic.LoadInt64(&skipped)
		elapsed := time.Since(r.Progress().Started)

		if scanID > 0 {
			r.db.CompleteScanHistory(scanID, int(finalProcessed), int(finalErrors))
		}

		for i := range diskProgressList {
			if diskProgressList[i].Phase != "complete" {
				diskProgressList[i].Phase = "cancelled"
			}
		}

		r.finishOperation("cancelled", finalProcessed, 0, finalErrors,
			fmt.Sprintf("Scan cancelled: %d hashed, %d skipped, %d errors in %s",
				finalProcessed, finalSkipped, finalErrors, elapsed.Round(time.Second)),
			cloneDiskProgress(diskProgressList))
		return
	}

	// Commit remaining
	if batchCount > 0 {
		if err := tx.Commit(); err != nil {
			r.finishOperation("error", atomic.LoadInt64(&totalProcessed), 0, atomic.LoadInt64(&totalErrors),
				fmt.Sprintf("commit final batch: %v", err), cloneDiskProgress(diskProgressList))
			return
		}
	}

	finalProcessed := atomic.LoadInt64(&totalProcessed)
	finalErrors := atomic.LoadInt64(&totalErrors)
	finalSkipped := atomic.LoadInt64(&skipped)

	if scanID > 0 {
		r.db.CompleteScanHistory(scanID, int(finalProcessed), int(finalErrors))
	}

	// Mark all disks as complete
	for i := range diskProgressList {
		diskProgressList[i].Phase = "complete"
	}

	elapsed := time.Since(r.Progress().Started)
	r.finishOperation("complete", finalProcessed, finalProcessed, finalErrors,
		fmt.Sprintf("Scan complete: %d hashed, %d skipped, %d errors in %s",
			finalProcessed, finalSkipped, finalErrors, elapsed.Round(time.Second)),
		cloneDiskProgress(diskProgressList))
}

func (r *Runner) runVerify(ctx context.Context) {
	scanID, _ := r.db.InsertScanHistory("verify", "")
	v := verifier.New(r.db, 4, false)

	// Load files to build per-disk progress
	allFiles, err := r.db.GetAllFiles()
	if err != nil {
		r.finishOperation("error", 0, 0, 0, fmt.Sprintf("load files: %v", err), nil)
		return
	}

	// Build per-disk counters
	diskFileCounts := make(map[string]int64)
	diskByteCounts := make(map[string]int64)
	for _, f := range allFiles {
		diskFileCounts[f.Disk]++
		diskByteCounts[f.Disk] += f.Size
	}

	// Create disk progress list
	diskProgressList := make([]DiskProgress, 0, len(diskFileCounts))
	diskProgressMap := make(map[string]*DiskProgress, len(diskFileCounts))
	for disk, count := range diskFileCounts {
		diskProgressList = append(diskProgressList, DiskProgress{
			Disk:       disk,
			Phase:      "verifying",
			FilesFound: count,
			BytesTotal: diskByteCounts[disk],
		})
		diskProgressMap[disk] = &diskProgressList[len(diskProgressList)-1]
	}

	total := int64(len(allFiles))

	r.updateProgress(func(p *RunnerProgress) {
		p.Phase = "verifying"
		p.Total = total
		p.Disks = cloneDiskProgress(diskProgressList)
		p.Message = fmt.Sprintf("Verifying %d files across %d disks", total, len(diskFileCounts))
	})

	resultCb := func(vr verifier.VerifyResult) {
		// No-op for web — progress is tracked via progressCb
	}

	var verifyDone int64
	progressCb := func(done, total int) {
		atomic.StoreInt64(&verifyDone, int64(done))

		// Update periodically (every 50 files)
		if done%50 == 0 || done == total {
			r.updateProgress(func(p *RunnerProgress) {
				p.Phase = "verifying"
				p.Done = int64(done)
				p.Total = int64(total)
				p.Disks = cloneDiskProgress(diskProgressList)
				p.Message = fmt.Sprintf("Verified %d / %d files", done, total)
			})
		}
	}

	summary, err := v.VerifyAllContext(ctx, resultCb, progressCb)

	if err != nil {
		// Check if it was a cancellation
		if ctx.Err() != nil {
			finalDone := atomic.LoadInt64(&verifyDone)
			elapsed := time.Since(r.Progress().Started)

			for i := range diskProgressList {
				if diskProgressList[i].Phase != "complete" {
					diskProgressList[i].Phase = "cancelled"
				}
			}

			if scanID > 0 && summary != nil {
				r.db.CompleteScanHistory(scanID, summary.TotalChecked, summary.Errors)
			}

			r.finishOperation("cancelled", finalDone, total, 0,
				fmt.Sprintf("Verify cancelled: %d / %d checked in %s",
					finalDone, total, elapsed.Round(time.Second)),
				cloneDiskProgress(diskProgressList))
			return
		}

		r.finishOperation("error", 0, 0, 0, fmt.Sprintf("verify failed: %v", err), nil)
		return
	}

	if scanID > 0 {
		r.db.CompleteScanHistory(scanID, summary.TotalChecked, summary.Errors)
	}

	// Mark all disks as complete
	for i := range diskProgressList {
		diskProgressList[i].Phase = "complete"
	}

	r.finishOperation("complete", int64(summary.TotalChecked), int64(summary.TotalChecked), int64(summary.Errors),
		fmt.Sprintf("Verify complete: %d checked, %d OK, %d corrupted, %d missing in %s",
			summary.TotalChecked, summary.OK, summary.Corrupted, summary.Missing,
			summary.Duration.Round(time.Second)),
		cloneDiskProgress(diskProgressList))
}

// cloneDiskProgress creates a snapshot of the disk progress slice for safe concurrent access.
func cloneDiskProgress(src []DiskProgress) []DiskProgress {
	if src == nil {
		return nil
	}
	dst := make([]DiskProgress, len(src))
	for i := range src {
		dst[i] = DiskProgress{
			Disk:       src[i].Disk,
			Phase:      src[i].Phase,
			FilesFound: atomic.LoadInt64(&src[i].FilesFound),
			FilesDone:  atomic.LoadInt64(&src[i].FilesDone),
			BytesTotal: atomic.LoadInt64(&src[i].BytesTotal),
			BytesDone:  atomic.LoadInt64(&src[i].BytesDone),
		}
	}
	return dst
}
