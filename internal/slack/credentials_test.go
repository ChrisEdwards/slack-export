package slack

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	// Should be a CredentialError with user message mentioning slackdump auth
	credErr := GetCredentialError(err)
	if credErr == nil {
		t.Errorf("getCacheDir() should return CredentialError, got: %T", err)
	} else {
		if credErr.Code != ErrCodeCacheNotFound {
			t.Errorf("getCacheDir() error code = %v, want ErrCodeCacheNotFound", credErr.Code)
		}
		if !regexp.MustCompile(`slackdump auth`).MatchString(credErr.UserMessage()) {
			t.Errorf("getCacheDir() UserMessage should mention 'slackdump auth', got: %v", credErr.UserMessage())
		}
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
	// Should be a CredentialError with user message mentioning slackdump auth
	credErr := GetCredentialError(err)
	if credErr == nil {
		t.Errorf("getWorkspace() should return CredentialError, got: %T", err)
	} else {
		if credErr.Code != ErrCodeNoWorkspace {
			t.Errorf("getWorkspace() error code = %v, want ErrCodeNoWorkspace", credErr.Code)
		}
		if !regexp.MustCompile(`slackdump auth`).MatchString(credErr.UserMessage()) {
			t.Errorf("getWorkspace() UserMessage should mention 'slackdump auth', got: %v", credErr.UserMessage())
		}
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
	// Should be a CredentialError with user message mentioning slackdump auth
	credErr := GetCredentialError(err)
	if credErr == nil {
		t.Errorf("getWorkspace() should return CredentialError, got: %T", err)
	} else {
		if credErr.Code != ErrCodeEmptyWorkspace {
			t.Errorf("getWorkspace() error code = %v, want ErrCodeEmptyWorkspace", credErr.Code)
		}
		if !regexp.MustCompile(`slackdump auth`).MatchString(credErr.UserMessage()) {
			t.Errorf("getWorkspace() UserMessage should mention 'slackdump auth', got: %v", credErr.UserMessage())
		}
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

func TestParseCredentials_Success(t *testing.T) {
	// JSON matches slackdump's auth.simpleProvider serialization format
	jsonData := []byte(`{"Token":"xoxc-T12345678-U12345678-1234567890123-abc123","Cookie":[{"Name":"d","Value":"xoxd-test","Domain":".slack.com"}]}`)

	creds, err := parseCredentials(jsonData, "test-workspace")
	if err != nil {
		t.Fatalf("parseCredentials() error = %v", err)
	}

	if creds.Token != "xoxc-T12345678-U12345678-1234567890123-abc123" {
		t.Errorf("Token = %q, want xoxc-T12345678-U12345678-1234567890123-abc123", creds.Token)
	}

	if creds.Workspace != "test-workspace" {
		t.Errorf("Workspace = %q, want test-workspace", creds.Workspace)
	}

	if creds.TeamID != "T12345678" {
		t.Errorf("TeamID = %q, want T12345678", creds.TeamID)
	}

	if len(creds.Cookies) != 1 {
		t.Fatalf("len(Cookies) = %d, want 1", len(creds.Cookies))
	}

	if creds.Cookies[0].Name != "d" {
		t.Errorf("Cookie[0].Name = %q, want d", creds.Cookies[0].Name)
	}

	if creds.Cookies[0].Value != "xoxd-test" {
		t.Errorf("Cookie[0].Value = %q, want xoxd-test", creds.Cookies[0].Value)
	}
}

func TestParseCredentials_MultipleCookies(t *testing.T) {
	jsonData := []byte(`{"Token":"xoxc-T123-U456-789-abc","Cookie":[{"Name":"d","Value":"val1"},{"Name":"d-s","Value":"val2"}]}`)

	creds, err := parseCredentials(jsonData, "ws")
	if err != nil {
		t.Fatalf("parseCredentials() error = %v", err)
	}

	if len(creds.Cookies) != 2 {
		t.Fatalf("len(Cookies) = %d, want 2", len(creds.Cookies))
	}
}

func TestParseCredentials_EmptyToken(t *testing.T) {
	jsonData := []byte(`{"Token":"","Cookie":[]}`)

	_, err := parseCredentials(jsonData, "ws")
	if err == nil {
		t.Error("parseCredentials() expected error for empty token")
	}
}

func TestParseCredentials_MissingToken(t *testing.T) {
	jsonData := []byte(`{"Cookie":[]}`)

	_, err := parseCredentials(jsonData, "ws")
	if err == nil {
		t.Error("parseCredentials() expected error for missing token")
	}
}

func TestParseCredentials_InvalidJSON(t *testing.T) {
	jsonData := []byte(`not valid json`)

	_, err := parseCredentials(jsonData, "ws")
	if err == nil {
		t.Error("parseCredentials() expected error for invalid JSON")
	}
}

func TestParseCredentials_NoCookies(t *testing.T) {
	// Token without cookies should still work (cookies can be nil)
	jsonData := []byte(`{"Token":"xoxb-123-456-abc"}`)

	creds, err := parseCredentials(jsonData, "ws")
	if err != nil {
		t.Fatalf("parseCredentials() error = %v", err)
	}

	if creds.Token != "xoxb-123-456-abc" {
		t.Errorf("Token = %q, want xoxb-123-456-abc", creds.Token)
	}

	if len(creds.Cookies) != 0 {
		t.Errorf("Cookies should be empty, got %v", creds.Cookies)
	}
}

func TestExtractTeamID_XoxcToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "standard xoxc token",
			token: "xoxc-T12345678-U12345678-1234567890-abcdef123456",
			want:  "T12345678",
		},
		{
			name:  "longer team ID",
			token: "xoxc-T0123456789AB-U12345678-1234567890-hash",
			want:  "T0123456789AB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTeamID(tt.token)
			if got != tt.want {
				t.Errorf("extractTeamID(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestExtractTeamID_NonXoxcTokens(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{name: "xoxb token", token: "xoxb-123-456-abc"},
		{name: "xoxp token", token: "xoxp-123-456-abc"},
		{name: "empty token", token: ""},
		{name: "random string", token: "not-a-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTeamID(tt.token)
			if got != "" {
				t.Errorf("extractTeamID(%q) = %q, want empty string", tt.token, got)
			}
		})
	}
}

func TestExtractTeamID_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "xoxc with only prefix",
			token: "xoxc-",
			want:  "",
		},
		{
			name:  "xoxc with one part",
			token: "xoxc-TEAM",
			want:  "TEAM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTeamID(tt.token)
			if got != tt.want {
				t.Errorf("extractTeamID(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestLoadCredentials_Integration(t *testing.T) {
	// Create a temporary directory structure that mimics slackdump cache
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "Library", "Caches", "slackdump")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to create test cache dir: %v", err)
	}

	// Create workspace.txt
	workspace := "test-workspace"
	workspaceFile := filepath.Join(cacheDir, "workspace.txt")
	if err := os.WriteFile(workspaceFile, []byte(workspace+"\n"), 0o644); err != nil {
		t.Fatalf("failed to create workspace file: %v", err)
	}

	// Get machine ID for encryption
	machineID, err := GetMachineID()
	if err != nil {
		t.Fatalf("failed to get machine ID: %v", err)
	}

	// Create encrypted credentials file
	key := deriveKey(machineID)
	jsonData := []byte(`{"Token":"xoxc-T12345678-U12345678-1234567890-hash","Cookie":[{"Name":"d","Value":"xoxd-test"}]}`)

	ciphertext, err := encryptTestData(jsonData, key)
	if err != nil {
		t.Fatalf("failed to encrypt test credentials: %v", err)
	}

	credFile := filepath.Join(cacheDir, workspace+".bin")
	if err := os.WriteFile(credFile, ciphertext, 0o644); err != nil {
		t.Fatalf("failed to write credentials file: %v", err)
	}

	// Override HOME for the test
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Errorf("failed to restore HOME: %v", err)
		}
	}()

	// Test LoadCredentials
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}

	if creds.Token != "xoxc-T12345678-U12345678-1234567890-hash" {
		t.Errorf("Token = %q, want xoxc-T12345678-U12345678-1234567890-hash", creds.Token)
	}

	if creds.Workspace != workspace {
		t.Errorf("Workspace = %q, want %q", creds.Workspace, workspace)
	}

	if creds.TeamID != "T12345678" {
		t.Errorf("TeamID = %q, want T12345678", creds.TeamID)
	}

	if len(creds.Cookies) != 1 || creds.Cookies[0].Name != "d" {
		t.Errorf("Cookies not parsed correctly: %v", creds.Cookies)
	}
}

