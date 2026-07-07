// Package export orchestrates the Slack export workflow.
package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chrisedwards/slack-export/internal/channels"
	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/chrisedwards/slack-export/internal/slack"
	"github.com/rusq/slackdump/v4/source"
)

// Exporter orchestrates the export workflow for Slack channels.
type Exporter struct {
	cfg        *config.Config
	edgeClient *slack.EdgeClient
	slackdump  string
	creds      *slack.Credentials
}

// NewExporter creates an Exporter with Slack credentials, Edge API, and slackdump.
func NewExporter(cfg *config.Config) (*Exporter, error) {
	creds, err := slack.LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("loading credentials: %w", err)
	}
	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	sdPath, err := FindSlackdump()
	if err != nil {
		return nil, err
	}

	edgeClient := slack.NewEdgeClient(creds)
	if _, err := edgeClient.AuthTest(context.Background()); err != nil {
		return nil, fmt.Errorf("verifying credentials: %w", err)
	}

	return &Exporter{cfg: cfg, edgeClient: edgeClient, slackdump: sdPath, creds: creds}, nil
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

// ArchiveDir returns the workspace-specific persistent archive directory.
func (e *Exporter) ArchiveDir() (string, error) {
	workspace := e.creds.Workspace
	if workspace == "" {
		workspace = e.creds.TeamID
	}
	return WorkspaceArchiveDir(e.cfg, workspace)
}

// WorkspaceArchiveDir returns the archive directory for a workspace name.
func WorkspaceArchiveDir(cfg *config.Config, workspace string) (string, error) {
	base, err := expandPath(cfg.ArchiveDir)
	if err != nil {
		return "", err
	}
	if workspace == "" {
		return "", errors.New("workspace name is required for archive path")
	}
	return filepath.Join(base, sanitizePathPart(workspace)), nil
}

// ConfigSeedDate returns the configured or inferred seed date.
func ConfigSeedDate(cfg *config.Config, now time.Time) (string, error) {
	return (&Exporter{cfg: cfg}).seedDate(now)
}

// ConfigRenderWindow returns the normal render window for a config.
func ConfigRenderWindow(cfg *config.Config, now time.Time) (string, string, error) {
	return (&Exporter{cfg: cfg}).renderWindow(now)
}

// ExportDate renders Slack messages for a single date from the archive database.
func (e *Exporter) ExportDate(ctx context.Context, date string) error {
	return e.ExportRange(ctx, date, date)
}

// ExportRange renders Slack messages for all dates in a range from the archive database.
func (e *Exporter) ExportRange(ctx context.Context, from, to string) error {
	if _, err := datesInRange(from, to, e.cfg.Timezone); err != nil {
		return err
	}

	seedDate, err := e.seedDate(time.Now())
	if err != nil {
		return err
	}
	if from < seedDate {
		return fmt.Errorf(
			"date %s predates archive seed_date %s; reseed with an earlier seed_date, "+
				"run slackdump archive -time-from, then render --full",
			from,
			seedDate,
		)
	}

	archiveDir, err := e.ArchiveDir()
	if err != nil {
		return err
	}
	if !archiveExists(archiveDir) {
		return fmt.Errorf("archive does not exist at %s; run slack-export sync first", archiveDir)
	}

	writes, err := RenderArchiveRange(ctx, archiveDir, e.cfg.OutputDir, from, to, e.cfg.Timezone)
	if err != nil {
		return err
	}
	fmt.Printf("Rendered %s through %s (%d changed file(s))\n", from, to, writes)
	return nil
}

