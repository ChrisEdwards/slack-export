package export

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chrisedwards/slack-export/internal/slack"
	"github.com/rusq/slackdump/v4/source"
	_ "modernc.org/sqlite"
)

type renderTarget struct {
	channelID string
	date      string
}

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

func writtenResumeRenderTargets(archiveDir, timezone string) ([]renderTarget, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone: %w", err)
	}
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
		SELECT DISTINCT M.CHANNEL_ID, M.TS
		FROM MESSAGE M
		JOIN CHUNK C ON C.ID = M.CHUNK_ID
		JOIN latest_resume S ON S.ID = C.SESSION_ID
		WHERE M.CHANNEL_ID IS NOT NULL
		  AND TRIM(M.CHANNEL_ID) <> ''
		  AND M.TS IS NOT NULL
		  AND TRIM(M.TS) <> ''
		ORDER BY M.CHANNEL_ID, M.TS
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	seen := make(map[renderTarget]bool)
	var targets []renderTarget
	for rows.Next() {
		var channelID, ts string
		if err := rows.Scan(&channelID, &ts); err != nil {
			return nil, err
		}
		date, err := workdayDateForSlackTS(ts, loc)
		if err != nil {
			return nil, err
		}
		target := renderTarget{channelID: channelID, date: date}
		if !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}

func workdayDateForSlackTS(ts string, loc *time.Location) (string, error) {
	posted, err := slack.ParseSlackTS(ts)
	if err != nil {
		return "", fmt.Errorf("parsing message timestamp %q: %w", ts, err)
	}
	local := posted.In(loc)
	if local.Hour() < 3 {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format("2006-01-02"), nil
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

func markSweepSuccess(archiveDir string, now time.Time) error {
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		return fmt.Errorf("creating archive metadata directory: %w", err)
	}
	if err := os.WriteFile(sweepSuccessPath(archiveDir), []byte(now.Format(time.RFC3339)), 0600); err != nil {
		return fmt.Errorf("writing sweep success marker: %w", err)
	}
	return nil
}

func lastSweepSuccess(archiveDir string) (time.Time, bool, error) {
	data, err := os.ReadFile(sweepSuccessPath(archiveDir))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}, false, nil
	}
	return last, true, nil
}

func sweepSuccessPath(archiveDir string) string {
	return filepath.Join(archiveDir, ".slack-export-last-success")
}