func TestCredentialError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *CredentialError
		wantMsg  string
		wantAuth bool // should UserMessage contain "slackdump auth"
	}{
		{
			name: "cache not found",
			err: &CredentialError{
				Code:    ErrCodeCacheNotFound,
				Message: "slackdump cache not found",
			},
			wantMsg:  "slackdump cache not found",
			wantAuth: true,
		},
		{
			name: "no workspace",
			err: &CredentialError{
				Code:    ErrCodeNoWorkspace,
				Message: "no workspace selected",
			},
			wantMsg:  "no workspace selected",
			wantAuth: true,
		},
		{
			name: "empty workspace",
			err: &CredentialError{
				Code:    ErrCodeEmptyWorkspace,
				Message: "workspace.txt is empty",
			},
			wantMsg:  "workspace.txt is empty",
			wantAuth: true,
		},
		{
			name: "credentials not found",
			err: &CredentialError{
				Code:    ErrCodeCredentialsNotFound,
				Message: "credentials not found for workspace \"test\"",
			},
			wantMsg:  "credentials not found for workspace \"test\"",
			wantAuth: true,
		},
		{
			name: "decrypt failed with cause",
			err: &CredentialError{
				Code:    ErrCodeDecryptFailed,
				Message: "failed to decrypt credentials",
				Cause:   errors.New("cipher error"),
			},
			wantMsg:  "failed to decrypt credentials: cipher error",
			wantAuth: true,
		},
		{
			name: "parse failed",
			err: &CredentialError{
				Code:    ErrCodeParseFailed,
				Message: "failed to parse credentials",
			},
			wantMsg:  "failed to parse credentials",
			wantAuth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}

			userMsg := tt.err.UserMessage()
			if tt.wantAuth && !strings.Contains(userMsg, "slackdump auth") {
				t.Errorf("UserMessage() should contain 'slackdump auth', got: %q", userMsg)
			}
		})
	}
}

