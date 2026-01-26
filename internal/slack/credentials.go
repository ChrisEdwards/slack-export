// Package slack provides Slack API integration including credential management
// and Edge API client for channel detection.
package slack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/denisbrodbeck/machineid"
)

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

	// TODO: Decrypt credentials from {workspace}.bin
	_ = workspace // Will be used for credential file path
	return nil, fmt.Errorf("credential decryption not yet implemented")
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
