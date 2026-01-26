package export

import (
	"fmt"
	"time"
)

// GetDateBounds calculates the UTC start and end times for a given date in the specified timezone.
// The start time is midnight (00:00:00) in the target timezone converted to UTC.
// The end time is the last nanosecond of the day (23:59:59.999999999) in the target timezone converted to UTC.
// This correctly handles DST transitions by constructing the end time explicitly in local time.
func GetDateBounds(date, timezone string) (start, end time.Time, err error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid timezone: %w", err)
	}

	t, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date: %w", err)
	}

	start = t.UTC()

	// Construct end of day explicitly in local time to handle DST transitions correctly.
	// Using time.Date ensures we get 23:59:59.999999999 in local time regardless of DST shifts.
	endLocal := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, loc)
	end = endLocal.UTC()

	return start, end, nil
}
