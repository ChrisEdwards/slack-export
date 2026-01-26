package export

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
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

// FormatText runs slackdump format text to convert an archive to text files.
// It returns the path to the created .zip file containing .txt files for each channel.
func FormatText(ctx context.Context, slackdumpPath, archiveDir string) (string, error) {
	// #nosec G204 -- slackdumpPath comes from user configuration, not untrusted input
	cmd := exec.CommandContext(ctx, slackdumpPath, "format", "text", archiveDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("slackdump format text failed: %w\nOutput: %s", err, output)
	}

	parentDir := filepath.Dir(archiveDir)
	zipPath, err := findZipFile(parentDir)
	if err != nil {
		return "", fmt.Errorf("%w\nOutput: %s", err, output)
	}

	return zipPath, nil
}

// ExtractAndProcess extracts the zip file and organizes files into the final output structure.
// It creates a date directory under outputDir (e.g., slack-logs/2026-01-22/) and renames
// files from channel ID format (C123456.txt) to dated channel name format (2026-01-22-engineering.md).
// The channelNames map provides ID to name mappings; unknown IDs fall back to the raw ID.
func ExtractAndProcess(zipPath, outputDir, date string, channelNames map[string]string) error {
	dateDir := filepath.Join(outputDir, date)
	if err := os.MkdirAll(dateDir, 0750); err != nil {
		return fmt.Errorf("creating date directory: %w", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip file: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if err := extractAndRenameFile(f, dateDir, date, channelNames); err != nil {
			return err
		}
	}
	return nil
}

// extractAndRenameFile extracts a single file from the zip and renames it appropriately.
func extractAndRenameFile(
	f *zip.File,
	dateDir, date string,
	channelNames map[string]string,
) error {
	// Skip directories
	if f.FileInfo().IsDir() {
		return nil
	}

	// Extract channel ID from filename (e.g., "C123456.txt")
	baseName := filepath.Base(f.Name)
	channelID := strings.TrimSuffix(baseName, ".txt")

	// Get channel name for filename
	name := channelID
	if channelNames != nil {
		if n, ok := channelNames[channelID]; ok && n != "" {
			name = n
		}
	}

	// Create output: YYYY-MM-DD-channelname.md
	outName := fmt.Sprintf("%s-%s.md", date, name)
	outPath := filepath.Join(dateDir, outName)

	return extractFile(f, outPath)
}

// extractFile extracts a single file from a zip archive to the given destination path.
func extractFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	// #nosec G304 -- destPath is constructed from trusted date/channel data, not user input
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating output file %s: %w", destPath, err)
	}
	defer func() { _ = outFile.Close() }()

	// #nosec G110 -- zip bomb protection not needed for slackdump output
	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("extracting %s: %w", f.Name, err)
	}

	return nil
}

// findZipFile locates the .zip file in the given directory.
func findZipFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	return "", errors.New("slackdump did not create expected zip file")
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
