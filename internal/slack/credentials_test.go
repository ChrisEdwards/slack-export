package slack

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestGetMachineID(t *testing.T) {
	id, err := GetMachineID()
	if err != nil {
		t.Fatalf("GetMachineID() error = %v", err)
	}

	if id == "" {
		t.Error("GetMachineID() returned empty string")
	}

	// UUID format: XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX (with or without hyphens)
	// machineid may return lowercase hex without hyphens on some platforms
	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)
	if !uuidPattern.MatchString(id) {
		t.Errorf("GetMachineID() = %q, does not match UUID pattern", id)
	}
}

func TestGetMachineID_Consistent(t *testing.T) {
	id1, err := GetMachineID()
	if err != nil {
		t.Fatalf("GetMachineID() first call error = %v", err)
	}

	id2, err := GetMachineID()
	if err != nil {
		t.Fatalf("GetMachineID() second call error = %v", err)
	}

	if id1 != id2 {
		t.Errorf("GetMachineID() not consistent: %q != %q", id1, id2)
	}
}

func TestGetCacheDir_Success(t *testing.T) {
	// Create a temporary directory structure that mimics slackdump cache
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "Library", "Caches", "slackdump")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to create test cache dir: %v", err)
	}

	// Save original HOME and restore after test
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("failed to restore HOME: %v", err)
		}
	}()

	got, err := getCacheDir()
	if err != nil {
		t.Errorf("getCacheDir() error = %v", err)
	}
	if got != cacheDir {
		t.Errorf("getCacheDir() = %q, want %q", got, cacheDir)
	}
}

func TestGetCacheDir_NotFound(t *testing.T) {
	// Create temp dir without slackdump cache
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("failed to restore HOME: %v", err)
		}
	}()

	_, err := getCacheDir()
	if err == nil {
		t.Error("getCacheDir() expected error for missing cache dir")
	}
	// Should mention running slackdump auth
	if err != nil && !regexp.MustCompile(`slackdump auth`).MatchString(err.Error()) {
		t.Errorf("getCacheDir() error should mention 'slackdump auth', got: %v", err)
	}
}

func TestGetWorkspace_Success(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceFile := filepath.Join(tmpDir, "workspace.txt")

	if err := os.WriteFile(workspaceFile, []byte("my-workspace\n"), 0o644); err != nil {
		t.Fatalf("failed to create test workspace file: %v", err)
	}

	got, err := getWorkspace(tmpDir)
	if err != nil {
		t.Errorf("getWorkspace() error = %v", err)
	}
	if got != "my-workspace" {
		t.Errorf("getWorkspace() = %q, want %q", got, "my-workspace")
	}
}

func TestGetWorkspace_TrimsWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceFile := filepath.Join(tmpDir, "workspace.txt")

	// Content with leading/trailing whitespace
	if err := os.WriteFile(workspaceFile, []byte("  test-workspace  \n\n"), 0o644); err != nil {
		t.Fatalf("failed to create test workspace file: %v", err)
	}

	got, err := getWorkspace(tmpDir)
	if err != nil {
		t.Errorf("getWorkspace() error = %v", err)
	}
	if got != "test-workspace" {
		t.Errorf("getWorkspace() = %q, want %q", got, "test-workspace")
	}
}

func TestGetWorkspace_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := getWorkspace(tmpDir)
	if err == nil {
		t.Error("getWorkspace() expected error for missing workspace.txt")
	}
	// Should mention running slackdump auth
	if err != nil && !regexp.MustCompile(`slackdump auth`).MatchString(err.Error()) {
		t.Errorf("getWorkspace() error should mention 'slackdump auth', got: %v", err)
	}
}

func TestGetWorkspace_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceFile := filepath.Join(tmpDir, "workspace.txt")

	// Empty file
	if err := os.WriteFile(workspaceFile, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to create test workspace file: %v", err)
	}

	_, err := getWorkspace(tmpDir)
	if err == nil {
		t.Error("getWorkspace() expected error for empty workspace.txt")
	}
	// Should mention running slackdump auth
	if err != nil && !regexp.MustCompile(`slackdump auth`).MatchString(err.Error()) {
		t.Errorf("getWorkspace() error should mention 'slackdump auth', got: %v", err)
	}
}

func TestGetWorkspace_OnlyWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceFile := filepath.Join(tmpDir, "workspace.txt")

	// Only whitespace
	if err := os.WriteFile(workspaceFile, []byte("   \n\n  "), 0o644); err != nil {
		t.Fatalf("failed to create test workspace file: %v", err)
	}

	_, err := getWorkspace(tmpDir)
	if err == nil {
		t.Error("getWorkspace() expected error for whitespace-only workspace.txt")
	}
}
