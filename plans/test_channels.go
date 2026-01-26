package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"time"

	"github.com/rusq/slackdump/v3/internal/cache"
	"github.com/rusq/slackdump/v3/internal/edge"
)

func main() {
	ctx := context.Background()

	// Get cache directory (~/Library/Caches/slackdump on macOS)
	ucd, err := os.UserCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting user cache dir: %v\n", err)
		os.Exit(1)
	}
	cacheDir := filepath.Join(ucd, "slackdump")

	// Create manager
	m, err := cache.NewManager(cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating manager: %v\n", err)
		os.Exit(1)
	}

	// Get current workspace
	current, err := m.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting current workspace: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Current workspace: %s\n", current)

	// Load provider
	prov, err := m.LoadProvider(current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading provider: %v\n", err)
		os.Exit(1)
	}

	// Test the provider
	testResp, err := prov.Test(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error testing provider: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Authenticated as: %s (Team: %s)\n", testResp.User, testResp.Team)

	// Create edge client
	edgeClient, err := edge.New(ctx, prov)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating edge client: %v\n", err)
		os.Exit(1)
	}

	// Call ClientCounts - returns only YOUR channels
	fmt.Println("\n=== ClientCounts (your channels with activity) ===")
	counts, err := edgeClient.ClientCounts(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting client counts: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nChannels: %d\n", len(counts.Channels))
	for i, ch := range counts.Channels {
		fmt.Printf("  %d. %s (mentions: %d, unreads: %v)\n", i+1, ch.ID, ch.MentionCount, ch.HasUnreads)
		if i >= 19 { // Limit output
			fmt.Printf("  ... and %d more\n", len(counts.Channels)-20)
			break
		}
	}

	fmt.Printf("\nDMs: %d\n", len(counts.IMs))
	for i, dm := range counts.IMs {
		fmt.Printf("  %d. %s (mentions: %d, unreads: %v)\n", i+1, dm.ID, dm.MentionCount, dm.HasUnreads)
		if i >= 9 {
			fmt.Printf("  ... and %d more\n", len(counts.IMs)-10)
			break
		}
	}

	fmt.Printf("\nGroup DMs (MPIMs): %d\n", len(counts.MPIMs))
	for i, mpim := range counts.MPIMs {
		fmt.Printf("  %d. %s (mentions: %d, unreads: %v)\n", i+1, mpim.ID, mpim.MentionCount, mpim.HasUnreads)
		if i >= 9 {
			fmt.Printf("  ... and %d more\n", len(counts.MPIMs)-10)
			break
		}
	}

	// Now try ClientUserBoot - returns your channels with full metadata
	fmt.Println("\n=== ClientUserBoot (your channels with full info) ===")
	userBoot, err := edgeClient.ClientUserBoot(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting user boot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nChannels from UserBoot: %d\n", len(userBoot.Channels))
	for i, ch := range userBoot.Channels {
		fmt.Printf("  %d. %s - %s\n", i+1, ch.ID, ch.Name)
		if i >= 19 {
			fmt.Printf("  ... and %d more\n", len(userBoot.Channels)-20)
			break
		}
	}

	// Summary
	fmt.Println("\n=== Summary ===")
	fmt.Printf("ClientCounts returned %d channels + %d DMs + %d MPIMs = %d total (FAST)\n",
		len(counts.Channels), len(counts.IMs), len(counts.MPIMs),
		len(counts.Channels)+len(counts.IMs)+len(counts.MPIMs))
	fmt.Printf("ClientUserBoot returned %d channels (with full metadata)\n", len(userBoot.Channels))

	// Build a map of channel counts for quick lookup
	countsMap := make(map[string]edge.ChannelSnapshot)
	for _, ch := range counts.Channels {
		countsMap[ch.ID] = ch
	}
	for _, ch := range counts.IMs {
		countsMap[ch.ID] = ch
	}
	for _, ch := range counts.MPIMs {
		countsMap[ch.ID] = ch
	}

	// Build a map of channel names from userBoot
	namesMap := make(map[string]string)
	archivedMap := make(map[string]bool)
	for _, ch := range userBoot.Channels {
		namesMap[ch.ID] = ch.Name
		archivedMap[ch.ID] = ch.IsArchived
	}

	// Write active channels with activity info
	f, err := os.Create("my_channels.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var archived, active int
	for _, ch := range userBoot.Channels {
		if ch.IsArchived {
			archived++
			continue
		}
		active++
		fmt.Fprintf(f, "%s\t%s\n", ch.ID, ch.Name)
	}
	fmt.Printf("\nWrote %d active channels to my_channels.txt (skipped %d archived)\n", active, archived)

	// Write channels with recent activity
	f2, err := os.Create("my_channels_with_activity.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating activity file: %v\n", err)
		os.Exit(1)
	}
	defer f2.Close()

	fmt.Fprintf(f2, "ID\tName\tLatest\tHasUnreads\tMentions\n")
	var withActivity int
	for id, snapshot := range countsMap {
		if archivedMap[id] {
			continue
		}
		name := namesMap[id]
		if name == "" {
			name = "(DM or MPIM)"
		}
		latestTime := time.Time(snapshot.Latest)
		fmt.Fprintf(f2, "%s\t%s\t%s\t%v\t%d\n", id, name, latestTime.Format("2006-01-02 15:04"), snapshot.HasUnreads, snapshot.MentionCount)
		withActivity++
	}
	fmt.Printf("Wrote %d channels with activity data to my_channels_with_activity.txt\n", withActivity)

	// Write archived channels with their last message date
	f3, err := os.Create("my_archived_channels.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating archived file: %v\n", err)
		os.Exit(1)
	}
	defer f3.Close()

	fmt.Fprintf(f3, "ID\tName\tLastMessage\tArchived\tCreated\n")
	var archivedCount int
	for _, ch := range userBoot.Channels {
		if !ch.IsArchived {
			continue
		}
		archivedCount++
		// Get Latest from ClientCounts (UserBoot doesn't have it for archived)
		var latestStr string
		if snapshot, ok := countsMap[ch.ID]; ok {
			latestTime := time.Time(snapshot.Latest)
			latestStr = latestTime.Format("2006-01-02 15:04")
		} else {
			latestStr = "(no data)"
		}
		// Updated is in milliseconds
		archivedTime := time.Unix(ch.Updated/1000, 0)
		createdTime := time.Unix(ch.Created, 0)
		fmt.Fprintf(f3, "%s\t%s\t%s\t%s\t%s\n", ch.ID, ch.Name, latestStr, archivedTime.Format("2006-01-02 15:04"), createdTime.Format("2006-01-02 15:04"))
	}
	fmt.Printf("Wrote %d archived channels to my_archived_channels.txt\n", archivedCount)

	// Find channels with activity since yesterday
	yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	fmt.Printf("\n=== Channels with activity since %s ===\n", yesterday.Format("2006-01-02"))

	f4, err := os.Create("channels_with_recent_activity.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating recent activity file: %v\n", err)
		os.Exit(1)
	}
	defer f4.Close()

	fmt.Fprintf(f4, "ID\tName\tLatest\tType\n")
	var recentCount int
	for id, snapshot := range countsMap {
		latestTime := time.Time(snapshot.Latest)
		if latestTime.Before(yesterday) {
			continue
		}
		recentCount++
		name := namesMap[id]
		chType := "channel"
		if name == "" {
			if id[0] == 'D' {
				chType = "DM"
			} else if id[0] == 'G' || (len(id) > 1 && id[0] == 'C' && archivedMap[id]) {
				chType = "MPIM"
			}
			name = "(DM or MPIM)"
		}
		if archivedMap[id] {
			chType = "archived"
		}
		fmt.Fprintf(f4, "%s\t%s\t%s\t%s\n", id, name, latestTime.Format("2006-01-02 15:04"), chType)
		fmt.Printf("  %s\t%s\t%s\t%s\n", id, name, latestTime.Format("2006-01-02 15:04"), chType)
	}
	fmt.Printf("\nFound %d channels with activity since yesterday\n", recentCount)
	fmt.Printf("Wrote to channels_with_recent_activity.txt\n")

	// Look up specific channel
	targetID := "DQNS4GZ1V"
	info, err := edgeClient.ConversationsGenericInfo(ctx, targetID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting info for %s: %v\n", targetID, err)
	} else if len(info) > 0 {
		ch := info[0]
		fmt.Printf("\n=== Info for %s ===\n", targetID)
		fmt.Printf("  Name: %s\n", ch.Name)
		fmt.Printf("  IsIM: %v\n", ch.IsIM)
		fmt.Printf("  User: %s\n", ch.User)

		// Look up the user
		if ch.User != "" {
			users, err := edgeClient.GetUsers(ctx, ch.User)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting user info: %v\n", err)
			} else if len(users) > 0 {
				u := users[0]
				fmt.Printf("  User Name: %s\n", u.Name)
				fmt.Printf("  Real Name: %s\n", u.Profile.RealName)
				fmt.Printf("  Display Name: %s\n", u.Profile.DisplayName)
			}
		}
	}
}
