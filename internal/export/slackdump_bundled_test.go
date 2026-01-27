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

func TestFindSlackdump_PrefersBundledOverPATH(t *testing.T) {
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

	// Create PATH binary
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
	if got != bundledPath {
		t.Errorf("FindSlackdump() = %q, want bundled %q (not PATH)", got, bundledPath)
	}
}

func TestFindSlackdump_FallsBackToPATH(t *testing.T) {
	// Empty bundled directory
	bundledDir := t.TempDir()

	// Create PATH binary
	pathDir := t.TempDir()
	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}
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
	if got != pathBin {
		t.Errorf("FindSlackdump() = %q, want PATH %q", got, pathBin)
	}
}
