// Package export orchestrates the Slack export workflow.
package export

import (
	"context"
	"fmt"

	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/chrisedwards/slack-export/internal/slack"
)

// Exporter orchestrates the export workflow for Slack channels.
// It holds all dependencies needed for the export process.
type Exporter struct {
	cfg        *config.Config
	edgeClient *slack.EdgeClient
	slackdump  string             // path to slackdump binary
	creds      *slack.Credentials // credentials for TeamID access
}

// NewExporter creates an Exporter with the given configuration.
// It loads credentials, finds slackdump, creates the Edge client,
// and verifies connectivity by fetching the TeamID.
func NewExporter(cfg *config.Config) (*Exporter, error) {
	creds, err := slack.LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("loading credentials: %w", err)
	}

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	sdPath, err := FindSlackdump(cfg.SlackdumpPath)
	if err != nil {
		return nil, err
	}

	edgeClient := slack.NewEdgeClient(creds)

	boot, err := edgeClient.ClientUserBoot(context.Background())
	if err != nil {
		return nil, fmt.Errorf("verifying credentials: %w", err)
	}
	creds.TeamID = boot.Self.TeamID

	return &Exporter{
		cfg:        cfg,
		edgeClient: edgeClient,
		slackdump:  sdPath,
		creds:      creds,
	}, nil
}

// Config returns the exporter's configuration.
func (e *Exporter) Config() *config.Config {
	return e.cfg
}

// EdgeClient returns the Edge API client.
func (e *Exporter) EdgeClient() *slack.EdgeClient {
	return e.edgeClient
}

// SlackdumpPath returns the path to the slackdump binary.
func (e *Exporter) SlackdumpPath() string {
	return e.slackdump
}

// Credentials returns the Slack credentials.
func (e *Exporter) Credentials() *slack.Credentials {
	return e.creds
}
