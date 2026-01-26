// Package slack provides Slack API integration including credential management
// and Edge API client for channel detection.
package slack

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/denisbrodbeck/machineid"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// appID is the application identifier used by slackdump to derive encryption keys.
	// This value is from github.com/rusq/encio package.
	appID = "76d19bf515c59483e8923fcad9f1b65025d445e71801688b7edfb9cc2e64497f"

	// deriveIterations is the number of PBKDF2 iterations.
	deriveIterations = 4096

	// keySize is the AES-256 key size in bytes.
	keySize = 32
)

// salt is the fixed salt from github.com/rusq/secure package used for PBKDF2 key derivation.
//
//nolint:gochecknoglobals // required to match slackdump's encryption format
var salt = []byte{
	0x51, 0xfc, 0xd8, 0xf9, 0xab, 0x85, 0x93, 0x5d, 0xd2, 0x85, 0x2e, 0x78,
	0x3f, 0x80, 0x3a, 0xce, 0x19, 0xf1, 0x20, 0x75, 0x2a, 0xdd, 0x7b, 0x5c,
	0xe6, 0x17, 0xdb, 0x4b, 0x72, 0xc7, 0x83, 0x06, 0x10, 0x91, 0x70, 0x33,
	0x42, 0x0d, 0x75, 0xf9, 0xb8, 0x14, 0x39, 0x5a, 0xcf, 0xae, 0x6a, 0xec,
	0x7d, 0x3a, 0x2a, 0x87, 0xf8, 0x86, 0xa8, 0xea, 0x25, 0x7e, 0xb5, 0xf9,
	0x61, 0xe8, 0xa5, 0x5e, 0x20, 0x2f, 0xa2, 0x99, 0x85, 0xa3, 0xcc, 0xcd,
	0x5c, 0x39, 0x1b, 0x6d, 0x1b, 0x17, 0xa9, 0xb4, 0xeb, 0x95, 0xdd, 0xfb,
	0xbe, 0x3c, 0x2c, 0x3b, 0xe9, 0x7d, 0x5d, 0x3e, 0x78, 0x37, 0x23, 0xda,
	0xa5, 0x35, 0xd8, 0x36, 0xa7, 0x42, 0xd6, 0xdb, 0x38, 0xba, 0x17, 0x12,
	0x8c, 0x76, 0x83, 0x38, 0xd8, 0x23, 0x02, 0x38, 0x26, 0xe3, 0xe7, 0xe2,
	0x5e, 0xcb, 0xc9, 0x90, 0xd2, 0x46, 0x27, 0x84, 0x77, 0x41, 0x6b, 0xb5,
	0x7a, 0x4a, 0x4f, 0x45, 0xaa, 0xab, 0x50, 0xa7, 0x58, 0x35, 0xe8, 0xa9,
	0x27, 0xc1, 0xb8, 0xa9, 0x32, 0x03, 0x02, 0x3d, 0x19, 0x77, 0x5a, 0xd2,
	0x0c, 0x52, 0x08, 0x01, 0xfa, 0xb9, 0xb2, 0x86, 0xfd, 0x24, 0x73, 0xc3,
	0x39, 0xde, 0x4f, 0x86, 0x93, 0x19, 0xd7, 0xd5, 0x65, 0x00, 0xf1, 0x2d,
	0x0c, 0x6f, 0x3c, 0x21, 0xd0, 0xc6, 0x27, 0x99, 0x05, 0x19, 0x7c, 0x0d,
	0x57, 0x33, 0x4f, 0x8c, 0x2f, 0x72, 0x97, 0x5a, 0xfa, 0x08, 0x51, 0x51,
	0xbc, 0x56, 0xd4, 0xc4, 0xed, 0x01, 0xeb, 0xe2, 0x6a, 0x82, 0xc6, 0x4c,
	0x09, 0x76, 0xe3, 0xfa, 0x87, 0xe2, 0xd7, 0x68, 0x13, 0xa5, 0xcf, 0x32,
	0xa2, 0x16, 0x6c, 0x53, 0x50, 0x2d, 0xd2, 0x58, 0xe4, 0x67, 0x18, 0x7b,
	0x8a, 0x84, 0xe3, 0xa4, 0x49, 0x14, 0x64, 0xd5, 0x06, 0x68, 0xc7, 0x45,
	0x68, 0xeb, 0x4a, 0xb0,
}

