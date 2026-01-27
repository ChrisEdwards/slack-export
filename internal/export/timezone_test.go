package export

import (
	"testing"
	"time"
)

func TestGetDateBounds_EST(t *testing.T) {
	// Standard time: America/New_York is UTC-5 in winter
	start, end, err := GetDateBounds("2026-01-22", "America/New_York")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-01-22 03:00:00 EST = 2026-01-22 08:00:00 UTC
	wantStart := time.Date(2026, 1, 22, 8, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-01-23 02:59:59 EST = 2026-01-23 07:59:59 UTC
	wantEnd := time.Date(2026, 1, 23, 7, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_EDT(t *testing.T) {
	// Daylight time: America/New_York is UTC-4 in summer
	start, end, err := GetDateBounds("2026-07-22", "America/New_York")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-07-22 03:00:00 EDT = 2026-07-22 07:00:00 UTC
	wantStart := time.Date(2026, 7, 22, 7, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-07-23 02:59:59 EDT = 2026-07-23 06:59:59 UTC
	wantEnd := time.Date(2026, 7, 23, 6, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_DSTTransitionSpring(t *testing.T) {
	// US DST spring transition 2026: March 8 at 2:00 AM local time
	// Clocks spring forward from 2:00 AM to 3:00 AM
	start, end, err := GetDateBounds("2026-03-08", "America/New_York")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-03-08 03:00:00 EDT (after spring forward) = 2026-03-08 07:00:00 UTC
	wantStart := time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-03-09 02:59:59 EDT = 2026-03-09 06:59:59 UTC
	wantEnd := time.Date(2026, 3, 9, 6, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_DSTTransitionFall(t *testing.T) {
	// US DST fall transition 2026: November 1 at 2:00 AM local time
	// Clocks fall back from 2:00 AM to 1:00 AM
	start, end, err := GetDateBounds("2026-11-01", "America/New_York")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-11-01 03:00:00 EST (after fall back) = 2026-11-01 08:00:00 UTC
	wantStart := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-11-02 02:59:59 EST = 2026-11-02 07:59:59 UTC
	wantEnd := time.Date(2026, 11, 2, 7, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_LosAngeles(t *testing.T) {
	// America/Los_Angeles is UTC-8 in winter (PST)
	start, end, err := GetDateBounds("2026-01-22", "America/Los_Angeles")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-01-22 03:00:00 PST = 2026-01-22 11:00:00 UTC
	wantStart := time.Date(2026, 1, 22, 11, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-01-23 02:59:59 PST = 2026-01-23 10:59:59 UTC
	wantEnd := time.Date(2026, 1, 23, 10, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_London(t *testing.T) {
	// Europe/London is UTC+0 in winter (GMT)
	start, end, err := GetDateBounds("2026-01-22", "Europe/London")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-01-22 03:00:00 GMT = 2026-01-22 03:00:00 UTC
	wantStart := time.Date(2026, 1, 22, 3, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-01-23 02:59:59 GMT = 2026-01-23 02:59:59 UTC
	wantEnd := time.Date(2026, 1, 23, 2, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_LondonBST(t *testing.T) {
	// Europe/London is UTC+1 in summer (BST)
	start, end, err := GetDateBounds("2026-07-22", "Europe/London")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-07-22 03:00:00 BST = 2026-07-22 02:00:00 UTC
	wantStart := time.Date(2026, 7, 22, 2, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-07-23 02:59:59 BST = 2026-07-23 01:59:59 UTC
	wantEnd := time.Date(2026, 7, 23, 1, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_UTC(t *testing.T) {
	start, end, err := GetDateBounds("2026-01-22", "UTC")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-01-22 03:00:00 UTC
	wantStart := time.Date(2026, 1, 22, 3, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-01-23 02:59:59 UTC
	wantEnd := time.Date(2026, 1, 23, 2, 59, 59, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_InvalidTimezone(t *testing.T) {
	_, _, err := GetDateBounds("2026-01-22", "Invalid/Timezone")
	if err == nil {
		t.Fatal("GetDateBounds() with invalid timezone should return error")
	}
}

func TestGetDateBounds_InvalidDate(t *testing.T) {
	testCases := []struct {
		name string
		date string
	}{
		{"wrong format", "01-22-2026"},
		{"not a date", "not-a-date"},
		{"incomplete date", "2026-01"},
		{"empty string", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := GetDateBounds(tc.date, "America/New_York")
			if err == nil {
				t.Errorf("GetDateBounds(%q) should return error for invalid date", tc.date)
			}
		})
	}
}

func TestGetDateBounds_ReturnedTimesAreUTC(t *testing.T) {
	start, end, err := GetDateBounds("2026-01-22", "America/New_York")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	if start.Location() != time.UTC {
		t.Errorf("start location = %v, want UTC", start.Location())
	}
	if end.Location() != time.UTC {
		t.Errorf("end location = %v, want UTC", end.Location())
	}
}