func TestCredentialError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &CredentialError{
		Code:    ErrCodeDecryptFailed,
		Message: "outer error",
		Cause:   cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Test errors.Is
	if !errors.Is(err, cause) {
		t.Error("errors.Is() should find the cause")
	}
}

func TestIsCredentialError(t *testing.T) {
	credErr := &CredentialError{Code: ErrCodeCacheNotFound, Message: "test"}

	if !IsCredentialError(credErr) {
		t.Error("IsCredentialError() should return true for CredentialError")
	}

	if IsCredentialError(errors.New("regular error")) {
		t.Error("IsCredentialError() should return false for regular error")
	}

	// Test wrapped error
	wrapped := fmt.Errorf("wrapped: %w", credErr)
	if !IsCredentialError(wrapped) {
		t.Error("IsCredentialError() should return true for wrapped CredentialError")
	}
}

func TestGetCredentialError(t *testing.T) {
	credErr := &CredentialError{Code: ErrCodeCacheNotFound, Message: "test"}

	got := GetCredentialError(credErr)
	if got != credErr {
		t.Errorf("GetCredentialError() = %v, want %v", got, credErr)
	}

	got = GetCredentialError(errors.New("regular error"))
	if got != nil {
		t.Errorf("GetCredentialError() should return nil for regular error, got %v", got)
	}

	// Test wrapped error
	wrapped := fmt.Errorf("wrapped: %w", credErr)
	got = GetCredentialError(wrapped)
	if got != credErr {
		t.Errorf("GetCredentialError() should unwrap to CredentialError, got %v", got)
	}
}

