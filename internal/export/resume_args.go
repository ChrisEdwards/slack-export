package export

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chrisedwards/slack-export/internal/slack"
	"github.com/rusq/slackdump/v4/source"
)

func (e *Exporter) resumeArgs(
	ctx context.Context,
	archiveDir string,
	tracked []slack.Channel,
	opts ResumeOptions,
) ([]string, bool, error) {
	if opts.Dedupe {
		args, err := fullSweepResumeArgs(ctx, archiveDir, tracked)
		return args, true, err
	}
	args, hasWork := e.scopedResumeArgs(ctx, archiveDir, tracked)
	return args, hasWork, nil
}

func fullSweepResumeArgs(ctx context.Context, archiveDir string, tracked []slack.Channel) ([]string, error) {
	src, err := source.Load(ctx, archiveDir)
	if err != nil {
		return nil, fmt.Errorf("loading archive checkpoints for full sweep: %w", err)
	}
	defer func() { _ = src.Close() }()

	latest, err := src.Latest(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading archive checkpoints for full sweep: %w", err)
	}
	coverageStart, err := archiveCoverageStart(archiveDir)
	if err != nil {
		return nil, fmt.Errorf("reading archive coverage for full sweep: %w", err)
	}
	if coverageStart.IsZero() {
		return nil, fmt.Errorf("archive coverage start unknown; refusing unbounded full sweep")
	}
	return fullSweepResumeArgsFromLatest(tracked, latest, coverageStart), nil
}

func fullSweepResumeArgsFromLatest[K interface {
	fmt.Stringer
	comparable
}](tracked []slack.Channel, latest map[K]time.Time, coverageStart time.Time) []string {
	trackedIDs := channelIDs(tracked)
	trackedSet := channelIDSet(trackedIDs)
	checkpoints := make(map[string]bool, len(latest))
	args := make([]string, 0)
	for link := range latest {
		key := fmt.Sprint(link)
		channelID := strings.SplitN(key, ":", 2)[0]
		if strings.Contains(key, ":") {
			continue
		}
		checkpoints[channelID] = true
	}
	for link := range latest {
		key := fmt.Sprint(link)
		channelID := strings.SplitN(key, ":", 2)[0]
		if !trackedSet[channelID] {
			args = append(args, "^"+key)
		}
	}
	if !coverageStart.IsZero() {
		for _, id := range trackedIDs {
			if !checkpoints[id] {
				args = append(args, id+","+coverageStart.UTC().Format(slackdumpTimeFormat))
			}
		}
	}
	return args
}
