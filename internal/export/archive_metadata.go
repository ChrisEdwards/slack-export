package export

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rusq/slackdump/v4/source"
	_ "modernc.org/sqlite"
)

func archiveCoverageStart(archiveDir string) (time.Time, error) {
	db, err := sql.Open("sqlite", filepath.Join(archiveDir, source.DefaultDBFile))
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = db.Close() }()

	var raw sql.NullString
	err = db.QueryRow(`
		SELECT MIN(FROM_TS)
		FROM SESSION
		WHERE MODE = 'archive'
		  AND FINISHED = 1
		  AND FROM_TS IS NOT NULL
	`).Scan(&raw)
	if err != nil {
		return time.Time{}, err
	}
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return time.Time{}, nil
	}
	return parseArchiveTimestamp(raw.String)
}

func parseArchiveTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("parsing archive timestamp %q: %w", value, lastErr)
}

func archiveExists(archiveDir string) bool {
	_, err := os.Stat(filepath.Join(archiveDir, source.DefaultDBFile))
	return err == nil
}

func fullSweepDue(archiveDir, interval string, now time.Time) bool {
	duration, err := parseFriendlyDuration(interval)
	if err != nil || duration == 0 {
		return false
	}
	data, err := os.ReadFile(fullSweepPath(archiveDir))
	if err != nil {
		return true
	}
	last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return true
	}
	return !now.Before(last.Add(duration))
}

func markFullSweep(archiveDir string, now time.Time) error {
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		return fmt.Errorf("creating archive metadata directory: %w", err)
	}
	if err := os.WriteFile(fullSweepPath(archiveDir), []byte(now.Format(time.RFC3339)), 0600); err != nil {
		return fmt.Errorf("writing full sweep marker: %w", err)
	}
	return nil
}

func fullSweepPath(archiveDir string) string {
	return filepath.Join(archiveDir, ".slack-export-last-full-sweep")
}
