package format

import "testing"

func TestSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
		{2199023255552, "2.00 TB"},
		{1572864, "1.50 MB"},          // 1.5 MB
		{10737418240, "10.00 GB"},     // 10 GB
		{5497558138880, "5.00 TB"},    // 5 TB
		{107374182400, "100.00 GB"},   // 100 GB
		{1099511627775, "1024.00 GB"}, // 1 TB - 1 byte (still GB range)
	}

	for _, tt := range tests {
		got := Size(tt.bytes)
		if got != tt.expected {
			t.Errorf("Size(%d) = %q, want %q", tt.bytes, got, tt.expected)
		}
	}
}

func TestSizeNegative(t *testing.T) {
	// Negative values should fall through to the default case
	got := Size(-1)
	if got != "-1 B" {
		t.Errorf("Size(-1) = %q, want %q", got, "-1 B")
	}
}
