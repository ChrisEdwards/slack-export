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

func changedResumeChannelIDs(archiveDir string) ([]string, error) {
	db, err := sql.Open("sqlite", filepath.Join(archiveDir, source.DefaultDBFile))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`
		WITH latest_resume AS (
			SELECT ID
			FROM SESSION
			WHERE MODE = 'resume'
			  AND FINISHED = 1
			ORDER BY ID DESC
			LIMIT 1
		)
		SELECT DISTINCT C.CHANNEL_ID
		FROM CHUNK C
		JOIN latest_resume S ON S.ID = C.SESSION_ID
		WHERE C.NUM_REC > 0
		  AND C.CHANNEL_ID IS NOT NULL
		  AND TRIM(C.CHANNEL_ID) <> ''
		ORDER BY C.CHANNEL_ID
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
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
