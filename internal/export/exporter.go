// Package export orchestrates the Slack export workflow.
package export

import (
	"context"
	"time"

	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/chrisedwards/slack-export/internal/slack"
)

// Exporter orchestrates the export workflow for Slack channels.
type Exporter struct {
	cfg *config.Config
}

// NewExporter creates an Exporter with the given configuration.
func NewExporter(cfg *config.Config) (*Exporter, error) {
	return &Exporter{cfg: cfg}, nil
}

// Config returns the exporter's configuration.
func (e *Exporter) Config() *config.Config {
	return e.cfg
}

// Export exports messages for the given channels on the specified date.
func (e *Exporter) Export(ctx context.Context, channels []slack.Channel, date time.Time) error {
	// TODO: Implement export orchestration
	return nil
}
