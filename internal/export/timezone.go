package export

import (
	"fmt"
	"time"
)

// GetDateBounds calculates the UTC start and end times for a given date in the specified timezone.
// Uses a "work day" definition: 3am on the date to 2:59:59am on the next day.
// This matches typical work patterns where late-night activity belongs to the previous day.
// This correctly handles DST transitions by constructing times explicitly in local time.
func GetDateBounds(date, timezone string) (start, end time.Time, err error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid timezone: %w", err)
	}

	t, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date: %w", err)
	}

	// Start at 3am on the export date
	startLocal := time.Date(t.Year(), t.Month(), t.Day(), 3, 0, 0, 0, loc)
	start = startLocal.UTC()

	// End at 2:59:59am on the next day
	nextDay := t.AddDate(0, 0, 1)
	endLocal := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 2, 59, 59, 0, loc)
	end = endLocal.UTC()

	return start, end, nil
}