// GetMachineID returns the machine's unique hardware identifier.
// This is used as the encryption key for slackdump's credential cache.
// On macOS, this returns the IOPlatformUUID.
func GetMachineID() (string, error) {
	return machineid.ID()
}

// LoadCredentials reads slackdump's cached credentials from the filesystem.
// Returns credentials needed for Slack Edge API calls.
func LoadCredentials() (*Credentials, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	workspace, err := getWorkspace(cacheDir)
	if err != nil {
		return nil, err
	}

	machineID, err := GetMachineID()
	if err != nil {
		return nil, fmt.Errorf("failed to get machine ID: %w", err)
	}

	key := deriveKey(machineID)

	credFile := filepath.Clean(filepath.Join(cacheDir, workspace+".bin"))
	ciphertext, err := os.ReadFile(credFile) //nolint:gosec // path validated by getCacheDir
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials file not found at %s - run 'slackdump auth' first", credFile)
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	plaintext, err := decrypt(ciphertext, key)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// TODO: Parse JSON and populate Credentials struct
	_ = plaintext // Will be used for JSON parsing
	return nil, fmt.Errorf("credential JSON parsing not yet implemented")
}

// getCacheDir returns the path to slackdump's cache directory.
// On macOS, this is ~/Library/Caches/slackdump/.
func getCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}

	cacheDir := filepath.Join(home, "Library", "Caches", "slackdump")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return "", fmt.Errorf("slackdump cache not found at %s - run 'slackdump auth' first", cacheDir)
	}

	return cacheDir, nil
}

// getWorkspace reads the current workspace name from slackdump's cache.
// The workspace name is stored in workspace.txt in the cache directory.
func getWorkspace(cacheDir string) (string, error) {
	workspaceFile := filepath.Clean(filepath.Join(cacheDir, "workspace.txt"))
	data, err := os.ReadFile(workspaceFile) //nolint:gosec // path is validated by getCacheDir
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workspace.txt not found in %s - run 'slackdump auth' first", cacheDir)
		}
		return "", fmt.Errorf("could not read workspace.txt: %w", err)
	}

	workspace := strings.TrimSpace(string(data))
	if workspace == "" {
		return "", fmt.Errorf("workspace.txt is empty - run 'slackdump auth' first")
	}

	return workspace, nil
}

// protectedID computes the HMAC-SHA256 of appID using machineID as the key.
// This matches machineid.ProtectedID() from github.com/denisbrodbeck/machineid.
func protectedID(machineID string) string {
	mac := hmac.New(sha256.New, []byte(machineID))
	mac.Write([]byte(appID))
	return hex.EncodeToString(mac.Sum(nil))
}

// deriveKey derives an AES-256 key from the machine ID using PBKDF2-SHA512.
// This matches the key derivation in github.com/rusq/secure.
func deriveKey(machineID string) []byte {
	protected := protectedID(machineID)
	return pbkdf2.Key([]byte(protected), salt, deriveIterations, keySize, sha512.New)
}

// decrypt decrypts AES-256-CFB encrypted data using the provided key.
// The first 16 bytes of ciphertext must be the initialization vector (IV).
func decrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short: need at least %d bytes for IV", aes.BlockSize)
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// CFB mode is deprecated but required for compatibility with slackdump's encryption format.
	stream := cipher.NewCFBDecrypter(block, iv) //nolint:staticcheck // required for slackdump compatibility
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}
