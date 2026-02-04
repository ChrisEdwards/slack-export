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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chrisedwards/slack-export/internal/slack"
)

// MinSlackdumpVersion is the minimum version that has the bug fix from PR #444.
const MinSlackdumpVersion = "3.1.13"

// CompareVersions compares two semver strings (X.Y.Z format).
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// Returns error if either version is malformed.
func CompareVersions(a, b string) (int, error) {
	parseVersion := func(v string) ([3]int, error) {
		parts := strings.Split(v, ".")
		if len(parts) != 3 {
			return [3]int{}, fmt.Errorf("invalid version format: %q (expected X.Y.Z)", v)
		}
		var result [3]int
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return [3]int{}, fmt.Errorf("invalid version segment %q: %w", p, err)
			}
			result[i] = n
		}
		return result, nil
	}

	va, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	vb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}

	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return -1, nil
		}
		if va[i] > vb[i] {
			return 1, nil
		}
	}
	return 0, nil
}

// findSlackdumpInDir looks for a slackdump binary in the given directory.
// Returns the full path if found, error otherwise.
func findSlackdumpInDir(dir string) (string, error) {
	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}

	bundled := filepath.Join(dir, binaryName)
	if _, err := os.Stat(bundled); err == nil {
		return bundled, nil
	}
	return "", fmt.Errorf("slackdump not found in %s", dir)
}

// testExeDir is used in tests to override os.Executable() directory.
// Empty string means use the real executable directory.
var testExeDir string

// FindSlackdump locates the slackdump binary.
// Priority order:
// 1. Bundled binary next to the executable
// 2. System PATH (fallback for development)
func FindSlackdump() (string, error) {
	// Try bundled binary first
	var exeDir string
	if testExeDir != "" {
		exeDir = testExeDir
	} else if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}

	if exeDir != "" {
		if path, err := findSlackdumpInDir(exeDir); err == nil {
			return path, nil
		}
	}

	// Fall back to PATH
	path, err := exec.LookPath("slackdump")
	if err != nil {
		return "", errors.New("slackdump not found - ensure it's installed alongside slack-export")
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

	// slackdump expects datetime without timezone suffix (e.g., "2006-01-02T15:04:05")
	const slackdumpTimeFormat = "2006-01-02T15:04:05"
	args := []string{
		"archive",
		"-files=false",
		fmt.Sprintf("-time-from=%s", timeFrom.UTC().Format(slackdumpTimeFormat)),
		fmt.Sprintf("-time-to=%s", timeTo.UTC().Format(slackdumpTimeFormat)),
	}
	args = append(args, channelIDs...)

	tmpDir, err := os.MkdirTemp("", "slack-export-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	// #nosec G204 -- slackdumpPath comes from user configuration, not untrusted input
	cmd := exec.CommandContext(ctx, slackdumpPath, args...)
	cmd.Dir = tmpDir

	// Debug: show the command being run
	fmt.Printf("EXECUTING: %s %s\n", slackdumpPath, strings.Join(args, " "))

	// Stream output in real-time so we can see progress
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("slackdump archive failed: %w", err)
	}

	archiveDir, err := findSlackdumpDir(tmpDir)
	if err != nil {
		return "", err
	}

	return archiveDir, nil
}

// FormatText runs slackdump format text to convert an archive to text files.
// It returns the path to the created .zip file containing .txt files for each channel.
func FormatText(ctx context.Context, slackdumpPath, archiveDir string) (string, error) {
	// #nosec G204 -- slackdumpPath comes from user configuration, not untrusted input
	cmd := exec.CommandContext(ctx, slackdumpPath, "format", "text", archiveDir)
	// Run in the parent directory so the zip file is created there
	cmd.Dir = filepath.Dir(archiveDir)

	// Debug: show the command being run
	fmt.Printf("EXECUTING: %s format text %s\n", slackdumpPath, archiveDir)

	// Stream output in real-time so we can see which channel fails
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("slackdump format text failed: %w", err)
	}

	parentDir := filepath.Dir(archiveDir)
	zipPath, err := findZipFile(parentDir)
	if err != nil {
		return "", err
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

// metadataFiles are slackdump output files that should be skipped (not channel exports).
var metadataFiles = map[string]bool{
	"channels.txt": true,
	"users.txt":    true,
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

	baseName := filepath.Base(f.Name)

	// Skip slackdump metadata files
	if metadataFiles[baseName] {
		return nil
	}

	// Extract channel ID from filename (e.g., "C123456.txt")
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
