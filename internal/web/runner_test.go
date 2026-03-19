package web

import (
	"context"
	"sync"
	"testing"
	"time"
)

// --- parseHHMM tests ---

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input string
		wantH int
		wantM int
	}{
		{"00:00", 0, 0},
		{"09:30", 9, 30},
		{"23:59", 23, 59},
		{"12:00", 12, 0},
		{"06:05", 6, 5},
		// Invalid
		{"", -1, -1},
		{"12", -1, -1},
		{"12:60", -1, -1},    // minute out of range
		{"24:00", -1, -1},    // hour out of range
		{"-1:00", -1, -1},    // negative hour
		{"12:-1", -1, -1},    // negative minute
		{"ab:cd", -1, -1},    // non-numeric
		{"12:00:00", -1, -1}, // extra colon — SplitN(2) gives "12" and "00:00", Atoi("00:00") fails
	}

	for _, tt := range tests {
		h, m := parseHHMM(tt.input)
		if h != tt.wantH || m != tt.wantM {
			t.Errorf("parseHHMM(%q) = (%d, %d), want (%d, %d)", tt.input, h, m, tt.wantH, tt.wantM)
		}
	}
}

// --- isInDndWindow tests ---

func makeTime(hour, minute int) time.Time {
	return time.Date(2026, 3, 19, hour, minute, 0, 0, time.Local)
}

func TestIsInDndWindow_SameDay(t *testing.T) {
	// Window: 09:00 - 17:00
	tests := []struct {
		now  time.Time
		want bool
	}{
		{makeTime(8, 59), false},
		{makeTime(9, 0), true},   // start boundary: inclusive
		{makeTime(12, 0), true},  // middle
		{makeTime(16, 59), true}, // just before end
		{makeTime(17, 0), false}, // end boundary: exclusive
		{makeTime(23, 0), false},
		{makeTime(0, 0), false},
	}

	for _, tt := range tests {
		got := isInDndWindow(tt.now, "09:00", "17:00")
		if got != tt.want {
			t.Errorf("isInDndWindow(%s, 09:00, 17:00) = %v, want %v",
				tt.now.Format("15:04"), got, tt.want)
		}
	}
}

func TestIsInDndWindow_CrossesMidnight(t *testing.T) {
	// Window: 23:00 - 06:00 (overnight)
	tests := []struct {
		now  time.Time
		want bool
	}{
		{makeTime(22, 59), false},
		{makeTime(23, 0), true},  // start boundary: inclusive
		{makeTime(23, 30), true}, // late evening
		{makeTime(0, 0), true},   // midnight
		{makeTime(3, 0), true},   // middle of night
		{makeTime(5, 59), true},  // just before end
		{makeTime(6, 0), false},  // end boundary: exclusive
		{makeTime(12, 0), false}, // midday
	}

	for _, tt := range tests {
		got := isInDndWindow(tt.now, "23:00", "06:00")
		if got != tt.want {
			t.Errorf("isInDndWindow(%s, 23:00, 06:00) = %v, want %v",
				tt.now.Format("15:04"), got, tt.want)
		}
	}
}

func TestIsInDndWindow_EqualStartEnd(t *testing.T) {
	// Window: 00:00 - 00:00 — start == end means startMin == endMin.
	// The "same-day" branch requires startMin < endMin, which is false here.
	// So it falls into the midnight-crossing branch: nowMin >= 0 || nowMin < 0,
	// which is always true. Equal start/end effectively means "always active".
	// This is an acceptable edge case — the UI should prevent equal start/end.
	got := isInDndWindow(makeTime(12, 0), "00:00", "00:00")
	if !got {
		t.Error("isInDndWindow with equal start/end (00:00-00:00) should be true (always active)")
	}
}

func TestIsInDndWindow_InvalidTimes(t *testing.T) {
	// Invalid start or end should never activate
	tests := []struct {
		start string
		end   string
	}{
		{"", "17:00"},
		{"09:00", ""},
		{"invalid", "17:00"},
		{"09:00", "invalid"},
		{"", ""},
	}

	for _, tt := range tests {
		got := isInDndWindow(makeTime(12, 0), tt.start, tt.end)
		if got {
			t.Errorf("isInDndWindow(12:00, %q, %q) = true, want false (invalid times)",
				tt.start, tt.end)
		}
	}
}

// --- dndPauseState tests ---

func TestDndPauseState_StartsUnpaused(t *testing.T) {
	s := newDndPauseState()
	if s.isPaused() {
		t.Error("new dndPauseState should start unpaused")
	}

	// waitIfPaused should return immediately
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.waitIfPaused(ctx); err != nil {
		t.Errorf("waitIfPaused on unpaused state: %v", err)
	}
}

func TestDndPauseState_PauseAndResume(t *testing.T) {
	s := newDndPauseState()

	s.pause()
	if !s.isPaused() {
		t.Error("expected paused after pause()")
	}

	// waitIfPaused should block — verify it times out
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := s.waitIfPaused(ctx)
	if err == nil {
		t.Error("waitIfPaused should have timed out while paused")
	}

	// Resume and verify it unblocks
	s.resume()
	if s.isPaused() {
		t.Error("expected unpaused after resume()")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := s.waitIfPaused(ctx2); err != nil {
		t.Errorf("waitIfPaused after resume: %v", err)
	}
}

func TestDndPauseState_DoublePauseResume(t *testing.T) {
	s := newDndPauseState()

	// Double pause shouldn't break anything
	s.pause()
	s.pause()
	if !s.isPaused() {
		t.Error("expected paused after double pause()")
	}

	// Double resume shouldn't panic
	s.resume()
	s.resume()
	if s.isPaused() {
		t.Error("expected unpaused after double resume()")
	}
}

func TestDndPauseState_ConcurrentWaiters(t *testing.T) {
	s := newDndPauseState()
	s.pause()

	const numWaiters = 10
	var wg sync.WaitGroup
	wg.Add(numWaiters)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < numWaiters; i++ {
		go func() {
			defer wg.Done()
			s.waitIfPaused(ctx)
		}()
	}

	// Let waiters block briefly, then resume
	time.Sleep(50 * time.Millisecond)
	s.resume()

	// All waiters should unblock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Error("not all waiters unblocked after resume")
	}
}

// --- diskThermalState tests ---

func TestDiskThermalState_StartsUnpaused(t *testing.T) {
	s := newDiskThermalState()
	if s.paused != 0 {
		t.Error("new diskThermalState should start unpaused")
	}
	if s.temp != -1 {
		t.Errorf("new diskThermalState temp = %d, want -1", s.temp)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.waitIfPaused(ctx); err != nil {
		t.Errorf("waitIfPaused on unpaused state: %v", err)
	}
}

func TestDiskThermalState_PauseAndResume(t *testing.T) {
	s := newDiskThermalState()

	s.pause()

	// Should block
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := s.waitIfPaused(ctx)
	if err == nil {
		t.Error("waitIfPaused should have timed out while paused")
	}

	s.resume()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := s.waitIfPaused(ctx2); err != nil {
		t.Errorf("waitIfPaused after resume: %v", err)
	}
}

func TestDiskThermalState_ContextCancellation(t *testing.T) {
	s := newDiskThermalState()
	s.pause()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	err := s.waitIfPaused(ctx)
	if err != context.Canceled {
		t.Errorf("waitIfPaused with cancelled context: got %v, want context.Canceled", err)
	}
}
