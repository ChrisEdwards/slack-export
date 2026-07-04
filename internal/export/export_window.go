package export

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (e *Exporter) renderWindow(now time.Time) (string, string, error) {
	loc, err := time.LoadLocation(e.cfg.Timezone)
	if err != nil {
		return "", "", fmt.Errorf("loading timezone: %w", err)
	}
	lookback, err := parseFriendlyDuration(e.cfg.Lookback)
	if err != nil {
		return "", "", fmt.Errorf("parsing lookback: %w", err)
	}
	days := int(lookback.Hours()/24) + 1
	to := now.In(loc)
	from := to.AddDate(0, 0, -days)
	return from.Format("2006-01-02"), to.Format("2006-01-02"), nil
}

func (e *Exporter) seedDate(now time.Time) (string, error) {
	if e.cfg.SeedDate != "" {
		if _, _, err := GetDateBounds(e.cfg.SeedDate, e.cfg.Timezone); err != nil {
			return "", fmt.Errorf("invalid seed_date: %w", err)
		}
		return e.cfg.SeedDate, nil
	}
	if date, err := findEarliestExportDate(e.cfg.OutputDir); err != nil {
		return "", err
	} else if date != "" {
		return date, nil
	}
	loc, err := time.LoadLocation(e.cfg.Timezone)
	if err != nil {
		return "", fmt.Errorf("loading timezone: %w", err)
	}
	return now.In(loc).Format("2006-01-02"), nil
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func sanitizePathPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "workspace"
	}
	return b.String()
}

var exportDateDirPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func findEarliestExportDate(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var earliest string
	for _, entry := range entries {
		if !entry.IsDir() || !exportDateDirPattern.MatchString(entry.Name()) {
			continue
		}
		if earliest == "" || entry.Name() < earliest {
			earliest = entry.Name()
		}
	}
	return earliest, nil
}

func parseFriendlyDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	if days, ok, err := parseDayDuration(value); ok || err != nil {
		return days, err
	}
	return time.ParseDuration(value)
}

func parseDayDuration(value string) (time.Duration, bool, error) {
	if !strings.HasSuffix(value, "d") {
		return 0, false, nil
	}
	n, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
	if err != nil {
		return 0, true, err
	}
	return time.Duration(n) * 24 * time.Hour, true, nil
}
