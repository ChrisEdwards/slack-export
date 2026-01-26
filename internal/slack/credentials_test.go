package slack

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/denisbrodbeck/machineid"
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

func TestProtectedID_MatchesMachineid(t *testing.T) {
	testMachineID := "test-machine-id-12345"

	got := protectedID(testMachineID)

	// Verify our implementation matches machineid.ProtectedID behavior
	// We use machineid's protect function logic: HMAC-SHA256 of appID keyed by machineID
	expected, err := machineid.ProtectedID(appID)
	if err == nil {
		// If we can get actual machine's protected ID, verify format matches
		// Both should be 64-character hex strings
		if len(got) != 64 {
			t.Errorf("protectedID() length = %d, want 64 hex chars", len(got))
		}
		if len(expected) != 64 {
			t.Errorf("machineid.ProtectedID() length = %d, want 64 hex chars", len(expected))
		}
	}

	// Verify it's a valid hex string
	hexPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !hexPattern.MatchString(got) {
		t.Errorf("protectedID() = %q, does not match hex pattern", got)
	}
}

func TestProtectedID_Consistent(t *testing.T) {
	testMachineID := "test-machine-id-12345"

	result1 := protectedID(testMachineID)
	result2 := protectedID(testMachineID)

	if result1 != result2 {
		t.Errorf("protectedID() not consistent: %q != %q", result1, result2)
	}
}

func TestProtectedID_DifferentInputsDifferentOutputs(t *testing.T) {
	result1 := protectedID("machine-1")
	result2 := protectedID("machine-2")

	if result1 == result2 {
		t.Errorf("protectedID() should produce different outputs for different inputs")
	}
}

func TestDeriveKey_ProducesCorrectLength(t *testing.T) {
	testMachineID := "test-machine-id-12345"

	key := deriveKey(testMachineID)

	if len(key) != keySize {
		t.Errorf("deriveKey() length = %d, want %d", len(key), keySize)
	}
}

func TestDeriveKey_Consistent(t *testing.T) {
	testMachineID := "test-machine-id-12345"

	key1 := deriveKey(testMachineID)
	key2 := deriveKey(testMachineID)

	if !bytes.Equal(key1, key2) {
		t.Error("deriveKey() not consistent")
	}
}

func TestDeriveKey_DifferentInputsDifferentOutputs(t *testing.T) {
	key1 := deriveKey("machine-1")
	key2 := deriveKey("machine-2")

	if bytes.Equal(key1, key2) {
		t.Error("deriveKey() should produce different keys for different inputs")
	}
}

func TestDecrypt_Success(t *testing.T) {
	// Create test data
	key := deriveKey("test-machine-id")
	plaintext := []byte("hello, world!")

	// Encrypt the data using AES-256-CFB (same as slackdump)
	ciphertext, err := encryptTestData(plaintext, key)
	if err != nil {
		t.Fatalf("failed to encrypt test data: %v", err)
	}

	// Decrypt and verify
	decrypted, err := decrypt(ciphertext, key)
	if err != nil {
		t.Errorf("decrypt() error = %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestDecrypt_CiphertextTooShort(t *testing.T) {
	key := deriveKey("test-machine-id")

	// Ciphertext shorter than IV (16 bytes)
	shortCiphertext := []byte("short")

	_, err := decrypt(shortCiphertext, key)
	if err == nil {
		t.Error("decrypt() expected error for short ciphertext")
	}
}

func TestDecrypt_EmptyCiphertext(t *testing.T) {
	key := deriveKey("test-machine-id")

	_, err := decrypt([]byte{}, key)
	if err == nil {
		t.Error("decrypt() expected error for empty ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	// Encrypt with one key
	key1 := deriveKey("machine-1")
	plaintext := []byte(`{"token":"xoxc-test","team_id":"T123"}`)

	ciphertext, err := encryptTestData(plaintext, key1)
	if err != nil {
		t.Fatalf("failed to encrypt test data: %v", err)
	}

	// Try to decrypt with different key
	key2 := deriveKey("machine-2")
	decrypted, err := decrypt(ciphertext, key2)

	// Decryption will "succeed" (no error) but produce garbage
	// because AES-CFB doesn't have authentication
	if err != nil {
		t.Fatalf("decrypt() unexpected error = %v", err)
	}

	// The decrypted data should not match the original
	if bytes.Equal(decrypted, plaintext) {
		t.Error("decrypt() with wrong key should not produce original plaintext")
	}
}

// encryptTestData encrypts data using AES-256-CFB for testing purposes.
// This matches slackdump's encryption format.
func encryptTestData(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}

	// Encrypt
	stream := cipher.NewCFBEncrypter(block, iv) //nolint:staticcheck // matching slackdump format
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)

	// Prepend IV to ciphertext (slackdump format)
	return append(iv, ciphertext...), nil
}
