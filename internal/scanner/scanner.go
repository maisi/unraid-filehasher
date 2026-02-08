package scanner

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/maisi/unraid-filehasher/internal/hasher"
)

// DiskType represents the storage type of a disk.
type DiskType int

const (
	DiskTypeUnknown DiskType = iota
	DiskTypeHDD
	DiskTypeSSD
)

func (dt DiskType) String() string {
	switch dt {
	case DiskTypeHDD:
		return "HDD"
	case DiskTypeSSD:
		return "SSD"
	default:
		return "unknown"
	}
}

// DefaultWorkers returns the recommended worker count for this disk type.
func (dt DiskType) DefaultWorkers() int {
	switch dt {
	case DiskTypeHDD:
		return 1
	case DiskTypeSSD:
		return 4
	default:
		return 2
	}
}

// Package-level compiled regex patterns for Unraid disk names.
var (
	diskPattern  = regexp.MustCompile(`^disk\d+$`)
	cachePattern = regexp.MustCompile(`^cache\d*$`)
)

// DiskInfo represents a detected Unraid disk.
type DiskInfo struct {
	Name string   // e.g., "disk1", "disk2", "cache"
	Path string   // e.g., "/mnt/disk1"
	Type DiskType // HDD, SSD, or unknown
}

// Scanner walks filesystem paths and feeds files to the hasher.
type Scanner struct {
	excludePatterns []*regexp.Regexp
}

// New creates a new Scanner with optional exclude patterns.
func New(excludePatterns []string) (*Scanner, error) {
	var compiled []*regexp.Regexp
	for _, p := range excludePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	return &Scanner{excludePatterns: compiled}, nil
}

// DetectUnraidDisks auto-detects mounted Unraid array disks and cache pools.
func DetectUnraidDisks() ([]DiskInfo, error) {
	var disks []DiskInfo

	entries, err := os.ReadDir("/mnt")
	if err != nil {
		return nil, fmt.Errorf("read /mnt: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join("/mnt", name)

		if diskPattern.MatchString(name) || cachePattern.MatchString(name) {
			// Verify it's actually mounted (has files)
			subEntries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			if len(subEntries) == 0 {
				continue
			}
			diskType := detectDiskType(path)
			disks = append(disks, DiskInfo{Name: name, Path: path, Type: diskType})
		}
	}

	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Name < disks[j].Name
	})

	return disks, nil
}

// detectDiskType checks /sys/block/<dev>/queue/rotational to determine HDD vs SSD.
// Returns DiskTypeHDD (rotational=1), DiskTypeSSD (rotational=0), or DiskTypeUnknown.
func detectDiskType(mountPath string) DiskType {
	// Find the mount source device for this mount point by reading /proc/mounts
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return DiskTypeUnknown
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || fields[1] != mountPath {
			continue
		}
		devBase := filepath.Base(fields[0]) // e.g. "sda1", "md1", "dm-0"
		return diskTypeFromBlockDevice(devBase)
	}

	return DiskTypeUnknown
}

// diskTypeFromBlockDevice attempts to determine HDD/SSD for a given block device name.
// It is more robust than directly reading /sys/block/<dev>/queue/rotational because Unraid
// often mounts array disks via /dev/mdX and pools via dm-crypt/LVM/dm devices.
func diskTypeFromBlockDevice(dev string) DiskType {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return DiskTypeUnknown
	}

	// If this is a partition (sda1, nvme0n1p1, etc.), try to map to the parent disk.
	parent := parentBlockDevice(dev)
	if parent == "" {
		parent = dev
	}

	// If this is a stacked device (md/dm), inspect its slaves.
	if t := diskTypeFromSlaves(parent); t != DiskTypeUnknown {
		return t
	}

	// Fall back to rotational flag.
	return diskTypeFromRotational(parent)
}

func parentBlockDevice(dev string) string {
	// nvme0n1p1 -> nvme0n1
	if strings.HasPrefix(dev, "nvme") {
		if idx := strings.LastIndex(dev, "p"); idx > 0 {
			return dev[:idx]
		}
		return dev
	}
	// mmcblk0p1 -> mmcblk0
	if strings.HasPrefix(dev, "mmcblk") {
		if idx := strings.LastIndex(dev, "p"); idx > 0 {
			return dev[:idx]
		}
		return dev
	}
	// sda1 -> sda, vda2 -> vda
	trimmed := strings.TrimRightFunc(dev, func(r rune) bool { return r >= '0' && r <= '9' })
	if trimmed == "" {
		return dev
	}
	return trimmed
}

