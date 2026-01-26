// Package export orchestrates the Slack export workflow.
package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chrisedwards/slack-export/internal/channels"
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

// ExportDate exports Slack messages for a single date.
// It orchestrates the full workflow: gets channels, filters them,
// archives via slackdump, formats to text, and organizes output.
func (e *Exporter) ExportDate(ctx context.Context, date string) error {
	start, end, err := GetDateBounds(date, e.cfg.Timezone)
	if err != nil {
		return fmt.Errorf("calculating date bounds: %w", err)
	}

	allChannels, err := e.edgeClient.GetActiveChannels(ctx, start)
	if err != nil {
		return fmt.Errorf("getting active channels: %w", err)
	}

	if len(allChannels) == 0 {
		fmt.Printf("No active channels found for %s\n", date)
		return nil
	}

	filtered := channels.FilterChannels(allChannels, e.cfg.Include, e.cfg.Exclude)
	if len(filtered) == 0 {
		fmt.Printf("All channels filtered out for %s\n", date)
		return nil
	}

	fmt.Printf("Exporting %d channels for %s\n", len(filtered), date)

	ids, names := buildChannelMaps(filtered)

	archiveDir, err := Archive(ctx, e.slackdump, ids, start, end)
	if err != nil {
		return fmt.Errorf("archiving channels: %w", err)
	}
	defer cleanupTempDir(archiveDir)

	zipPath, err := FormatText(ctx, e.slackdump, archiveDir)
	if err != nil {
		return fmt.Errorf("formatting text: %w", err)
	}

	if err := ExtractAndProcess(zipPath, e.cfg.OutputDir, date, names); err != nil {
		return fmt.Errorf("extracting output: %w", err)
	}

	fmt.Printf("Successfully exported %d channels to %s/%s/\n", len(filtered), e.cfg.OutputDir, date)
	return nil
}

// buildChannelMaps builds a list of channel IDs and a map of ID to name.
func buildChannelMaps(chans []slack.Channel) ([]string, map[string]string) {
	ids := make([]string, 0, len(chans))
	names := make(map[string]string, len(chans))
	for _, ch := range chans {
		ids = append(ids, ch.ID)
		names[ch.ID] = ch.Name
	}
	return ids, names
}

// cleanupTempDir removes the temporary directory created by Archive.
func cleanupTempDir(archiveDir string) {
	if archiveDir != "" {
		_ = os.RemoveAll(filepath.Dir(archiveDir))
	}
}
