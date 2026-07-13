package export

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed slackdump_sweep_api.toml
var sweepAPIConfig []byte

func writeSweepAPIConfig(archiveDir string) (string, error) {
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		return "", fmt.Errorf("creating archive metadata directory: %w", err)
	}
	path := filepath.Join(archiveDir, ".slack-export-sweep-api.toml")
	if err := os.WriteFile(path, sweepAPIConfig, 0600); err != nil {
		return "", fmt.Errorf("writing slackdump sweep API config: %w", err)
	}
	return path, nil
}