func diskTypeFromSlaves(dev string) DiskType {
	slavesDir := filepath.Join("/sys/class/block", dev, "slaves")
	entries, err := os.ReadDir(slavesDir)
	if err != nil || len(entries) == 0 {
		return DiskTypeUnknown
	}

	seen := 0
	anyHDD := false
	allSSD := true

	for _, e := range entries {
		// Entries in /sys/class/block/<dev>/slaves are typically symlinks, but on sysfs
		// DirEntry.Type() may be TypeUnknown (0). Don't try to filter based on Type();
		// just use the name and attempt rotational detection.
		seen++
		slave := e.Name()
		// slave might itself be a partition; normalize
		slaveParent := parentBlockDevice(slave)
		if slaveParent == "" {
			slaveParent = slave
		}
		t := diskTypeFromRotational(slaveParent)
		switch t {
		case DiskTypeHDD:
			anyHDD = true
			allSSD = false
		case DiskTypeSSD:
			// keep allSSD true
		default:
			allSSD = false
		}
	}

	if seen == 0 {
		return DiskTypeUnknown
	}
	if anyHDD {
		return DiskTypeHDD
	}
	if allSSD {
		return DiskTypeSSD
	}
	return DiskTypeUnknown
}

func diskTypeFromRotational(dev string) DiskType {
	rot, ok := readRotationalSysfs(dev)
	if !ok {
		return DiskTypeUnknown
	}
	if rot == "1" {
		return DiskTypeHDD
	}
	if rot == "0" {
		return DiskTypeSSD
	}
	return DiskTypeUnknown
}

func readRotationalSysfs(dev string) (string, bool) {
	start := filepath.Join("/sys/class/block", dev)
	resolved, err := filepath.EvalSymlinks(start)
	if err != nil {
		return "", false
	}
	// Walk up until we find a queue/rotational file.
	p := resolved
	for i := 0; i < 6; i++ {
		rotPath := filepath.Join(p, "queue", "rotational")
		data, err := os.ReadFile(rotPath)
		if err == nil {
			return strings.TrimSpace(string(data)), true
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return "", false
}

// ResolveDisk determines which Unraid disk a path belongs to.
// For paths like /mnt/disk1/..., it returns "disk1".
// For paths like /mnt/cache/..., it returns "cache".
// For paths like /mnt/user/..., it resolves the symlink to find the actual disk.
// For other paths, it returns the base of the given root.
func ResolveDisk(filePath, scanRoot string) string {
	if strings.HasPrefix(filePath, "/mnt/") {
		parts := strings.SplitN(strings.TrimPrefix(filePath, "/mnt/"), "/", 2)
		if len(parts) >= 1 {
			name := parts[0]
			if diskPattern.MatchString(name) || cachePattern.MatchString(name) {
				return name
			}
		}

		// For /mnt/user/ paths, try to resolve the symlink
		if strings.HasPrefix(filePath, "/mnt/user/") || strings.HasPrefix(filePath, "/mnt/user0/") {
			resolved, err := filepath.EvalSymlinks(filePath)
			if err == nil && resolved != filePath {
				return ResolveDisk(resolved, scanRoot)
			}
		}
	}

	// Fallback: use the scan root's base name
	return filepath.Base(scanRoot)
}

// Walk walks a directory tree and sends discovered files to the channel.
// It skips files matching the exclude patterns.
// Each file includes its stat info (size, mtime) so callers don't need to re-stat.
func (s *Scanner) Walk(root string, disk string, files chan<- hasher.FileInfo) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log but continue on permission errors, etc.
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
			return nil
		}

		// Skip directories (we only hash files)
		if d.IsDir() {
			for _, re := range s.excludePatterns {
				if re.MatchString(path) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip non-regular files (symlinks, devices, sockets, etc.)
		if !d.Type().IsRegular() {
			return nil
		}

		// Check exclude patterns
		for _, re := range s.excludePatterns {
			if re.MatchString(path) {
				return nil
			}
		}

		// Get file info for size and mtime
		info, err := d.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: stat %s: %v\n", path, err)
			return nil
		}

		// Skip zero-byte files — there's nothing to hash.
		// NOTE: If corruption truncates a file to 0 bytes, it will not appear
		// in the database and won't be flagged as corrupted. This is an
		// intentional trade-off: tracking millions of legitimately empty files
		// (lock files, markers, etc.) would add noise for little benefit.
		if info.Size() == 0 {
			return nil
		}

		files <- hasher.FileInfo{
			Path:  path,
			Disk:  disk,
			Size:  info.Size(),
			Mtime: info.ModTime().Unix(),
		}
		return nil
	})

	return err
}
