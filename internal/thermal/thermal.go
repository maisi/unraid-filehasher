package thermal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const disksINIPath = "/var/local/emhttp/disks.ini"

// TempReading holds a temperature reading for a single disk.
type TempReading struct {
	Temp      int  // Celsius
	Available bool // false if temp couldn't be read (spun down, missing, etc.)
}

// DiskINIEntry holds parsed fields from a disks.ini section.
type DiskINIEntry struct {
	Name       string // logical name: disk1, cache, parity, etc.
	Device     string // block device: sda, nvme0n1, etc.
	Temp       int    // Celsius; -1 if unavailable
	Rotational bool   // true = HDD
	Transport  string // sata, nvme, usb, etc.
}

// ReadTemperatures reads current disk temperatures.
// It tries /var/local/emhttp/disks.ini first (Unraid), then falls back to
// smartctl for any disks not found or with unavailable temperatures.
//
// diskNames is a list of logical disk names (e.g., "disk1", "cache").
// Returns a map from disk name to temperature reading.
func ReadTemperatures(diskNames []string) map[string]TempReading {
	result := make(map[string]TempReading, len(diskNames))

	// Try disks.ini first
	iniEntries := ParseDisksINI(disksINIPath)

	for _, name := range diskNames {
		if entry, ok := iniEntries[name]; ok && entry.Temp >= 0 {
			result[name] = TempReading{Temp: entry.Temp, Available: true}
			continue
		}

		// Fallback: try smartctl if we know the device
		if entry, ok := iniEntries[name]; ok && entry.Device != "" {
			temp, err := ReadSmartTemp("/dev/" + entry.Device)
			if err == nil {
				result[name] = TempReading{Temp: temp, Available: true}
				continue
			}
		}

		// Try to resolve device from /proc/mounts
		device := resolveDevice(name)
		if device != "" {
			temp, err := ReadSmartTemp(device)
			if err == nil {
				result[name] = TempReading{Temp: temp, Available: true}
				continue
			}
		}

		result[name] = TempReading{Available: false}
	}

	return result
}

// ParseDisksINI parses Unraid's /var/local/emhttp/disks.ini.
// Section names look like ["disk1"], values are key="value" pairs.
// Returns a map from logical disk name to parsed entry.
func ParseDisksINI(path string) map[string]DiskINIEntry {
	entries := make(map[string]DiskINIEntry)

	f, err := os.Open(path)
	if err != nil {
		return entries
	}
	defer f.Close()

	var current *DiskINIEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header: ["disk1"] or [disk1]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := line[1 : len(line)-1]
			// Remove quotes if present: ["disk1"] -> "disk1" -> disk1
			name = strings.Trim(name, "\"")
			entry := DiskINIEntry{Name: name, Temp: -1}
			entries[name] = entry
			entryCopy := entries[name]
			current = &entryCopy
			continue
		}

		// Key=value pair
		if current == nil {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		switch key {
		case "device":
			current.Device = val
		case "temp":
			if val != "*" && val != "" {
				if t, err := strconv.Atoi(val); err == nil {
					current.Temp = t
				}
			}
		case "rotational":
			current.Rotational = val == "1"
		case "transport":
			current.Transport = val
		}

		// Write back to map
		entries[current.Name] = *current
	}

	return entries
}

// smartctlOutput is the JSON structure returned by smartctl -A -j.
type smartctlOutput struct {
	Smartctl *struct {
		ExitStatus int `json:"exit_status"`
	} `json:"smartctl"`
	PowerMode *struct {
		IsStandby bool `json:"is_standby"`
	} `json:"power_mode"`
	ATASmartAttributes *struct {
		Table []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Raw  struct {
				Value int `json:"value"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	NVMEHealthLog *struct {
		Temperature int   `json:"temperature"`
		TempSensors []int `json:"temperature_sensors"`
	} `json:"nvme_smart_health_information_log"`
}

// ReadSmartTemp reads the current temperature from a block device using smartctl.
// Uses -n standby to avoid spinning up sleeping HDDs.
// Handles both SATA (attribute 194/190) and NVMe temperature reporting.
func ReadSmartTemp(device string) (int, error) {
	out, err := exec.Command("smartctl", "-n", "standby", "-A", "-j", device).Output()
	if err != nil {
		// smartctl returns non-zero for non-fatal conditions (bitmask exit status).
		// Only fail if we got no output at all.
		if len(out) == 0 {
			return 0, fmt.Errorf("smartctl failed for %s: %w", device, err)
		}
	}

	var result smartctlOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, fmt.Errorf("parse smartctl JSON for %s: %w", device, err)
	}

	// Check if disk is in standby — don't report temp (would require spin-up)
	if result.PowerMode != nil && result.PowerMode.IsStandby {
		return 0, fmt.Errorf("disk %s is in standby", device)
	}

	// NVMe?
	if result.NVMEHealthLog != nil {
		return result.NVMEHealthLog.Temperature, nil
	}

	// SATA: look for attribute 194 first, then 190
	if result.ATASmartAttributes != nil {
		for _, attr := range result.ATASmartAttributes.Table {
			if attr.ID == 194 {
				return attr.Raw.Value, nil
			}
		}
		for _, attr := range result.ATASmartAttributes.Table {
			if attr.ID == 190 {
				return attr.Raw.Value, nil
			}
		}
	}

	return 0, fmt.Errorf("no temperature attribute found for %s", device)
}

// resolveDevice maps a logical Unraid disk name to its block device path
// by scanning /proc/mounts for /mnt/<name>.
func resolveDevice(diskName string) string {
	mountPoint := "/mnt/" + diskName

	f, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		if fields[1] == mountPoint && strings.HasPrefix(fields[0], "/dev/") {
			return fields[0]
		}
	}
	return ""
}