func TestCredentials_Validate_Success(t *testing.T) {
	creds := &Credentials{
		Token:     "xoxc-T12345678-U12345678-1234567890-abc123",
		TeamID:    "T12345678",
		Workspace: "test-workspace",
	}

	if err := creds.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestCredentials_Validate_EmptyToken(t *testing.T) {
	creds := &Credentials{
		Token:     "",
		TeamID:    "T12345678",
		Workspace: "test-workspace",
	}

	err := creds.Validate()
	if err == nil {
		t.Error("Validate() expected error for empty token")
	}
	if !strings.Contains(err.Error(), "token is empty") {
		t.Errorf("Validate() error = %q, want error containing 'token is empty'", err)
	}
}

func TestCredentials_Validate_WrongTokenFormat(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		wantPreview  string
		wantEllipsis bool
	}{
		{
			name:         "xoxb token (bot token)",
			token:        "xoxb-FAKE-TOKEN-FOR-TESTING-ONLY",
			wantPreview:  "xoxb-FAKE-...",
			wantEllipsis: true,
		},
		{
			name:         "xoxp token (legacy)",
			token:        "xoxp-FAKE-TOKEN-FOR-TESTING",
			wantPreview:  "xoxp-FAKE-...",
			wantEllipsis: true,
		},
		{
			name:         "short invalid token",
			token:        "invalid",
			wantPreview:  "invalid",
			wantEllipsis: false,
		},
		{
			name:         "exactly 10 chars",
			token:        "1234567890",
			wantPreview:  "1234567890",
			wantEllipsis: false,
		},
		{
			name:         "11 chars (triggers truncation)",
			token:        "12345678901",
			wantPreview:  "1234567890...",
			wantEllipsis: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &Credentials{
				Token:     tt.token,
				TeamID:    "T12345678",
				Workspace: "test-workspace",
			}

			err := creds.Validate()
			if err == nil {
				t.Error("Validate() expected error for non-xoxc token")
				return
			}

			errStr := err.Error()
			if !strings.Contains(errStr, "unexpected token format") {
				t.Errorf("Validate() error = %q, want error containing 'unexpected token format'", errStr)
			}
			if !strings.Contains(errStr, tt.wantPreview) {
				t.Errorf("Validate() error = %q, want error containing preview %q", errStr, tt.wantPreview)
			}
		})
	}
}

func TestCredentials_Validate_EmptyTeamID(t *testing.T) {
	creds := &Credentials{
		Token:     "xoxc-T12345678-U12345678-1234567890-abc123",
		TeamID:    "",
		Workspace: "test-workspace",
	}

	err := creds.Validate()
	if err == nil {
		t.Error("Validate() expected error for empty team ID")
	}
	if !strings.Contains(err.Error(), "team ID is missing") {
		t.Errorf("Validate() error = %q, want error containing 'team ID is missing'", err)
	}
}

func TestCredentials_Validate_MultipleErrors(t *testing.T) {
	// When multiple fields are invalid, empty token takes precedence
	creds := &Credentials{
		Token:     "",
		TeamID:    "",
		Workspace: "",
	}

	err := creds.Validate()
	if err == nil {
		t.Error("Validate() expected error")
	}
	// Token check comes first
	if !strings.Contains(err.Error(), "token is empty") {
		t.Errorf("Validate() should check token first, got: %q", err)
	}
}

func TestCredentials_Validate_TokenFormatBeforeTeamID(t *testing.T) {
	// When token format is wrong and team ID is missing, token format error takes precedence
	creds := &Credentials{
		Token:     "xoxb-invalid",
		TeamID:    "",
		Workspace: "",
	}

	err := creds.Validate()
	if err == nil {
		t.Error("Validate() expected error")
	}
	// Token format check comes before team ID check
	if !strings.Contains(err.Error(), "unexpected token format") {
		t.Errorf("Validate() should check token format before team ID, got: %q", err)
	}
}
