package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