// Sync refreshes the persistent archive and renders the recent change window.
func (e *Exporter) Sync(ctx context.Context, now time.Time) error {
	tracked, err := e.trackedChannels(ctx)
	if err != nil {
		return err
	}
	if len(tracked) == 0 {
		fmt.Println("No tracked channels found")
		return nil
	}

	archiveDir, err := e.ArchiveDir()
	if err != nil {
		return err
	}
	ids := channelIDs(tracked)

	if !archiveExists(archiveDir) {
		seedDate, err := e.seedDate(now)
		if err != nil {
			return err
		}
		seedStart, _, err := GetDateBounds(seedDate, e.cfg.Timezone)
		if err != nil {
			return fmt.Errorf("calculating seed date bounds: %w", err)
		}
		fmt.Printf("Bootstrapping archive from %s into %s\n", seedDate, archiveDir)
		if err := BootstrapArchive(ctx, e.slackdump, archiveDir, ids, seedStart); err != nil {
			return fmt.Errorf("bootstrapping archive: %w", err)
		}
		if err := markFullSweep(archiveDir, now); err != nil {
			return err
		}
	} else if err := e.resumeArchive(ctx, archiveDir, tracked); err != nil {
		return err
	}
	if err := saveChannelNames(archiveDir, tracked); err != nil {
		return fmt.Errorf("saving channel names: %w", err)
	}

	from, to, err := e.renderWindow(now)
	if err != nil {
		return err
	}
	writes, err := RenderArchiveRangeForChannels(ctx, archiveDir, e.cfg.OutputDir, from, to, e.cfg.Timezone, ids)
	if err != nil {
		return err
	}
	fmt.Printf("Rendered %s through %s (%d changed file(s))\n", from, to, writes)
	return nil
}

func (e *Exporter) resumeArchive(ctx context.Context, archiveDir string, tracked []slack.Channel) error {
	resumeArgs, hasWork := e.scopedResumeArgs(ctx, archiveDir, tracked)
	if !hasWork {
		fmt.Println("Archive already current")
		return nil
	}

	now := time.Now()
	opts := e.resumeOptions(archiveDir, now)
	if len(resumeArgs) == 0 {
		fmt.Println("Resuming archive with existing checkpoints")
	} else {
		fmt.Printf("Resuming archive with %d scoped entity arg(s)\n", len(resumeArgs))
	}
	if err := ResumeArchive(ctx, e.slackdump, archiveDir, resumeArgs, opts); err != nil {
		return fmt.Errorf("resuming archive: %w", err)
	}
	if opts.SkipStaleThreads == "" && opts.SkipStaleChannels == "" {
		if err := markFullSweep(archiveDir, now); err != nil {
			return err
		}
	}
	return nil
}

func (e *Exporter) trackedChannels(ctx context.Context) ([]slack.Channel, error) {
	userIndex, err := e.edgeClient.FetchUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching users: %w", err)
	}
	cache := slack.NewUserCache(slack.DefaultCachePath())
	if err := cache.Load(); err != nil {
		return nil, fmt.Errorf("loading user cache: %w", err)
	}
	resolver := slack.NewUserResolver(userIndex, cache, e.edgeClient)
	allChannels, err := e.edgeClient.GetActiveChannelsWithResolver(ctx, time.Time{}, resolver)
	if err != nil {
		return nil, fmt.Errorf("getting active channels: %w", err)
	}
	if err := cache.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save user cache: %v\n", err)
	}
	return channels.FilterChannels(allChannels, e.cfg.Include, e.cfg.Exclude), nil
}

func (e *Exporter) resumeOptions(archiveDir string, now time.Time) ResumeOptions {
	fullSweep := fullSweepDue(archiveDir, e.cfg.FullSweepInterval, now)
	opts := ResumeOptions{
		Lookback:            e.cfg.Lookback,
		SkipStaleThreads:    e.cfg.SkipStaleThreads,
		SkipStaleChannels:   e.cfg.SkipStaleChannels,
		SkipCompleteThreads: e.cfg.SkipCompleteThreads,
		Dedupe:              fullSweep,
	}
	if fullSweep {
		opts.SkipStaleThreads = ""
		opts.SkipStaleChannels = ""
	}
	return opts
}

