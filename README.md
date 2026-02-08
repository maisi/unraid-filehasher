# filehasher

File integrity checker for Unraid servers. Catalogs every file across your array disks and cache pools, stores SHA-256 hashes in a local SQLite database, and detects corruption or missing files on subsequent verification runs.

## Why

Unraid's parity protects against disk failure, but not against filesystem corruption, bitrot, or silent data loss. If a file gets corrupted at the filesystem level, parity dutifully mirrors the damage. `filehasher` gives you:

- **A catalog** of every file, which disk it lives on, and its SHA-256 hash
- **Corruption detection** by re-hashing files and comparing against stored values
- **Missing file detection** when files disappear unexpectedly
- **Per-disk visibility** so you know exactly what's on each drive
- **A web dashboard** for browsing status at a glance
- **Cron-friendly** exit codes and JSON output for automation

## Quick Start

### Build

```bash
# Requires Go 1.24+
go build -o filehasher ./cmd/

# Production build (smaller, static binary)
CGO_ENABLED=0 go build -ldflags="-s -w" -o filehasher ./cmd/
```

The result is a single ~13MB static binary with zero runtime dependencies.

### Install on Unraid

**Option A: Plugin (recommended)**

Install `filehasher.plg` through Unraid's Community Applications or manually:

```bash
plugin install https://github.com/OWNER/filehasher/releases/latest/download/filehasher.plg
```

This installs the binary, creates the config directory, and optionally sets up a weekly cron job.

**Option B: Manual install**

Copy the binary to the USB boot drive so it persists across reboots:

```bash
mkdir -p /boot/config/filehasher
cp filehasher /boot/config/filehasher/
```

The database will be stored alongside it at `/boot/config/filehasher/filehasher.db` by default (auto-detected when `/boot/config` exists).

### Initial Scan

```bash
# Auto-detect all array disks and cache pools
filehasher scan --auto

# Or specify paths manually
filehasher scan /mnt/disk1 /mnt/disk2 /mnt/cache
```

This walks every directory, hashes every file (SHA-256), and stores the results. The initial scan of a multi-TB array will take hours -- this is unavoidable as it's disk I/O bound.

Subsequent scans are **incremental by default**: files whose size and mtime haven't changed since the last scan are skipped. Use `--full` to force re-hashing all files.

### Verify Integrity

```bash
# Verify all tracked files
filehasher verify

# Verify a specific disk
filehasher verify --disk disk3

# Quick verify -- only re-hash files whose mtime or size changed
filehasher verify --quick
```

Exit codes:
- `0` -- All files OK
- `2` -- Corruption or missing files detected

### View Reports

```bash
# Overview with per-disk breakdown
filehasher report

# Show only corrupted files
filehasher report --status corrupted

# Show only missing files
filehasher report --status missing

# Show all files on a specific disk
filehasher report --disk disk3

# JSON output (for scripting)
filehasher report --json
```

### Web Dashboard

```bash
filehasher server --port 8787
```

Open `http://<server-ip>:8787` in your browser. The dashboard provides:

- **Overview** -- Total files, total size, health status, last scan/verify times
- **Disk breakdown** -- Per-disk file count, size, corruption count
- **Corrupted files** -- List of files with hash mismatches
- **Missing files** -- Files that were cataloged but no longer exist
- **Search** -- Find files by path
- **History** -- Timeline of all scan and verify operations

## Commands

### `filehasher scan [paths...]`

Walk directories, hash all files, and store results in the database.

By default, scans are **incremental**: only files that are new or have changed (different size or mtime) are hashed. Previously scanned files that haven't changed are skipped.

When using `--auto`, each disk gets its own hashing pipeline with worker counts tuned to the storage type:
- **HDD**: 1 worker (spinning disks get slower with parallel reads)
- **SSD**: 4 workers (solid state benefits from parallelism)
- **Unknown**: 2 workers (safe default)

Disk type is auto-detected via `/sys/block/<dev>/queue/rotational`.

| Flag | Description |
|------|-------------|
| `--auto` | Auto-detect Unraid disks (`/mnt/disk*`, `/mnt/cache*`) |
| `--full` | Force re-hash all files (disable incremental mode) |
| `-e, --exclude PATTERN` | Regex patterns to exclude (repeatable) |
| `--exclude-simple TEXT` | Simple exclude (substring match on full path; repeatable) |
| `--exclude-appdata` | Exclude Unraid `appdata` folders (useful to skip noisy docker data) |
| `--disk-type auto|hdd|ssd` | Force disk type (overrides /sys rotational detection) |
| `--db PATH` | Database path (default: auto-detected) |
| `--json` | JSON output |

### `filehasher verify`

Re-hash tracked files and compare against stored hashes.

| Flag | Description |
|------|-------------|
| `--quick` | Only check files whose mtime or size changed |
| `--disk NAME` | Only verify files on a specific disk |
| `-w, --workers N` | Parallel hash workers (default: 4) |
| `--json` | JSON output |

### `filehasher report`

Display integrity reports.

| Flag | Description |
|------|-------------|
| `--status STATUS` | Filter by status: `ok`, `corrupted`, `missing` |
| `--disk NAME` | Show files on a specific disk |
| `--json` | JSON output |

### `filehasher server`

Launch the web dashboard.

| Flag | Description |
|------|-------------|
| `-p, --port PORT` | Listen port (default: 8787) |

## Global Flags

