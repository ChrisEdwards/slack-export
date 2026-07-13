package export

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFindSlackdump_FromPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a temp dir with a mock slackdump that reports sufficient version
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 4.4.1 (commit: test1234) built on: 2026-07-01"
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Use empty exe dir so it falls back to PATH
	oldExeDir := testExeDir
	testExeDir = t.TempDir() // empty dir
	defer func() { testExeDir = oldExeDir }()

	// Prepend tmpDir to PATH
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != fakeBin {
		t.Errorf("FindSlackdump() = %q, want %q", got, fakeBin)
	}
}

func TestParseSlackdumpVersion_V441(t *testing.T) {
	got, err := parseSlackdumpVersion("Slackdump 4.4.1 (commit: abc12345) built on: 2026-07-01")
	if err != nil {
		t.Fatalf("parseSlackdumpVersion() error = %v", err)
	}
	if got != "4.4.1" {
		t.Errorf("parseSlackdumpVersion() = %q, want 4.4.1", got)
	}
}

func TestFindSlackdump_NotFound(t *testing.T) {
	// Set both exe dir and PATH to empty dirs
	oldExeDir := testExeDir
	testExeDir = t.TempDir()
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", t.TempDir())

	_, err := FindSlackdump()
	if err == nil {
		t.Fatal("FindSlackdump() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should mention 'not found'", err.Error())
	}
}

func TestBootstrapArchive_EmptyChannels(t *testing.T) {
	ctx := context.Background()
	timeFrom := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)

	err := BootstrapArchive(ctx, "/nonexistent/slackdump", t.TempDir(), nil, timeFrom, "")
	if err == nil {
		t.Fatal("BootstrapArchive() with empty channels should return error")
	}
	if !strings.Contains(err.Error(), "no channels to archive") {
		t.Errorf("error %q should mention 'no channels to archive'", err.Error())
	}

	err = BootstrapArchive(ctx, "/nonexistent/slackdump", t.TempDir(), []string{}, timeFrom, "")
	if err == nil {
		t.Fatal("BootstrapArchive() with empty slice should return error")
	}
}

func TestBootstrapArchive_CommandShape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "args.txt")
	fakeBin := filepath.Join(tmpDir, "slackdump")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + logPath + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	archiveDir := filepath.Join(tmpDir, "archive")
	apiConfigPath := filepath.Join(tmpDir, "slackdump-api-limits.yaml")
	seed := time.Date(2026, 1, 22, 8, 0, 0, 0, time.UTC)
	err := BootstrapArchive(context.Background(), fakeBin, archiveDir, []string{"C123", "D456"}, seed, apiConfigPath)
	if err != nil {
		t.Fatalf("BootstrapArchive() error = %v", err)
	}

	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	want := strings.Join([]string{
		"archive",
		"-files=false",
		"-y",
		"-o", archiveDir,
		"-time-from=2026-01-22T08:00:00",
		"-api-config", apiConfigPath,
		"C123",
		"D456",
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("BootstrapArchive args:\n%s\nwant:\n%s", string(got), want)
	}
}

func TestResumeArchive_CommandShape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "args.txt")
	fakeBin := filepath.Join(tmpDir, "slackdump")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + logPath + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	archiveDir := filepath.Join(tmpDir, "archive")
	opts := ResumeOptions{
		Lookback:            "7d",
		SkipStaleThreads:    "21d",
		SkipCompleteThreads: true,
		Dedupe:              true,
		APIConfigPath:       filepath.Join(tmpDir, "slackdump-api-limits.yaml"),
	}
	err := ResumeArchive(context.Background(), fakeBin, archiveDir, []string{"C123"}, opts)
	if err != nil {
		t.Fatalf("ResumeArchive() error = %v", err)
	}

	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	want := strings.Join([]string{
		"resume",
		"-threads",
		"-lookback", "p7d",
		"-skip-stale-threads", "p21d",
		"-skip-complete-threads",
		"-dedupe",
		"-api-config", opts.APIConfigPath,
		archiveDir,
		"C123",
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("ResumeArchive args:\n%s\nwant:\n%s", string(got), want)
	}
}

func TestWriteSweepAPIConfig_TunesTier3Pacing(t *testing.T) {
	path, err := writeSweepAPIConfig(t.TempDir())
	if err != nil {
		t.Fatalf("writeSweepAPIConfig() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading API config: %v", err)
	}
	text := string(got)
	for _, want := range []string{
		"[tier_3]",
		"  boost = 0",
		"  burst = 1",
		"  retries = 10",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sweep API config missing %q:\n%s", want, text)
		}
	}
}

func TestBootstrapArchive_InvalidBinary(t *testing.T) {
	ctx := context.Background()
	timeFrom := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)

	err := BootstrapArchive(ctx, "/nonexistent/slackdump", t.TempDir(), []string{"C123"}, timeFrom, "")
	if err == nil {
		t.Fatal("BootstrapArchive() with nonexistent binary should return error")
	}
	if !strings.Contains(err.Error(), "slackdump archive failed") {
		t.Errorf("error %q should mention 'slackdump archive failed'", err.Error())
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    int
		wantErr bool
	}{
		{"equal", "3.1.13", "3.1.13", 0, false},
		{"a greater patch", "3.1.14", "3.1.13", 1, false},
		{"a less patch", "3.1.12", "3.1.13", -1, false},
		{"a greater minor", "3.2.0", "3.1.13", 1, false},
		{"a less minor", "3.0.0", "3.1.13", -1, false},
		{"a greater major", "4.0.0", "3.1.13", 1, false},
		{"a less major", "2.9.99", "3.1.13", -1, false},
		{"malformed a", "abc", "3.1.13", 0, true},
		{"malformed b", "3.1.13", "xyz", 0, true},
		{"empty a", "", "3.1.13", 0, true},
		{"empty b", "3.1.13", "", 0, true},
		{"missing patch", "3.1", "3.1.13", 0, true},
		{"too many segments", "3.1.13.4", "3.1.13", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersions(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareVersions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("CompareVersions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSlackdumpVersion(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:   "valid version",
			output: "Slackdump 3.1.13 (commit: abc12345) built on: 2024-01-15",
			want:   "3.1.13",
		},
		{
			name:   "version with v prefix in output",
			output: "Slackdump 3.2.0 (commit: def67890) built on: 2024-02-01",
			want:   "3.2.0",
		},
		{
			name:        "unknown version",
			output:      "Slackdump unknown (commit: unknown) built on: unknown",
			wantErr:     true,
			errContains: "unknown",
		},
		{
			name:        "malformed output",
			output:      "not slackdump output",
			wantErr:     true,
			errContains: "parse",
		},
		{
			name:        "empty output",
			output:      "",
			wantErr:     true,
			errContains: "parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSlackdumpVersion(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSlackdumpVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if got != tt.want {
				t.Errorf("parseSlackdumpVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
