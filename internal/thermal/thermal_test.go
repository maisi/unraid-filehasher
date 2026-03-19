package thermal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseDisksINI_Basic(t *testing.T) {
	content := `["disk1"]
device="sda"
temp="36"
rotational="1"
transport="sata"

["disk2"]
device="sdb"
temp="42"
rotational="1"
transport="sata"

["cache"]
device="nvme0n1"
temp="55"
rotational="0"
transport="nvme"
`
	path := writeTempFile(t, content)
	entries := ParseDisksINI(path)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// disk1
	d1 := entries["disk1"]
	if d1.Device != "sda" {
		t.Errorf("disk1.Device = %q, want %q", d1.Device, "sda")
	}
	if d1.Temp != 36 {
		t.Errorf("disk1.Temp = %d, want 36", d1.Temp)
	}
	if !d1.Rotational {
		t.Error("disk1.Rotational should be true")
	}
	if d1.Transport != "sata" {
		t.Errorf("disk1.Transport = %q, want %q", d1.Transport, "sata")
	}

	// cache (NVMe)
	cache := entries["cache"]
	if cache.Device != "nvme0n1" {
		t.Errorf("cache.Device = %q, want %q", cache.Device, "nvme0n1")
	}
	if cache.Temp != 55 {
		t.Errorf("cache.Temp = %d, want 55", cache.Temp)
	}
	if cache.Rotational {
		t.Error("cache.Rotational should be false")
	}
	if cache.Transport != "nvme" {
		t.Errorf("cache.Transport = %q, want %q", cache.Transport, "nvme")
	}
}

func TestParseDisksINI_UnavailableTemp(t *testing.T) {
	content := `["disk1"]
device="sda"
temp="*"
`
	path := writeTempFile(t, content)
	entries := ParseDisksINI(path)

	d1 := entries["disk1"]
	if d1.Temp != -1 {
		t.Errorf("disk1.Temp = %d, want -1 (unavailable)", d1.Temp)
	}
}

func TestParseDisksINI_EmptyTemp(t *testing.T) {
	content := `["disk1"]
device="sda"
temp=""
`
	path := writeTempFile(t, content)
	entries := ParseDisksINI(path)

	d1 := entries["disk1"]
	if d1.Temp != -1 {
		t.Errorf("disk1.Temp = %d, want -1 (empty)", d1.Temp)
	}
}

func TestParseDisksINI_NoTempKey(t *testing.T) {
	content := `["disk1"]
device="sda"
rotational="1"
`
	path := writeTempFile(t, content)
	entries := ParseDisksINI(path)

	d1 := entries["disk1"]
	if d1.Temp != -1 {
		t.Errorf("disk1.Temp = %d, want -1 (no temp key)", d1.Temp)
	}
}

func TestParseDisksINI_SectionWithoutQuotes(t *testing.T) {
	// Some formats might not have quotes around section names
	content := `[disk1]
device="sda"
temp="40"
`
	path := writeTempFile(t, content)
	entries := ParseDisksINI(path)

	if _, ok := entries["disk1"]; !ok {
		t.Error("expected disk1 entry for unquoted section name")
	}
	if entries["disk1"].Temp != 40 {
		t.Errorf("disk1.Temp = %d, want 40", entries["disk1"].Temp)
	}
}

func TestParseDisksINI_EmptyFile(t *testing.T) {
	path := writeTempFile(t, "")
	entries := ParseDisksINI(path)

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty file, got %d", len(entries))
	}
}

func TestParseDisksINI_NonexistentFile(t *testing.T) {
	entries := ParseDisksINI("/nonexistent/path/disks.ini")

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for nonexistent file, got %d", len(entries))
	}
}

func TestParseDisksINI_CommentsAndBlankLines(t *testing.T) {
	content := `# This is a comment

["disk1"]
device="sda"
# Another comment
temp="45"

`
	path := writeTempFile(t, content)
	entries := ParseDisksINI(path)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries["disk1"].Temp != 45 {
		t.Errorf("disk1.Temp = %d, want 45", entries["disk1"].Temp)
	}
}