| Flag | Description |
|------|-------------|
| `--db PATH` | SQLite database path |
| `-e, --exclude PATTERN` | Regex exclude patterns (repeatable) |
| `--exclude-simple TEXT` | Simple exclude (substring match on full path; repeatable) |
| `--exclude-appdata` | Exclude Unraid `appdata` folders |
| `--json` | JSON output for all commands |
| `-v, --version` | Print version |

## Automation

### Cron (Recommended)

Add to `/boot/config/go` on Unraid (this script runs at array start):

```bash
#!/bin/bash

# Weekly full verification at 3 AM Sunday
echo "0 3 * * 0 /boot/config/filehasher/filehasher verify >> /boot/config/filehasher/verify.log 2>&1" >> /etc/crontab

# Monthly re-scan to pick up new files (1 AM, 1st of month)
echo "0 1 1 * * /boot/config/filehasher/filehasher scan --auto >> /boot/config/filehasher/scan.log 2>&1" >> /etc/crontab

# Start the web dashboard
nohup /boot/config/filehasher/filehasher server --port 8787 > /dev/null 2>&1 &
```

If you installed via the plugin, you can enable the built-in cron job by editing `/boot/config/filehasher/cron.cfg` and setting `ENABLED=yes`.

### Exclude Patterns

Skip files you don't care about:

```bash
# Skip Plex transcoding cache and thumbnail files
filehasher scan --auto \
  -e "\.plex/.*Transcode" \
  -e "\.plex/.*Cache" \
  -e "\.Trash-" \
  -e "\.DS_Store" \
  -e "Thumbs\.db"
```

### JSON Integration

Use JSON output to integrate with monitoring or notification systems:

```bash
# Check for corruption and send alert
result=$(filehasher verify --json)
corrupted=$(echo "$result" | jq '.corrupted')
if [ "$corrupted" -gt 0 ]; then
  # Send Discord/Telegram/email notification
  echo "ALERT: $corrupted corrupted files detected!" | mail -s "filehasher alert" you@example.com
fi
```

## How It Works

### Scanning

1. Walks the specified directory trees, skipping symlinks, devices, and zero-byte files
2. Groups files by disk with a separate hashing pipeline per disk
3. For each disk, auto-detects HDD vs SSD and sets worker count accordingly
4. In incremental mode (default), compares each file's size and mtime against the database and skips unchanged files
5. Hashes changed/new files with SHA-256 using 1MB read buffers
6. Stores path, disk name, size, mtime, and hash in SQLite
7. Batches writes in transactions of 1000 for performance

### Disk Detection

On Unraid, disks are mounted at `/mnt/disk1`, `/mnt/disk2`, etc. and cache pools at `/mnt/cache`, `/mnt/cache2`, etc. The `--auto` flag detects these automatically. For `/mnt/user/` paths (the fuse mount), filehasher resolves symlinks back to the physical disk.

HDD vs SSD detection reads `/sys/block/<dev>/queue/rotational` after resolving the mount point's block device from `/proc/mounts`.

### Verification

1. Loads all tracked file records from the database
2. Checks if each file still exists (marks missing if not)
3. Re-hashes existing files and compares against stored SHA-256
4. Updates status: `ok`, `corrupted`, or `missing`
5. In `--quick` mode, skips files whose mtime and size match the stored values

### Database

Single SQLite file with WAL mode enabled for performance. Schema:

```
files:         path, disk, size, mtime, sha256, first_seen, last_verified, status
scan_history:  scan_type, started_at, ended_at, disks, files_processed, errors, status
```

The database is fully self-contained -- you can copy it off the server for backup or analysis.

## Performance

- **Hashing speed**: Bound by disk I/O, not CPU. SHA-256 is hardware-accelerated on modern CPUs.
- **Per-disk parallelism**: Each disk gets its own pipeline. HDDs get 1 worker (sequential reads are fastest). SSDs get 4 workers. This is auto-detected; no configuration needed with `--auto`.
- **Incremental scans**: By default, only new or changed files are hashed. After the initial full scan, subsequent scans complete in seconds if nothing changed. Use `--full` to force re-hashing everything.
- **Memory**: Minimal. Files are streamed through the hasher in 1MB chunks. The existing file index is loaded into a map for O(1) lookups during incremental comparison, which uses ~100 bytes per tracked file.

## Project Structure

```
filehasher/
├── cmd/main.go                  # CLI entry point (scan, verify, report, server)
├── internal/
│   ├── db/db.go                 # SQLite database layer
│   ├── format/format.go         # Shared size formatting
│   ├── hasher/hasher.go         # Parallel SHA-256 hashing engine
│   ├── scanner/scanner.go       # Filesystem walker + Unraid disk detection
│   ├── verifier/verifier.go     # Hash comparison logic
│   └── web/
│       ├── server.go            # HTTP handlers + JSON API
│       └── templates.go         # Embedded HTML templates
├── filehasher.plg               # Unraid plugin package
├── go.mod
├── go.sum
└── Makefile
```

## Building for Unraid

Unraid runs on x86_64 Linux. Cross-compile from any platform:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o filehasher ./cmd/
```

## Future Plans

- **Notifications** (Discord, Telegram, email) on corruption detection
- **Par2 recovery** -- Generate repair data to fix partially corrupted files
- **Scheduled scans** built into the web UI
- **File change history** -- Track when files were modified over time

## License

MIT
