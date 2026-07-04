// Package export orchestrates the Slack export workflow.
package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	} else if err := e.resumeArchive(ctx, archiveDir, tracked); err != nil {
		return err
	}

	from, to, err := e.renderWindow(now)
	if err != nil {
		return err
	}
	writes, err := RenderArchiveRange(ctx, archiveDir, e.cfg.OutputDir, from, to, e.cfg.Timezone)
	if err != nil {
		return err
	}
	fmt.Printf("Rendered %s through %s (%d changed file(s))\n", from, to, writes)
	return nil
}

func (e *Exporter) resumeArchive(ctx context.Context, archiveDir string, tracked []slack.Channel) error {
	resumeIDs := e.scopedResumeChannels(ctx, archiveDir, tracked)
	if len(resumeIDs) == 0 {
		fmt.Println("Archive already current")
		return nil
	}

	now := time.Now()
	opts := e.resumeOptions(archiveDir, now)
	fmt.Printf("Resuming archive for %d channel(s)\n", len(resumeIDs))
	if err := ResumeArchive(ctx, e.slackdump, archiveDir, resumeIDs, opts); err != nil {
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
	opts := ResumeOptions{
		Lookback:          e.cfg.Lookback,
		SkipStaleThreads:  e.cfg.SkipStaleThreads,
		SkipStaleChannels: e.cfg.SkipStaleThreads,
		Dedupe:            true,
	}
	if fullSweepDue(archiveDir, e.cfg.FullSweepInterval, now) {
		opts.SkipStaleThreads = ""
		opts.SkipStaleChannels = ""
	}
	return opts
}

func (e *Exporter) scopedResumeChannels(ctx context.Context, archiveDir string, tracked []slack.Channel) []string {
	fallback := channelIDs(tracked)
	counts, err := e.edgeClient.ClientCounts(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: counts scoping failed, resuming all channels: %v\n", err)
		return fallback
	}
	src, err := source.Load(ctx, archiveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: archive checkpoint load failed, resuming all channels: %v\n", err)
		return fallback
	}
	defer func() { _ = src.Close() }()

	latest, err := src.Latest(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: archive checkpoint read failed, resuming all channels: %v\n", err)
		return fallback
	}

	checkpoints := make(map[string]time.Time, len(latest))
	for link, ts := range latest {
		key := fmt.Sprint(link)
		if !strings.Contains(key, ":") {
			checkpoints[key] = ts
		}
	}

	countLatest := countsLatestByID(counts)
	var moved []string
	for _, ch := range tracked {
		checkpoint, ok := checkpoints[ch.ID]
		if !ok {
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

func (e *Exporter) renderWindow(now time.Time) (string, string, error) {
	loc, err := time.LoadLocation(e.cfg.Timezone)
	if err != nil {
		return "", "", fmt.Errorf("loading timezone: %w", err)
	}
	lookback, err := parseFriendlyDuration(e.cfg.Lookback)
	if err != nil {
		return "", "", fmt.Errorf("parsing lookback: %w", err)
	}
	days := int(lookback.Hours()/24) + 1
	to := now.In(loc)
	from := to.AddDate(0, 0, -days)
	return from.Format("2006-01-02"), to.Format("2006-01-02"), nil
}

func (e *Exporter) seedDate(now time.Time) (string, error) {
	if e.cfg.SeedDate != "" {
		if _, _, err := GetDateBounds(e.cfg.SeedDate, e.cfg.Timezone); err != nil {
			return "", fmt.Errorf("invalid seed_date: %w", err)
		}
		return e.cfg.SeedDate, nil
	}
	if date, err := findEarliestExportDate(e.cfg.OutputDir); err != nil {
		return "", err
	} else if date != "" {
		return date, nil
	}
	loc, err := time.LoadLocation(e.cfg.Timezone)
	if err != nil {
		return "", fmt.Errorf("loading timezone: %w", err)
	}
	return now.In(loc).Format("2006-01-02"), nil
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func sanitizePathPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "workspace"
	}
	return b.String()
}

func archiveExists(archiveDir string) bool {
	_, err := os.Stat(filepath.Join(archiveDir, source.DefaultDBFile))
	return err == nil
}

var exportDateDirPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func findEarliestExportDate(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var earliest string
	for _, entry := range entries {
		if !entry.IsDir() || !exportDateDirPattern.MatchString(entry.Name()) {
			continue
		}
		if earliest == "" || entry.Name() < earliest {
			earliest = entry.Name()
		}
	}
	return earliest, nil
}

func parseFriendlyDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	if days, ok, err := parseDayDuration(value); ok || err != nil {
		return days, err
	}
	return time.ParseDuration(value)
}

func parseDayDuration(value string) (time.Duration, bool, error) {
	if !strings.HasSuffix(value, "d") {
		return 0, false, nil
	}
	n, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
	if err != nil {
		return 0, true, err
	}
	return time.Duration(n) * 24 * time.Hour, true, nil
}

func fullSweepDue(archiveDir, interval string, now time.Time) bool {
	duration, err := parseFriendlyDuration(interval)
	if err != nil || duration == 0 {
		return false
	}
	data, err := os.ReadFile(fullSweepPath(archiveDir))
	if err != nil {
		return true
	}
	last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return true
	}
	return !now.Before(last.Add(duration))
}

func markFullSweep(archiveDir string, now time.Time) error {
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		return fmt.Errorf("creating archive metadata directory: %w", err)
	}
	if err := os.WriteFile(fullSweepPath(archiveDir), []byte(now.Format(time.RFC3339)), 0600); err != nil {
		return fmt.Errorf("writing full sweep marker: %w", err)
	}
	return nil
}

func fullSweepPath(archiveDir string) string {
	return filepath.Join(archiveDir, ".slack-export-last-full-sweep")
}

func channelIDs(chans []slack.Channel) []string {
	ids := make([]string, 0, len(chans))
	for _, ch := range chans {
		ids = append(ids, ch.ID)
	}
	return ids
}
