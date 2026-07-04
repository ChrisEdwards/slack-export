package export

import (
	"fmt"
	"time"
)

func datesInRange(from, to, timezone string) ([]string, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone: %w", err)
	}
	start, err := time.ParseInLocation("2006-01-02", from, loc)
	if err != nil {
		return nil, fmt.Errorf("parsing from date: %w", err)
	}
	end, err := time.ParseInLocation("2006-01-02", to, loc)
	if err != nil {
		return nil, fmt.Errorf("parsing to date: %w", err)
	}
	if start.After(end) {
		return nil, fmt.Errorf("from date %s cannot be after to date %s", from, to)
	}
	var dates []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	return dates, nil
}
