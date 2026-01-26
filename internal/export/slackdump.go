package export

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/chrisedwards/slack-export/internal/slack"
)

// FindSlackdump locates the slackdump binary.
// If configPath is non-empty, it validates that path exists.
// Otherwise, it searches PATH for "slackdump".
func FindSlackdump(configPath string) (string, error) {
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		return "", fmt.Errorf("slackdump not found at %s", configPath)
	}
	path, err := exec.LookPath("slackdump")
	if err != nil {
		return "", fmt.Errorf("slackdump not found in PATH - install from https://github.com/rusq/slackdump")
	}
	return path, nil
}

// SlackdumpRunner wraps the slackdump CLI for message export.
type SlackdumpRunner struct {
	binPath string
}

// NewSlackdumpRunner creates a runner with the optional binary path.
// If binPath is empty, it will search PATH for slackdump.
func NewSlackdumpRunner(binPath string) *SlackdumpRunner {
	return &SlackdumpRunner{binPath: binPath}
}

// ExportChannel exports messages for a single channel on the given date.
func (r *SlackdumpRunner) ExportChannel(ctx context.Context, ch slack.Channel, date time.Time, outputDir string) error {
	// TODO: Implement slackdump CLI invocation
	return nil
}
