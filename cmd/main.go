package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maisi/unraid-filehasher/internal/db"
	"github.com/maisi/unraid-filehasher/internal/format"
	"github.com/maisi/unraid-filehasher/internal/hasher"
	"github.com/maisi/unraid-filehasher/internal/scanner"
	"github.com/maisi/unraid-filehasher/internal/verifier"
	"github.com/maisi/unraid-filehasher/internal/web"
	"github.com/spf13/cobra"
)

var (
	version  = "dev"
	dbPath   string
	jsonOut  bool
	excludes []string
)

func defaultDBPath() string {
	// On Unraid, prefer the USB boot drive for persistence
	if _, err := os.Stat("/boot/config"); err == nil {
		dir := "/boot/config/filehasher"
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "warning: create config dir %s: %v\n", dir, err)
			return "filehasher.db"
		}
		return filepath.Join(dir, "filehasher.db")
	}
	// Fallback to current directory
	return "filehasher.db"
}

func main() {
	rootCmd := &cobra.Command{
		Use:     "filehasher",
		Short:   "File integrity checker for Unraid servers",
		Long:    "filehasher catalogs, hashes, and verifies file integrity across Unraid array disks and cache pools.",
		Version: version,
	}

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath(), "path to SQLite database")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output results as JSON")
	rootCmd.PersistentFlags().StringSliceVarP(&excludes, "exclude", "e", nil, "regex patterns to exclude (can be repeated)")

	rootCmd.AddCommand(scanCmd())
	rootCmd.AddCommand(verifyCmd())
	rootCmd.AddCommand(reportCmd())
	rootCmd.AddCommand(serverCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func scanCmd() *cobra.Command {
	var autoDetect bool
	var fullScan bool

	cmd := &cobra.Command{
		Use:   "scan [paths...]",
		Short: "Scan directories and hash all files",
		Long: `Walk the specified directories (or auto-detect Unraid disks), hash every file,
and store results in the catalog database.

By default, incremental mode is used: files whose size and mtime haven't
changed since the last scan are skipped. Use --full to force re-hashing
every file.

When using --auto, each disk gets its own hashing pipeline with worker
counts tuned to the disk type (1 worker for HDDs, 4 for SSDs).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine scan targets
			var disks []scanner.DiskInfo

			if autoDetect {
				detected, err := scanner.DetectUnraidDisks()
				if err != nil {
					return fmt.Errorf("auto-detect disks: %w", err)
				}
				if len(detected) == 0 {
					return fmt.Errorf("no Unraid disks detected under /mnt/")
				}
				disks = detected
				for _, d := range disks {
					fmt.Printf("Detected: %s (%s, %s, %d workers)\n",
						d.Name, d.Path, d.Type, d.Type.DefaultWorkers())
				}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("no paths specified; use --auto or provide paths as arguments")
				}
				for _, p := range args {
					absPath, err := filepath.Abs(p)
					if err != nil {
						return fmt.Errorf("resolve path %s: %w", p, err)
					}
					name := scanner.ResolveDisk(absPath, absPath)
					disks = append(disks, scanner.DiskInfo{
						Name: name,
						Path: absPath,
						Type: scanner.DiskTypeUnknown,
					})
				}
			}

			// Open database
			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			// Load existing file index for incremental scan
			var lookupMap map[string]*db.QuickLookup
			if !fullScan {
				lookupMap, err = database.LoadQuickLookupMap()
				if err != nil {
					return fmt.Errorf("load lookup map: %w", err)
				}
				if !jsonOut {
					fmt.Printf("Loaded %d existing file records for incremental comparison\n", len(lookupMap))
				}
			}

			// Create scanner
			sc, err := scanner.New(excludes)
			if err != nil {
				return err
			}

			// Record scan history
			var pathNames []string
			for _, d := range disks {
				pathNames = append(pathNames, d.Name)
			}
			scanID, err := database.InsertScanHistory("scan", strings.Join(pathNames, ","))
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to record scan history: %v\n", err)
			}

			start := time.Now()

			// Aggregate result channel — all disk pipelines feed into this
			results := make(chan hasher.Result, 256)

			// Track scan errors safely
			var scanErrors []string
			var scanErrMu sync.Mutex

			// Counters
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

				// Forward disk pipeline output to aggregate results channel
				pipelineWg.Add(1)
				go func() {
					defer pipelineWg.Done()
					for r := range output {
						results <- r
					}
				}()

				// Start hasher workers for this disk
				go h.HashFiles(diskInput, output)

				// Start scanner goroutine for this disk.
				// In incremental mode, filter out unchanged files before hashing.
				disk := d // capture loop variable
				go func() {
					defer close(diskInput)

					// Intermediate channel: scanner writes here, we filter before sending to hasher
					scanned := make(chan hasher.FileInfo, workers*4)
					go func() {
						defer close(scanned)
						err := sc.Walk(disk.Path, disk.Name, scanned)
						if err != nil {
							scanErrMu.Lock()
							scanErrors = append(scanErrors, fmt.Sprintf("%s: %v", disk.Name, err))
							scanErrMu.Unlock()
							fmt.Fprintf(os.Stderr, "error scanning %s: %v\n", disk.Path, err)
						}
					}()

					for fi := range scanned {
						// Incremental check: skip if file hasn't changed since last scan
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

			// Close aggregate results channel when all disk pipelines finish
			go func() {
				pipelineWg.Wait()
				close(results)
			}()

			// Process results from all disks
			tx, txErr := database.BeginBatch()
			if txErr != nil {
				return fmt.Errorf("begin transaction: %w", txErr)
			}
			defer func() { tx.Rollback() }() // closure captures tx by reference; rolls back whichever tx is current

			batchSize := 1000
			batchCount := 0

			for result := range results {
				atomic.AddInt64(&totalProcessed, 1)
				processed := atomic.LoadInt64(&totalProcessed)

				if result.Err != nil {
					atomic.AddInt64(&totalErrors, 1)
					fmt.Fprintf(os.Stderr, "error: %s: %v\n", result.Path, result.Err)
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

				// Safe move detection (helps with rebalancing):
				// If this looks like a new path, try to find an older record with the same basename+size.
				// If the old path is gone and the SHA matches, re-key the DB entry to the new path.
				if lookupMap != nil {
					if _, ok := lookupMap[result.Path]; !ok {
						base := filepath.Base(result.Path)
						cands, err := database.FindMoveCandidates(base, result.Size, 20)
						if err == nil {
							for _, cand := range cands {
								if cand.Path == result.Path {
									continue
								}
								// Only treat as moved if the old path is actually gone
								_, statErr := os.Stat(cand.Path)
								if statErr == nil {
									continue
								}
								if !os.IsNotExist(statErr) {
									continue
								}

								if cand.SHA256 == result.SHA256 {
									if err := database.MovePathTx(tx, cand.Path, result.Path, result.Disk, result.Size, result.Mtime); err != nil {
										atomic.AddInt64(&totalErrors, 1)
										fmt.Fprintf(os.Stderr, "error moving record %s -> %s: %v\n", cand.Path, result.Path, err)
									} else {
										// Re-keyed successfully; skip normal upsert
										record = nil
									}
									break
								}

								// Likely moved-but-changed: basename+size match, old path missing, but SHA differs.
								// Flag the new path as corrupted and log a loud warning.
								fmt.Fprintf(os.Stderr, "warning: possible move corruption: %s -> %s (size=%d, oldSHA=%s..., newSHA=%s...)\n",
									cand.Path, result.Path, result.Size, cand.SHA256[:12], result.SHA256[:12])
								record.Status = "corrupted"
								break
							}
						}
					}
				}
				// If record was re-keyed, do not upsert a duplicate.
				if record != nil {
					if err := database.UpsertFileTx(tx, record); err != nil {
						atomic.AddInt64(&totalErrors, 1)
						fmt.Fprintf(os.Stderr, "error storing %s: %v\n", result.Path, err)
					}
				}

				batchCount++
				if batchCount >= batchSize {
					if err := tx.Commit(); err != nil {
						return fmt.Errorf("commit batch: %w", err)
					}
					tx, txErr = database.BeginBatch()
					if txErr != nil {
						return fmt.Errorf("begin new batch: %w", txErr)
					}
					batchCount = 0
				}

				// Progress output
				if !jsonOut && processed%100 == 0 {
					elapsed := time.Since(start)
					rate := float64(processed) / elapsed.Seconds()
					fmt.Printf("\r  Processed: %d files (%.0f files/sec)", processed, rate)
				}
			}

			// Commit remaining
			if batchCount > 0 {
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("commit final batch: %w", err)
				}
			}

			elapsed := time.Since(start)
			finalProcessed := int(atomic.LoadInt64(&totalProcessed))
			finalErrors := int(atomic.LoadInt64(&totalErrors))
			finalSkipped := int(atomic.LoadInt64(&skipped))

			// Update scan history
			if scanID > 0 {
				if err := database.CompleteScanHistory(scanID, finalProcessed, finalErrors); err != nil {
					fmt.Fprintf(os.Stderr, "warning: complete scan history: %v\n", err)
				}
			}

			if jsonOut {
				out := map[string]interface{}{
					"files_processed": finalProcessed,
					"files_skipped":   finalSkipped,
					"errors":          finalErrors,
					"duration":        elapsed.String(),
					"full_scan":       fullScan,
					"disks":           pathNames,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Printf("\n\nScan complete:\n")
			fmt.Printf("  Files hashed:    %d\n", finalProcessed)
			fmt.Printf("  Files skipped:   %d (unchanged)\n", finalSkipped)
			fmt.Printf("  Total files:     %d\n", finalProcessed+finalSkipped)
			fmt.Printf("  Errors:          %d\n", finalErrors)
			fmt.Printf("  Duration:        %s\n", elapsed.Round(time.Millisecond))
			fmt.Printf("  Database:        %s\n", dbPath)
			if !fullScan {
				fmt.Printf("  Mode:            incremental (use --full to re-hash all)\n")
			} else {
				fmt.Printf("  Mode:            full\n")
			}

			scanErrMu.Lock()
			defer scanErrMu.Unlock()
			if len(scanErrors) > 0 {
				return fmt.Errorf("scan errors: %s", strings.Join(scanErrors, "; "))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&autoDetect, "auto", false, "auto-detect Unraid array disks and cache")
	cmd.Flags().BoolVar(&fullScan, "full", false, "force re-hash all files (skip incremental comparison)")
	return cmd
}

func verifyCmd() *cobra.Command {
	var quick bool
	var disk string
	var workers int

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify file integrity against stored hashes",
		Long:  "Re-hash files and compare against the stored SHA-256 hashes to detect corruption or missing files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			scanID, _ := database.InsertScanHistory("verify", disk)

			v := verifier.New(database, workers, quick)

			corrupted := 0
			missing := 0

			resultCb := func(r verifier.VerifyResult) {
				switch r.Status {
				case "corrupted":
					corrupted++
					if jsonOut {
						return
					}
					fmt.Printf("  CORRUPTED: %s\n", r.Path)
					if r.OldHash != "" && r.NewHash != "" {
						fmt.Printf("    expected: %s\n", r.OldHash)
						fmt.Printf("    got:      %s\n", r.NewHash)
					}
				case "missing":
					missing++
					if !jsonOut {
						fmt.Printf("  MISSING:   %s\n", r.Path)
					}
				}
			}

			var summary *verifier.Summary
			if disk != "" {
				fmt.Printf("Verifying files on disk: %s\n", disk)
				summary, err = v.VerifyDisk(disk, resultCb)
			} else {
				fmt.Printf("Verifying all tracked files...\n")
				summary, err = v.VerifyAll(resultCb)
			}
			if err != nil {
				return fmt.Errorf("verify: %w", err)
			}

			if scanID > 0 {
				if err := database.CompleteScanHistory(scanID, summary.TotalChecked, summary.Errors); err != nil {
					fmt.Fprintf(os.Stderr, "warning: complete scan history: %v\n", err)
				}
			}

			if jsonOut {
				out := map[string]interface{}{
					"total_checked": summary.TotalChecked,
					"ok":            summary.OK,
					"corrupted":     summary.Corrupted,
					"missing":       summary.Missing,
					"skipped":       summary.Skipped,
					"errors":        summary.Errors,
					"duration":      summary.Duration.String(),
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Printf("\nVerification complete:\n")
			fmt.Printf("  Total checked: %d\n", summary.TotalChecked)
			fmt.Printf("  OK:            %d\n", summary.OK)
			fmt.Printf("  Corrupted:     %d\n", summary.Corrupted)
			fmt.Printf("  Missing:       %d\n", summary.Missing)
			if summary.Skipped > 0 {
				fmt.Printf("  Skipped:       %d (unchanged)\n", summary.Skipped)
			}
			fmt.Printf("  Errors:        %d\n", summary.Errors)
			fmt.Printf("  Duration:      %s\n", summary.Duration.Round(time.Millisecond))

			if summary.Corrupted > 0 || summary.Missing > 0 {
				os.Exit(2) // non-zero exit for cron alerting
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&quick, "quick", false, "skip files whose mtime and size haven't changed")
	cmd.Flags().StringVar(&disk, "disk", "", "only verify files on a specific disk")
	cmd.Flags().IntVarP(&workers, "workers", "w", 4, "number of parallel hash workers")
	return cmd
}

func reportCmd() *cobra.Command {
	var disk string
	var status string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show file integrity reports",
		Long:  "Display reports on file inventory, per-disk stats, and corruption status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			// If a specific status is requested, show those files
			if status != "" {
				files, err := database.GetFilesByStatus(status)
				if err != nil {
					return fmt.Errorf("get files: %w", err)
				}
				if jsonOut {
					return json.NewEncoder(os.Stdout).Encode(files)
				}
				fmt.Printf("Files with status '%s': %d\n\n", status, len(files))
				for _, f := range files {
					fmt.Printf("  %s\n", f.Path)
					fmt.Printf("    disk: %s  size: %s  sha256: %s\n",
						f.Disk, format.Size(f.Size), f.SHA256[:16]+"...")
				}
				return nil
			}

			// If a specific disk is requested, show that disk's files
			if disk != "" {
				files, err := database.GetFilesByDisk(disk)
				if err != nil {
					return fmt.Errorf("get files: %w", err)
				}
				if jsonOut {
					return json.NewEncoder(os.Stdout).Encode(files)
				}
				fmt.Printf("Files on disk '%s': %d\n\n", disk, len(files))
				for _, f := range files {
					fmt.Printf("  [%s] %s (%s)\n", f.Status, f.Path, format.Size(f.Size))
				}
				return nil
			}

			// Default: show overview
			stats, err := database.GetStats()
			if err != nil {
				return fmt.Errorf("get stats: %w", err)
			}

			diskStats, err := database.GetDiskStats()
			if err != nil {
				return fmt.Errorf("get disk stats: %w", err)
			}

			if jsonOut {
				out := map[string]interface{}{
					"overview": stats,
					"disks":    diskStats,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Println("=== File Integrity Report ===")
			fmt.Println()
			fmt.Printf("  Total files:     %d\n", stats.TotalFiles)
			fmt.Printf("  Total size:      %s\n", format.Size(stats.TotalSize))
			fmt.Printf("  OK:              %d\n", stats.OKFiles)
			fmt.Printf("  Corrupted:       %d\n", stats.CorruptedFiles)
			fmt.Printf("  Missing:         %d\n", stats.MissingFiles)
			if stats.LastScan != nil {
				fmt.Printf("  Last scan:       %s\n", stats.LastScan.Format(time.RFC3339))
			}
			if stats.LastVerify != nil {
				fmt.Printf("  Last verify:     %s\n", stats.LastVerify.Format(time.RFC3339))
			}

			if len(diskStats) > 0 {
				fmt.Println()
				fmt.Println("  Per-disk breakdown:")
				fmt.Printf("  %-12s %10s %12s %10s %10s\n",
					"DISK", "FILES", "SIZE", "CORRUPT", "MISSING")
				for _, ds := range diskStats {
					fmt.Printf("  %-12s %10d %12s %10d %10d\n",
						ds.Disk, ds.TotalFiles, format.Size(ds.TotalSize),
						ds.CorruptedFiles, ds.MissingFiles)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&disk, "disk", "", "show files on a specific disk")
	cmd.Flags().StringVar(&status, "status", "", "show files with a specific status (ok, corrupted, missing)")
	return cmd
}

func serverCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the web dashboard",
		Long:  "Launch a web server that displays file integrity status, per-disk stats, and corruption reports.",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			addr := fmt.Sprintf(":%d", port)
			fmt.Printf("Starting filehasher dashboard at http://0.0.0.0%s\n", addr)
			return web.Serve(database, addr)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8787, "port to listen on")
	return cmd
}
