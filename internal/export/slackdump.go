package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chrisedwards/slack-export/internal/slack"
)

// MinSlackdumpVersion is the minimum version with database archive resume support.
const MinSlackdumpVersion = "4.4.1"

// slackdumpTimeFormat is the timestamp layout slackdump accepts for
// -time-from and for entity time bounds (ID,oldest[,latest]).
const slackdumpTimeFormat = "2006-01-02T15:04:05"

// parseSlackdumpVersion extracts the version from slackdump version output.
// Expected format: "Slackdump 3.1.13 (commit: abc12345) built on: 2024-01-15"
func parseSlackdumpVersion(output string) (string, error) {
	// Look for "Slackdump " prefix
	const prefix = "Slackdump "
	if !strings.HasPrefix(output, prefix) {
		return "", fmt.Errorf("failed to parse slackdump version: unexpected output format")
	}

	// Extract version between "Slackdump " and " ("
	rest := output[len(prefix):]
	idx := strings.Index(rest, " (")
	if idx == -1 {
		return "", fmt.Errorf("failed to parse slackdump version: missing version delimiter")
	}

	version := rest[:idx]
	if version == "unknown" {
		return "", fmt.Errorf("slackdump version is unknown (development build)")
	}

	return version, nil
}

// SlackdumpVersion runs slackdump version and returns the parsed version string.
func SlackdumpVersion(binaryPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// #nosec G204 -- binaryPath comes from FindSlackdump, not untrusted input
	cmd := exec.CommandContext(ctx, binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run slackdump version: %w", err)
	}

	return parseSlackdumpVersion(strings.TrimSpace(string(output)))
}

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
// 1. System PATH if version >= MinSlackdumpVersion
// 2. Bundled binary next to the executable
func FindSlackdump() (string, error) {
	// Try system PATH first, check version
	if path, err := exec.LookPath("slackdump"); err == nil {
		version, verr := SlackdumpVersion(path)
		if verr == nil {
			cmp, cerr := CompareVersions(version, MinSlackdumpVersion)
			if cerr == nil && cmp >= 0 {
				return path, nil
			}
			// Version is below minimum, fall back to bundled
			fmt.Printf("System slackdump version %s is below minimum %s, using bundled binary\n",
				version, MinSlackdumpVersion)
		}
		// Version check failed (unknown or parse error), fall back to bundled
	}

	// Try bundled binary
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

	return "", errors.New("slackdump not found - ensure it's installed alongside slack-export")
}

// ResumeOptions configures a slackdump v4 resume run.
type ResumeOptions struct {
	Lookback            string
	SkipStaleThreads    string
	SkipStaleChannels   string
	SkipCompleteThreads bool
	Dedupe              bool
}

// BootstrapArchive creates a persistent slackdump v4 database archive.
func BootstrapArchive(
	ctx context.Context,
	slackdumpPath string,
	archiveDir string,
	channelIDs []string,
	timeFrom time.Time,
) error {
	if len(channelIDs) == 0 {
		return errors.New("no channels to archive")
	}

	args := []string{
		"archive",
		"-files=false",
		"-y",
		"-o", archiveDir,
		fmt.Sprintf("-time-from=%s", timeFrom.UTC().Format(slackdumpTimeFormat)),
	}
	args = append(args, channelIDs...)

	return runSlackdump(ctx, slackdumpPath, args, "slackdump archive failed")
}

// ResumeArchive refreshes a persistent slackdump v4 database archive.
func ResumeArchive(
	ctx context.Context,
	slackdumpPath string,
	archiveDir string,
	entityArgs []string,
	opts ResumeOptions,
) error {
	args := []string{"resume", "-threads"}
	if opts.Lookback != "" {
		args = append(args, "-lookback", toISODuration(opts.Lookback))
	}
	if opts.SkipStaleThreads != "" {
		args = append(args, "-skip-stale-threads", toISODuration(opts.SkipStaleThreads))
	}
	if opts.SkipStaleChannels != "" {
		args = append(args, "-skip-stale-channels", toISODuration(opts.SkipStaleChannels))
	}
	if opts.SkipCompleteThreads {
		args = append(args, "-skip-complete-threads")
	}
	if opts.Dedupe {
		args = append(args, "-dedupe")
	}
	args = append(args, archiveDir)
	args = append(args, entityArgs...)

	return runSlackdump(ctx, slackdumpPath, args, "slackdump resume failed")
}

func toISODuration(value string) string {
	if value == "" || strings.HasPrefix(strings.ToLower(value), "p") {
		return value
	}
	return "p" + value
}

func runSlackdump(ctx context.Context, slackdumpPath string, args []string, errPrefix string) error {
	// #nosec G204 -- slackdumpPath comes from FindSlackdump, not untrusted input
	cmd := exec.CommandContext(ctx, slackdumpPath, args...)
	fmt.Printf("EXECUTING: %s %s\n", slackdumpPath, strings.Join(args, " "))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", errPrefix, err)
	}
	return nil
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
