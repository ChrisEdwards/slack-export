package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chrisedwards/slack-export/internal/slack"
)

// FindSlackdump locates the slackdump binary.
// If configPath is non-empty, it validates that path exists.
// Otherwise, it searches PATH for "slackdump".
func FindSlackdump(configPath string) (string, error) {
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		return "", fmt.Errorf("slackdump not found at %s", configPath)
	}
	path, err := exec.LookPath("slackdump")
	if err != nil {
		return "", fmt.Errorf("slackdump not found in PATH - install from https://github.com/rusq/slackdump")
	}
	return path, nil
}

// Archive runs slackdump archive with the given channels and time range.
// It creates a temp directory, runs slackdump there, and returns the path to
// the created archive directory (slackdump_YYYYMMDD_HHMMSS/).
// The caller is responsible for cleaning up with os.RemoveAll(filepath.Dir(archiveDir)).
func Archive(
	ctx context.Context,
	slackdumpPath string,
	channelIDs []string,
	timeFrom, timeTo time.Time,
) (string, error) {
	if len(channelIDs) == 0 {
		return "", errors.New("no channels to archive")
	}

	args := []string{
		"archive",
		"-files=false",
		fmt.Sprintf("-time-from=%s", timeFrom.UTC().Format(time.RFC3339)),
		fmt.Sprintf("-time-to=%s", timeTo.UTC().Format(time.RFC3339)),
	}
	args = append(args, channelIDs...)

	tmpDir, err := os.MkdirTemp("", "slack-export-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	// #nosec G204 -- slackdumpPath comes from user configuration, not untrusted input
	cmd := exec.CommandContext(ctx, slackdumpPath, args...)
	cmd.Dir = tmpDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("slackdump archive failed: %w\nOutput: %s", err, output)
	}

	archiveDir, err := findSlackdumpDir(tmpDir)
	if err != nil {
		return "", fmt.Errorf("%w\nOutput: %s", err, output)
	}

	return archiveDir, nil
}

// findSlackdumpDir locates the slackdump_* directory in the given parent.
func findSlackdumpDir(parentDir string) (string, error) {
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return "", fmt.Errorf("reading temp dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "slackdump_") {
			return filepath.Join(parentDir, entry.Name()), nil
		}
	}

	return "", errors.New("slackdump did not create expected output directory")
}

// SlackdumpRunner wraps the slackdump CLI for message export.
type SlackdumpRunner struct {
	binPath string
}

// NewSlackdumpRunner creates a runner with the optional binary path.
// If binPath is empty, it will search PATH for slackdump.
func NewSlackdumpRunner(binPath string) *SlackdumpRunner {
	return &SlackdumpRunner{binPath: binPath}
}

// ExportChannel exports messages for a single channel on the given date.
func (r *SlackdumpRunner) ExportChannel(ctx context.Context, ch slack.Channel, date time.Time, outputDir string) error {
	// TODO: Implement slackdump CLI invocation
	return nil
}
