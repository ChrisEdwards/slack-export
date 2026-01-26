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

	// 2026-01-22 00:00:00 EST = 2026-01-22 05:00:00 UTC
	wantStart := time.Date(2026, 1, 22, 5, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-01-22 23:59:59.999999999 EST = 2026-01-23 04:59:59.999999999 UTC
	wantEnd := time.Date(2026, 1, 23, 4, 59, 59, 999999999, time.UTC)
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

	// 2026-07-22 00:00:00 EDT = 2026-07-22 04:00:00 UTC
	wantStart := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-07-22 23:59:59.999999999 EDT = 2026-07-23 03:59:59.999999999 UTC
	wantEnd := time.Date(2026, 7, 23, 3, 59, 59, 999999999, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_DSTTransitionSpring(t *testing.T) {
	// US DST spring transition 2026: March 8 at 2:00 AM local time
	// Clocks spring forward from 2:00 AM to 3:00 AM
	// So this day only has 23 hours in local time
	start, end, err := GetDateBounds("2026-03-08", "America/New_York")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-03-08 00:00:00 EST = 2026-03-08 05:00:00 UTC (still EST before transition)
	wantStart := time.Date(2026, 3, 8, 5, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// The end of 2026-03-08 in local time is 23:59:59.999999999 EDT (after DST transition)
	// 2026-03-08 23:59:59.999999999 EDT (UTC-4) = 2026-03-09 03:59:59.999999999 UTC
	wantEnd := time.Date(2026, 3, 9, 3, 59, 59, 999999999, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_DSTTransitionFall(t *testing.T) {
	// US DST fall transition 2026: November 1 at 2:00 AM local time
	// Clocks fall back from 2:00 AM to 1:00 AM
	// So this day has 25 hours in wall-clock time
	start, end, err := GetDateBounds("2026-11-01", "America/New_York")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	// 2026-11-01 00:00:00 EDT = 2026-11-01 04:00:00 UTC (still EDT before transition)
	wantStart := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// End of day: 2026-11-01 23:59:59.999999999 EST (UTC-5) = 2026-11-02 04:59:59.999999999 UTC
	wantEnd := time.Date(2026, 11, 2, 4, 59, 59, 999999999, time.UTC)
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

	// 2026-01-22 00:00:00 PST = 2026-01-22 08:00:00 UTC
	wantStart := time.Date(2026, 1, 22, 8, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-01-22 23:59:59.999999999 PST = 2026-01-23 07:59:59.999999999 UTC
	wantEnd := time.Date(2026, 1, 23, 7, 59, 59, 999999999, time.UTC)
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

	// 2026-01-22 00:00:00 GMT = 2026-01-22 00:00:00 UTC
	wantStart := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-01-22 23:59:59.999999999 GMT = 2026-01-22 23:59:59.999999999 UTC
	wantEnd := time.Date(2026, 1, 22, 23, 59, 59, 999999999, time.UTC)
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

	// 2026-07-22 00:00:00 BST = 2026-07-21 23:00:00 UTC
	wantStart := time.Date(2026, 7, 21, 23, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	// 2026-07-22 23:59:59.999999999 BST = 2026-07-22 22:59:59.999999999 UTC
	wantEnd := time.Date(2026, 7, 22, 22, 59, 59, 999999999, time.UTC)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestGetDateBounds_UTC(t *testing.T) {
	start, end, err := GetDateBounds("2026-01-22", "UTC")
	if err != nil {
		t.Fatalf("GetDateBounds() error = %v", err)
	}

	wantStart := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}

	wantEnd := time.Date(2026, 1, 22, 23, 59, 59, 999999999, time.UTC)
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
