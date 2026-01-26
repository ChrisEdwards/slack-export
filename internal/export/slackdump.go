package export

import (
	"context"
	"time"

	"github.com/chrisedwards/slack-export/internal/slack"
)

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
