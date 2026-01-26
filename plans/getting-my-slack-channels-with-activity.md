# Getting My Slack Channels With Activity

## Problem

Slackdump's `list channels` and `archive -member-only` commands fetch ALL channels in a workspace before filtering, which is extremely slow for large workspaces (thousands of channels).

**Real-world test:** Running `slackdump archive -member-only` on an enterprise workspace took 6+ minutes and was still iterating through non-member channels when cancelled.

I only want to:
1. Get channels I'm a member of
2. Filter to channels with recent activity
3. Do it FAST (seconds, not minutes)

## Solution

Slack's undocumented Edge API provides two fast endpoints that return only YOUR channels:

### 1. `client.userBoot`
- Returns all channels you're a member of with full metadata (name, archived status, etc.)
- Fast single API call
- Includes ~400+ channels in my case

### 2. `client.counts`
- Returns channels with activity metadata: `Latest` timestamp, `HasUnreads`, `MentionCount`
- Only returns channels with some activity (261 in my case vs 418 from userBoot)
- Perfect for filtering to "channels I care about"

## The `active_channels` Tool

A standalone tool that uses these fast endpoints to list channels with recent activity.

### Installation

```bash
# Build from slackdump source
cd /path/to/slackdump
go build -o ~/bin/active_channels ./cmd/active_channels/

# Or install to GOPATH/bin
go install ./cmd/active_channels/
```

### Usage

```bash
# Channels with activity since yesterday (default)
active_channels

# Channels since a specific date
active_channels -since 2026-01-15

# Output formats:
active_channels -format tsv       # Tab-separated (default)
active_channels -format ids       # Just channel IDs, one per line
active_channels -format slackdump # Space-separated for command line
```

### Example Output

```
$ active_channels -since 2026-01-20
Channels with activity since 2026-01-20: 67
ID              Name                    Latest              Type
C0AAZATFU8Y     mpdm-jake-shane-chris   2026-01-21 22:14    channel
C03HLJJF3DE     customer-dbs            2026-01-21 22:03    channel
CH3BS21QA       teamserver-deploys      2026-01-21 21:52    channel
DQNS4GZ1V       (DM)                    2026-01-21 19:53    dm
...
```

### Use with Slackdump

```bash
# Export channels with activity since yesterday
slackdump archive $(active_channels -format slackdump 2>/dev/null) \
  -time-from "2026-01-20" -time-to "2026-01-21" -o yesterday_export

# Or pipe IDs
active_channels -format ids 2>/dev/null | xargs slackdump archive \
  -time-from "2026-01-20" -o yesterday_export
```

**Performance:** Completes in ~1 second vs 6+ minutes with `-member-only`.

## How It Works

### Channel ID Prefixes
- `C` = Public/private channel
- `D` = Direct message (DM)
- `G` = Group DM (legacy)

### Key Data Structures

From `client.counts`:
```go
type ChannelSnapshot struct {
    ID             string
    LastRead       fasttime.Time  // When you last read the channel
    Latest         fasttime.Time  // Timestamp of latest message
    MentionCount   int            // Unread mentions of you
    HasUnreads     bool           // Whether there are unread messages
}
```

From `client.userBoot`:
```go
type UserBootChannel struct {
    ID         string
    Name       string
    IsArchived bool
    Created    int64   // Unix seconds
    Updated    int64   // Unix milliseconds (when archived)
    // ... more fields
}
```

### Archived Channels

- `ClientUserBoot` returns archived channels but `Latest` is empty
- `ClientCounts` HAS the `Latest` timestamp for archived channels
- `Updated` field (in milliseconds) indicates when the channel was archived
- Archived channels are excluded by default since they won't have recent activity

## Results (My Workspace)

| Metric | Count |
|--------|-------|
| Total channels I'm in | 418 |
| Active (non-archived) | 355 |
| Archived | 63 |
| With activity data | 261 |
| Activity since Jan 20 | 67 |
| Activity today (Jan 21) | 37 |

## Why Slackdump's `-member-only` is Slow

The current implementation in `internal/edge/slacker.go` runs THREE parallel operations:

```go
var pipeline = []func(){
    func() {
        // FAST - returns only YOUR channels
        ub, err := cl.ClientUserBoot(ctx)
    },
    func() {
        // FAST - returns YOUR DMs
        ims, err := cl.IMList(ctx)
    },
    func() {
        // SLOW - searches ALL workspace channels!
        ch, err := cl.SearchChannels(ctx, "")
    },
}
```

The `SearchChannels(ctx, "")` call paginates through EVERY channel in the workspace. The `-member-only` flag only filters AFTER all channels are fetched.

## GitHub Issue

Created issue #598 requesting a fast member-only mode:
https://github.com/rusq/slackdump/issues/598

## Files in This Folder

- `getting-my-slack-channels-with-activity.md` - This documentation
- `active_channels.go` - The active_channels tool source
- `test_channels.go` - Original exploration script

## Key Files in Slackdump

- `/internal/edge/client.go` - `ClientCounts()` implementation
- `/internal/edge/client_boot.go` - `ClientUserBoot()` implementation
- `/internal/edge/slacker.go` - `GetConversationsContext()` with the slow `SearchChannels("")`
- `/internal/edge/search.go` - `SearchChannels()` with `SearchOnlyMyChannels: false`
- `/internal/cache/manager.go` - Credential loading from stored workspaces