func (e *Exporter) scopedResumeArgs(ctx context.Context, archiveDir string, tracked []slack.Channel) ([]string, bool) {
	counts, err := e.edgeClient.ClientCounts(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: counts scoping failed; skipping archive resume to avoid an unscoped Slackdump run: %v\n", err)
		return nil, false
	}
	src, err := source.Load(ctx, archiveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: archive checkpoint load failed; skipping archive resume to avoid an unscoped Slackdump run: %v\n", err)
		return nil, false
	}
	defer func() { _ = src.Close() }()

	latest, err := src.Latest(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: archive checkpoint read failed; skipping archive resume to avoid an unscoped Slackdump run: %v\n", err)
		return nil, false
	}

	checkpoints := make(map[string]time.Time, len(latest))
	for link, ts := range latest {
		key := fmt.Sprint(link)
		if !strings.Contains(key, ":") {
			checkpoints[key] = ts
		}
	}

	countLatest := countsLatestByID(counts)
	coverageStart, err := archiveCoverageStart(archiveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: archive coverage read failed; skipping archive resume to avoid an unbounded Slackdump run: %v\n", err)
		return nil, false
	}
	if coverageStart.IsZero() {
		fmt.Fprintln(os.Stderr, "Warning: archive coverage start unknown; skipping archive resume to avoid an unbounded Slackdump run")
		return nil, false
	}
	movedIDs := movedResumeChannelIDs(tracked, checkpoints, countLatest, coverageStart)
	if len(movedIDs) == 0 {
		return nil, false
	}
	return scopedResumeArgsFromLatest(tracked, latest, checkpoints, movedIDs, coverageStart), true
}

func scopedResumeArgsFromLatest[K interface {
	fmt.Stringer
	comparable
}](
	tracked []slack.Channel,
	latest map[K]time.Time,
	checkpoints map[string]time.Time,
	movedIDs []string,
	coverageStart time.Time,
) []string {
	if len(movedIDs) == 0 {
		return nil
	}

	trackedSet := channelIDSet(channelIDs(tracked))
	movedSet := channelIDSet(movedIDs)
	args := make([]string, 0, len(latest))
	for link := range latest {
		key := fmt.Sprint(link)
		channelID := strings.SplitN(key, ":", 2)[0]
		if !trackedSet[channelID] || !movedSet[channelID] {
			args = append(args, "^"+key)
		}
	}

	for _, id := range movedIDs {
		if _, ok := checkpoints[id]; !ok {
			// A checkpoint-less channel would otherwise be fetched from the
			// beginning of its history (plus every thread ever posted in it).
			// Bound it at the archive coverage start; nothing older is rendered.
			args = append(args, id+","+coverageStart.UTC().Format(slackdumpTimeFormat))
		}
	}

	return args
}

func movedResumeChannelIDs(
	tracked []slack.Channel,
	checkpoints map[string]time.Time,
	countLatest map[string]time.Time,
	coverageStart time.Time,
) []string {
	var moved []string
	for _, ch := range tracked {
		checkpoint, ok := checkpoints[ch.ID]
		if !ok {
			if latest, ok := countLatest[ch.ID]; ok && !coverageStart.IsZero() && latest.Before(coverageStart) {
				continue
			}
			moved = append(moved, ch.ID)
			continue
		}
		if latest, ok := countLatest[ch.ID]; ok && latest.After(checkpoint) {
			moved = append(moved, ch.ID)
		}
	}
	return moved
}

func countsLatestByID(counts *slack.CountsResponse) map[string]time.Time {
	result := make(map[string]time.Time)
	add := func(snapshot slack.ChannelSnapshot) {
		latest := laterSlackTS(snapshot.Latest, snapshot.ThreadLatest)
		if !latest.IsZero() {
			result[snapshot.ID] = latest
		}
	}
	for _, ch := range counts.Channels {
		add(ch)
	}
	for _, im := range counts.IMs {
		add(im)
	}
	for _, mpim := range counts.MPIMs {
		add(mpim)
	}
	return result
}

func laterSlackTS(a, b string) time.Time {
	at, _ := slack.ParseSlackTS(a)
	bt, _ := slack.ParseSlackTS(b)
	if bt.After(at) {
		return bt
	}
	return at
}

func channelIDs(chans []slack.Channel) []string {
	ids := make([]string, 0, len(chans))
	for _, ch := range chans {
		ids = append(ids, ch.ID)
	}
	return ids
}

func channelIDSet(ids []string) map[string]bool {
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}
