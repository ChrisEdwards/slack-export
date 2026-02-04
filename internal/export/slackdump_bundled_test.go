package export

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindSlackdump_BundledBinary(t *testing.T) {
	// Create a fake "executable" directory with bundled slackdump
	tmpDir := t.TempDir()

	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}

	bundledPath := filepath.Join(tmpDir, binaryName)
	if err := os.WriteFile(bundledPath, []byte("fake bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	// Override executable path for test
	got, err := findSlackdumpInDir(tmpDir)
	if err != nil {
		t.Fatalf("findSlackdumpInDir() error = %v", err)
	}
	if got != bundledPath {
		t.Errorf("findSlackdumpInDir() = %q, want %q", got, bundledPath)
	}
}

func TestFindSlackdump_BundledBinaryNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// No bundled binary, should return error
	_, err := findSlackdumpInDir(tmpDir)
	if err == nil {
		t.Fatal("findSlackdumpInDir() with no bundled binary should return error")
	}
}

func TestFindSlackdump_UsesBundledWhenPathVersionCheckFails(t *testing.T) {
	// Create bundled binary
	bundledDir := t.TempDir()
	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}
	bundledPath := filepath.Join(bundledDir, binaryName)
	if err := os.WriteFile(bundledPath, []byte("bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create PATH binary that doesn't report a valid version (not executable script)
	pathDir := t.TempDir()
	pathBin := filepath.Join(pathDir, binaryName)
	if err := os.WriteFile(pathBin, []byte("path"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	// Set the executable directory override for testing
	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	// PATH binary can't report version, so falls back to bundled
	if got != bundledPath {
		t.Errorf("FindSlackdump() = %q, want bundled %q (PATH version check failed)", got, bundledPath)
	}
}

func TestFindSlackdump_UsesPathWhenVersionSufficientAndNoBundled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Empty bundled directory (no bundled binary)
	bundledDir := t.TempDir()

	// Create PATH binary with sufficient version
	pathDir := t.TempDir()
	pathBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 3.2.0 (commit: test1234) built on: 2024-01-01"
`
	if err := os.WriteFile(pathBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	// Set the executable directory override for testing
	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != pathBin {
		t.Errorf("FindSlackdump() = %q, want PATH %q", got, pathBin)
	}
}

func TestFindSlackdump_SystemVersionSufficient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a mock slackdump that reports version 3.2.0 (above minimum)
	pathDir := t.TempDir()
	mockBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 3.2.0 (commit: test1234) built on: 2024-01-01"
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Create bundled binary (should NOT be used)
	bundledDir := t.TempDir()
	bundledBin := filepath.Join(bundledDir, "slackdump")
	if err := os.WriteFile(bundledBin, []byte("#!/bin/sh\necho bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", pathDir)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != mockBin {
		t.Errorf("FindSlackdump() = %q, want system PATH %q (version sufficient)", got, mockBin)
	}
}

func TestFindSlackdump_SystemVersionInsufficient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a mock slackdump that reports version 3.1.12 (below minimum)
	pathDir := t.TempDir()
	mockBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 3.1.12 (commit: old12345) built on: 2023-01-01"
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Create bundled binary (should be used as fallback)
	bundledDir := t.TempDir()
	bundledBin := filepath.Join(bundledDir, "slackdump")
	if err := os.WriteFile(bundledBin, []byte("#!/bin/sh\necho bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", pathDir)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != bundledBin {
		t.Errorf("FindSlackdump() = %q, want bundled %q (system version too old)", got, bundledBin)
	}
}

func TestFindSlackdump_SystemVersionUnknown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a mock slackdump that reports unknown version (dev build)
	pathDir := t.TempDir()
	mockBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump unknown (commit: unknown) built on: unknown"
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Create bundled binary (should be used as fallback)
	bundledDir := t.TempDir()
	bundledBin := filepath.Join(bundledDir, "slackdump")
	if err := os.WriteFile(bundledBin, []byte("#!/bin/sh\necho bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", pathDir)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != bundledBin {
		t.Errorf("FindSlackdump() = %q, want bundled %q (system version unknown)", got, bundledBin)
	}
}
