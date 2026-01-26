package export

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindSlackdump_ExplicitPath(t *testing.T) {
	// Create a temp file to simulate an existing binary
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "slackdump")
	if err := os.WriteFile(fakeBin, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := FindSlackdump(fakeBin)
	if err != nil {
		t.Fatalf("FindSlackdump(%q) error = %v", fakeBin, err)
	}
	if got != fakeBin {
		t.Errorf("FindSlackdump(%q) = %q, want %q", fakeBin, got, fakeBin)
	}
}

func TestFindSlackdump_ExplicitPathNotFound(t *testing.T) {
	nonexistent := "/nonexistent/path/slackdump"

	_, err := FindSlackdump(nonexistent)
	if err == nil {
		t.Fatalf("FindSlackdump(%q) expected error, got nil", nonexistent)
	}
	if !strings.Contains(err.Error(), "not found at") {
		t.Errorf("error %q should mention 'not found at'", err.Error())
	}
}

func TestFindSlackdump_FromPATH(t *testing.T) {
	// Create a temp dir with a fake slackdump binary
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "slackdump")
	if err := os.WriteFile(fakeBin, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	// Prepend tmpDir to PATH
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)

	got, err := FindSlackdump("")
	if err != nil {
		t.Fatalf("FindSlackdump(\"\") error = %v", err)
	}
	if got != fakeBin {
		t.Errorf("FindSlackdump(\"\") = %q, want %q", got, fakeBin)
	}
}

func TestFindSlackdump_NotInPATH(t *testing.T) {
	// Set PATH to empty dir so slackdump won't be found
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	_, err := FindSlackdump("")
	if err == nil {
		t.Fatal("FindSlackdump(\"\") expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error %q should mention 'not found in PATH'", err.Error())
	}
	if !strings.Contains(err.Error(), "github.com/rusq/slackdump") {
		t.Errorf("error %q should include install URL", err.Error())
	}
}

func TestArchive_EmptyChannels(t *testing.T) {
	ctx := context.Background()
	timeFrom := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)

	_, err := Archive(ctx, "/nonexistent/slackdump", nil, timeFrom, timeTo)
	if err == nil {
		t.Fatal("Archive() with empty channels should return error")
	}
	if !strings.Contains(err.Error(), "no channels to archive") {
		t.Errorf("error %q should mention 'no channels to archive'", err.Error())
	}

	_, err = Archive(ctx, "/nonexistent/slackdump", []string{}, timeFrom, timeTo)
	if err == nil {
		t.Fatal("Archive() with empty slice should return error")
	}
}

func TestArchive_InvalidBinary(t *testing.T) {
	ctx := context.Background()
	timeFrom := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)
	timeTo := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)

	_, err := Archive(ctx, "/nonexistent/slackdump", []string{"C123"}, timeFrom, timeTo)
	if err == nil {
		t.Fatal("Archive() with nonexistent binary should return error")
	}
	if !strings.Contains(err.Error(), "slackdump archive failed") {
		t.Errorf("error %q should mention 'slackdump archive failed'", err.Error())
	}
}

func TestFindSlackdumpDir_Found(t *testing.T) {
	tmpDir := t.TempDir()

	slackdumpDir := filepath.Join(tmpDir, "slackdump_20260122_120000")
	if err := os.MkdirAll(slackdumpDir, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := findSlackdumpDir(tmpDir)
	if err != nil {
		t.Fatalf("findSlackdumpDir() error = %v", err)
	}
	if got != slackdumpDir {
		t.Errorf("findSlackdumpDir() = %q, want %q", got, slackdumpDir)
	}
}

func TestFindSlackdumpDir_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some other files/dirs that shouldn't match
	os.MkdirAll(filepath.Join(tmpDir, "other_dir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "some_file.txt"), []byte("data"), 0644)

	_, err := findSlackdumpDir(tmpDir)
	if err == nil {
		t.Fatal("findSlackdumpDir() with no slackdump dir should return error")
	}
	if !strings.Contains(err.Error(), "did not create expected output directory") {
		t.Errorf("error %q should mention expected output directory", err.Error())
	}
}

func TestFindSlackdumpDir_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := findSlackdumpDir(tmpDir)
	if err == nil {
		t.Fatal("findSlackdumpDir() with empty dir should return error")
	}
}

func TestFindSlackdumpDir_NonexistentDir(t *testing.T) {
	_, err := findSlackdumpDir("/nonexistent/path")
	if err == nil {
		t.Fatal("findSlackdumpDir() with nonexistent path should return error")
	}
	if !strings.Contains(err.Error(), "reading temp dir") {
		t.Errorf("error %q should mention 'reading temp dir'", err.Error())
	}
}

func TestFormatText_InvalidBinary(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "slackdump_20260122_120000")

	_, err := FormatText(ctx, "/nonexistent/slackdump", archiveDir)
	if err == nil {
		t.Fatal("FormatText() with nonexistent binary should return error")
	}
	if !strings.Contains(err.Error(), "slackdump format text failed") {
		t.Errorf("error %q should mention 'slackdump format text failed'", err.Error())
	}
}

func TestFindZipFile_Found(t *testing.T) {
	tmpDir := t.TempDir()

	zipFile := filepath.Join(tmpDir, "slackdump_20260122_120000.zip")
	if err := os.WriteFile(zipFile, []byte("fake zip"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := findZipFile(tmpDir)
	if err != nil {
		t.Fatalf("findZipFile() error = %v", err)
	}
	if got != zipFile {
		t.Errorf("findZipFile() = %q, want %q", got, zipFile)
	}
}

func TestFindZipFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some other files/dirs that shouldn't match
	os.MkdirAll(filepath.Join(tmpDir, "other_dir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "some_file.txt"), []byte("data"), 0644)

	_, err := findZipFile(tmpDir)
	if err == nil {
		t.Fatal("findZipFile() with no zip should return error")
	}
	if !strings.Contains(err.Error(), "did not create expected zip file") {
		t.Errorf("error %q should mention expected zip file", err.Error())
	}
}

func TestFindZipFile_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := findZipFile(tmpDir)
	if err == nil {
		t.Fatal("findZipFile() with empty dir should return error")
	}
}

func TestFindZipFile_NonexistentDir(t *testing.T) {
	_, err := findZipFile("/nonexistent/path")
	if err == nil {
		t.Fatal("findZipFile() with nonexistent path should return error")
	}
	if !strings.Contains(err.Error(), "reading directory") {
		t.Errorf("error %q should mention 'reading directory'", err.Error())
	}
}

func TestFindZipFile_IgnoresDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with .zip suffix (edge case)
	zipDir := filepath.Join(tmpDir, "fake.zip")
	if err := os.MkdirAll(zipDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := findZipFile(tmpDir)
	if err == nil {
		t.Fatal("findZipFile() should ignore directories with .zip suffix")
	}
}
