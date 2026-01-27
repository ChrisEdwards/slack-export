package export

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindSlackdump_FromPATH(t *testing.T) {
	// Create a temp dir with a fake slackdump binary
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "slackdump")
	if err := os.WriteFile(fakeBin, []byte("fake"), 0755); err != nil {
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

// createTestZip creates a zip file with the given entries for testing.
// entries maps filename to content.
func createTestZip(t *testing.T, zipPath string, entries map[string]string) {
	t.Helper()

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("creating zip file: %v", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("creating zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("writing zip entry %s: %v", name, err)
		}
	}
}

func TestExtractAndProcess_Success(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	outputDir := filepath.Join(tmpDir, "output")

	// Create a test zip with channel files
	createTestZip(t, zipPath, map[string]string{
		"C123456.txt": "messages from engineering",
		"D789012.txt": "messages from dm",
	})

	channelNames := map[string]string{
		"C123456": "engineering",
		"D789012": "dm_bob_smith",
	}

	err := ExtractAndProcess(zipPath, outputDir, "2026-01-22", channelNames)
	if err != nil {
		t.Fatalf("ExtractAndProcess() error = %v", err)
	}

	// Verify output structure
	expected := map[string]string{
		"2026-01-22/2026-01-22-engineering.md":  "messages from engineering",
		"2026-01-22/2026-01-22-dm_bob_smith.md": "messages from dm",
	}

	for relPath, wantContent := range expected {
		fullPath := filepath.Join(outputDir, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("reading %s: %v", relPath, err)
			continue
		}
		if string(content) != wantContent {
			t.Errorf("content of %s = %q, want %q", relPath, content, wantContent)
		}
	}
}

func TestExtractAndProcess_FallbackToChannelID(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	outputDir := filepath.Join(tmpDir, "output")

	createTestZip(t, zipPath, map[string]string{
		"C123456.txt":  "known channel",
		"CUNKNOWN.txt": "unknown channel",
	})

	// Only provide name for one channel
	channelNames := map[string]string{
		"C123456": "engineering",
	}

	err := ExtractAndProcess(zipPath, outputDir, "2026-01-22", channelNames)
	if err != nil {
		t.Fatalf("ExtractAndProcess() error = %v", err)
	}

	// Check that unknown channel falls back to ID
	unknownPath := filepath.Join(outputDir, "2026-01-22", "2026-01-22-CUNKNOWN.md")
	if _, err := os.Stat(unknownPath); err != nil {
		t.Errorf("expected file at %s for unknown channel ID fallback", unknownPath)
	}

	knownPath := filepath.Join(outputDir, "2026-01-22", "2026-01-22-engineering.md")
	if _, err := os.Stat(knownPath); err != nil {
		t.Errorf("expected file at %s for known channel", knownPath)
	}
}

func TestExtractAndProcess_NilChannelNames(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	outputDir := filepath.Join(tmpDir, "output")

	createTestZip(t, zipPath, map[string]string{
		"C123456.txt": "content",
	})

	// nil channelNames should work (falls back to ID)
	err := ExtractAndProcess(zipPath, outputDir, "2026-01-22", nil)
	if err != nil {
		t.Fatalf("ExtractAndProcess() error = %v", err)
	}

	expectedPath := filepath.Join(outputDir, "2026-01-22", "2026-01-22-C123456.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected file at %s", expectedPath)
	}
}

func TestExtractAndProcess_InvalidZipPath(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	err := ExtractAndProcess("/nonexistent/path.zip", outputDir, "2026-01-22", nil)
	if err == nil {
		t.Fatal("ExtractAndProcess() with nonexistent zip should return error")
	}
	if !strings.Contains(err.Error(), "opening zip file") {
		t.Errorf("error %q should mention 'opening zip file'", err.Error())
	}
}

func TestExtractAndProcess_InvalidZipFile(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "invalid.zip")
	outputDir := filepath.Join(tmpDir, "output")

	// Create an invalid zip file
	if err := os.WriteFile(zipPath, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	err := ExtractAndProcess(zipPath, outputDir, "2026-01-22", nil)
	if err == nil {
		t.Fatal("ExtractAndProcess() with invalid zip should return error")
	}
	if !strings.Contains(err.Error(), "opening zip file") {
		t.Errorf("error %q should mention 'opening zip file'", err.Error())
	}
}

func TestExtractAndProcess_EmptyZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "empty.zip")
	outputDir := filepath.Join(tmpDir, "output")

	createTestZip(t, zipPath, map[string]string{})

	err := ExtractAndProcess(zipPath, outputDir, "2026-01-22", nil)
	if err != nil {
		t.Fatalf("ExtractAndProcess() with empty zip should succeed, got error = %v", err)
	}

	// Date directory should still be created
	dateDir := filepath.Join(outputDir, "2026-01-22")
	info, err := os.Stat(dateDir)
	if err != nil {
		t.Errorf("date directory should exist: %v", err)
	} else if !info.IsDir() {
		t.Error("date directory should be a directory")
	}
}

func TestExtractAndProcess_SkipsDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	outputDir := filepath.Join(tmpDir, "output")

	// Create a zip with a directory entry manually
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	w := zip.NewWriter(zipFile)
	// Add a directory entry
	_, err = w.Create("subdir/")
	if err != nil {
		t.Fatal(err)
	}
	// Add a file
	f, err := w.Create("C123.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("content"))
	w.Close()
	zipFile.Close()

	err = ExtractAndProcess(zipPath, outputDir, "2026-01-22", nil)
	if err != nil {
		t.Fatalf("ExtractAndProcess() error = %v", err)
	}

	// The file should exist
	filePath := filepath.Join(outputDir, "2026-01-22", "2026-01-22-C123.md")
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("expected file at %s", filePath)
	}
}

func TestExtractAndProcess_EmptyChannelName(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	outputDir := filepath.Join(tmpDir, "output")

	createTestZip(t, zipPath, map[string]string{
		"C123456.txt": "content",
	})

	// Empty string value should fall back to ID
	channelNames := map[string]string{
		"C123456": "",
	}

	err := ExtractAndProcess(zipPath, outputDir, "2026-01-22", channelNames)
	if err != nil {
		t.Fatalf("ExtractAndProcess() error = %v", err)
	}

	// Should use channel ID since name is empty
	expectedPath := filepath.Join(outputDir, "2026-01-22", "2026-01-22-C123456.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected file at %s (fallback to ID when name is empty)", expectedPath)
	}
}
