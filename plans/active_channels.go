package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rusq/slackdump/v3/internal/cache"
	"github.com/rusq/slackdump/v3/internal/edge"
)

func main() {
	var (
		sinceStr  string
		outputFmt string
		showAll   bool
	)
	flag.StringVar(&sinceStr, "since", "", "Date to filter from (YYYY-MM-DD), defaults to yesterday")
	flag.StringVar(&outputFmt, "format", "tsv", "Output format: tsv, ids, or slackdump")
	flag.BoolVar(&showAll, "all", false, "Include all channels (not just those with activity since date)")
	flag.Parse()

	// Parse the since date
	var since time.Time
	if sinceStr == "" {
		since = time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	} else {
		var err error
		since, err = time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid date format. Use YYYY-MM-DD: %v\n", err)
			os.Exit(1)
		}
	}

	ctx := context.Background()

	// Get cache directory
	ucd, err := os.UserCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting user cache dir: %v\n", err)
		os.Exit(1)
	}
	cacheDir := filepath.Join(ucd, "slackdump")

	// Create manager and load provider
	m, err := cache.NewManager(cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating manager: %v\n", err)
		os.Exit(1)
	}

	current, err := m.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting current workspace: %v\n", err)
		os.Exit(1)
	}

	prov, err := m.LoadProvider(current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading provider: %v\n", err)
		os.Exit(1)
	}

	// Create edge client
	edgeClient, err := edge.New(ctx, prov)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating edge client: %v\n", err)
		os.Exit(1)
	}

	// Get channel counts (fast - returns only YOUR channels with activity data)
	counts, err := edgeClient.ClientCounts(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting client counts: %v\n", err)
		os.Exit(1)
	}

	// Get channel metadata from UserBoot
	userBoot, err := edgeClient.ClientUserBoot(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting user boot: %v\n", err)
		os.Exit(1)
	}

	// Build maps for channel info
	namesMap := make(map[string]string)
	archivedMap := make(map[string]bool)
	for _, ch := range userBoot.Channels {
		namesMap[ch.ID] = ch.Name
		archivedMap[ch.ID] = ch.IsArchived
	}

	// Combine all channel snapshots
	allChannels := make(map[string]edge.ChannelSnapshot)
	for _, ch := range counts.Channels {
		allChannels[ch.ID] = ch
	}
	for _, ch := range counts.IMs {
		allChannels[ch.ID] = ch
	}
	for _, ch := range counts.MPIMs {
		allChannels[ch.ID] = ch
	}

	// Filter and collect results
	type channelInfo struct {
		ID     string
		Name   string
		Latest time.Time
		Type   string
	}
	var results []channelInfo

	for id, snapshot := range allChannels {
		latestTime := time.Time(snapshot.Latest)

		// Skip if before the since date (unless showing all)
		if !showAll && latestTime.Before(since) {
			continue
		}

		// Skip archived channels
		if archivedMap[id] {
			continue
		}

		name := namesMap[id]
		chType := "channel"
		if name == "" {
			if len(id) > 0 && id[0] == 'D' {
				chType = "dm"
				name = "(DM)"
			} else {
				chType = "mpim"
				name = "(MPIM)"
			}
		}

		results = append(results, channelInfo{
			ID:     id,
			Name:   name,
			Latest: latestTime,
			Type:   chType,
		})
	}

	// Sort by latest message time (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Latest.After(results[j].Latest)
	})

	// Output based on format
	switch outputFmt {
	case "ids":
		// Just channel IDs, one per line (for piping to slackdump)
		for _, r := range results {
			fmt.Println(r.ID)
		}
	case "slackdump":
		// Format suitable for slackdump command line
		for i, r := range results {
			if i > 0 {
				fmt.Print(" ")
			}
			fmt.Print(r.ID)
		}
		fmt.Println()
	case "tsv":
		fallthrough
	default:
		// TSV with header
		fmt.Fprintf(os.Stderr, "Channels with activity since %s: %d\n", since.Format("2006-01-02"), len(results))
		fmt.Println("ID\tName\tLatest\tType")
		for _, r := range results {
			fmt.Printf("%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Latest.Format("2006-01-02 15:04"), r.Type)
		}
	}
}