func TestParseDisksINI_MultipleDisks(t *testing.T) {
	// Realistic Unraid scenario with parity + data + cache
	content := `["parity"]
device="sda"
temp="35"
rotational="1"
transport="sata"

["disk1"]
device="sdb"
temp="37"
rotational="1"
transport="sata"

["disk2"]
device="sdc"
temp="*"
rotational="1"
transport="sata"

["cache"]
device="nvme0n1"
temp="52"
rotational="0"
transport="nvme"

["flash"]
device="sdd"
temp="*"
rotational="0"
transport="usb"
`
	path := writeTempFile(t, content)
	entries := ParseDisksINI(path)

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// parity
	if entries["parity"].Temp != 35 {
		t.Errorf("parity.Temp = %d, want 35", entries["parity"].Temp)
	}

	// disk2 — spun down
	if entries["disk2"].Temp != -1 {
		t.Errorf("disk2.Temp = %d, want -1 (spun down)", entries["disk2"].Temp)
	}

	// cache
	if entries["cache"].Transport != "nvme" {
		t.Errorf("cache.Transport = %q, want %q", entries["cache"].Transport, "nvme")
	}

	// flash
	if entries["flash"].Transport != "usb" {
		t.Errorf("flash.Transport = %q, want %q", entries["flash"].Transport, "usb")
	}
}

// --- smartctlOutput JSON parsing tests ---

func TestSmartctlOutput_SATA194(t *testing.T) {
	// Test parsing the smartctlOutput struct directly with SATA attr 194
	jsonData := `{
		"smartctl": {"exit_status": 0},
		"ata_smart_attributes": {
			"table": [
				{"id": 1, "name": "Raw_Read_Error_Rate", "raw": {"value": 0}},
				{"id": 194, "name": "Temperature_Celsius", "raw": {"value": 36}},
				{"id": 9, "name": "Power_On_Hours", "raw": {"value": 12345}}
			]
		}
	}`

	var result smartctlOutput
	if err := parseJSON([]byte(jsonData), &result); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if result.ATASmartAttributes == nil {
		t.Fatal("ATASmartAttributes is nil")
	}

	// Find attr 194
	found := false
	for _, attr := range result.ATASmartAttributes.Table {
		if attr.ID == 194 {
			if attr.Raw.Value != 36 {
				t.Errorf("attr 194 raw value = %d, want 36", attr.Raw.Value)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("attribute 194 not found")
	}
}

func TestSmartctlOutput_NVMe(t *testing.T) {
	jsonData := `{
		"smartctl": {"exit_status": 0},
		"nvme_smart_health_information_log": {
			"temperature": 42,
			"temperature_sensors": [42, 45]
		}
	}`

	var result smartctlOutput
	if err := parseJSON([]byte(jsonData), &result); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if result.NVMEHealthLog == nil {
		t.Fatal("NVMEHealthLog is nil")
	}
	if result.NVMEHealthLog.Temperature != 42 {
		t.Errorf("NVMe temp = %d, want 42", result.NVMEHealthLog.Temperature)
	}
}

func TestSmartctlOutput_Standby(t *testing.T) {
	jsonData := `{
		"smartctl": {"exit_status": 2},
		"power_mode": {"is_standby": true}
	}`

	var result smartctlOutput
	if err := parseJSON([]byte(jsonData), &result); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if result.PowerMode == nil {
		t.Fatal("PowerMode is nil")
	}
	if !result.PowerMode.IsStandby {
		t.Error("expected IsStandby = true")
	}
}

func TestSmartctlOutput_SATA190Fallback(t *testing.T) {
	// Some drives only have attr 190 (Airflow_Temperature_Cel), not 194
	jsonData := `{
		"smartctl": {"exit_status": 0},
		"ata_smart_attributes": {
			"table": [
				{"id": 190, "name": "Airflow_Temperature_Cel", "raw": {"value": 33}},
				{"id": 9, "name": "Power_On_Hours", "raw": {"value": 5000}}
			]
		}
	}`

	var result smartctlOutput
	if err := parseJSON([]byte(jsonData), &result); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Should find attr 190 as fallback
	found := false
	for _, attr := range result.ATASmartAttributes.Table {
		if attr.ID == 190 {
			if attr.Raw.Value != 33 {
				t.Errorf("attr 190 raw value = %d, want 33", attr.Raw.Value)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("attribute 190 not found")
	}
}

// --- helpers ---

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "disks.ini")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// parseJSON is a thin wrapper to keep tests from importing encoding/json directly.
func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
