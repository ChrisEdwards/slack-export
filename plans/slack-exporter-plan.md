# slack-export Tool Implementation Plan

## Overview

Build a standalone Go CLI tool that exports Slack channel logs to dated markdown files. The tool will:
1. Detect active channels using Slack's Edge API (fast, ~1 second)
2. Apply include/exclude patterns
3. Call `slackdump` for actual message export
4. Organize output into dated folders as markdown files

## Repository Setup

**Location:** `~/projects/oss/slack-export`
**Module:** `github.com/chrisedwards/slack-export`

## Architecture

```
slack-export/
├── cmd/
│   └── slack-export/
│       └── main.go           # CLI entry point (cobra)
├── internal/
│   ├── config/
│   │   └── config.go         # YAML config loading
│   ├── slack/
│   │   ├── credentials.go    # Read slackdump's cached credentials
│   │   ├── edge.go           # Edge API client (standalone HTTP)
│   │   └── types.go          # API response types
│   ├── channels/
│   │   └── filter.go         # Include/exclude pattern matching
│   └── export/
│       ├── exporter.go       # Orchestrates the export process
│       └── slackdump.go      # Wraps slackdump CLI calls
├── config.example.yaml       # Example config file
├── go.mod
├── go.sum
├── Makefile                  # Build targets
└── README.md
```

## Configuration File (slack-export.yaml)

```yaml
# Output settings
output_dir: "./slack-logs"     # Where to write exports
timezone: "America/New_York"   # Target timezone for date boundaries

# Channel filtering
include:
  - "eng-*"                    # Glob patterns
  - "dm_bob_smith"             # Exact names
  - "C03TSU00NK1"              # Channel IDs also work

exclude:
  - "_app_*"                   # App notification channels
  - "*-deploys"                # Deploy notification channels
  - "random"                   # Exact name

# Optional: Override slackdump path (defaults to finding in PATH)
# slackdump_path: "/usr/local/bin/slackdump"
```

## CLI Interface

```bash
# Export single date
slack-export export 2026-01-22

# Export date range
slack-export export --from 2026-01-15 --to 2026-01-20

# Export from a date to today
slack-export export --from 2026-01-15

# Sync: auto-detect last export date, export from there to today
# (Looks at output_dir for most recent dated folder, re-exports that day + all subsequent)
slack-export sync

# List active channels (for debugging/discovery)
slack-export channels
slack-export channels --since 2026-01-20

# Show current config
slack-export config

# Use alternate config file
slack-export --config /path/to/config.yaml export 2026-01-22
```

## Output Structure

```
slack-logs/
├── 2026-01-22/
│   ├── 2026-01-22-engineering.md
│   ├── 2026-01-22-dm_bob_smith.md
│   └── 2026-01-22-ai-team.md
├── 2026-01-23/
│   └── ...
```

## Implementation Steps

### Step 1: Initialize Repository
- Create `~/projects/oss/slack-export`
- `go mod init github.com/chrisedwards/slack-export`
- Add dependencies: `cobra` (CLI), `viper` (config), `gopkg.in/yaml.v3`

### Step 2: Implement Credential Reading
File: `internal/slack/credentials.go`

Read slackdump's cached credentials from `~/Library/Caches/slackdump/`:
- Read `workspace.txt` to get current workspace name
- Decrypt `{workspace}.bin` using AES-256-CFB with machine UUID as key
- Extract token and cookies from JSON

Key functions:
- `LoadCredentials() (*Credentials, error)`
- `GetMachineID() (string, error)` - uses `sysctl hw.uuid` on macOS

### Step 3: Implement Edge API Client
File: `internal/slack/edge.go`

Make direct HTTP calls to Slack's Edge API:
- `ClientUserBoot(ctx) (*UserBootResponse, error)` - gets channel metadata
- `ClientCounts(ctx) (*CountsResponse, error)` - gets activity timestamps
- `GetActiveChannels(ctx, since time.Time) ([]Channel, error)` - combines both

Endpoints:
- `POST https://edgeapi.slack.com/cache/{TEAM_ID}/client.userBoot`
- `POST https://edgeapi.slack.com/cache/{TEAM_ID}/client.counts`

### Step 4: Implement Channel Filtering
File: `internal/channels/filter.go`

Pattern matching for include/exclude:
- `MatchPattern(pattern, value string) bool` - glob matching using `filepath.Match`
- `FilterChannels(channels []Channel, include, exclude []string) []Channel`

Logic:
1. If channel matches any exclude pattern → skip
2. If include list is empty → include all non-excluded
3. If include list is non-empty → only include if matches include pattern

### Step 5: Implement Export Orchestration
File: `internal/export/exporter.go`

Main export workflow:
1. Load config
2. Get active channels for the date (Edge API)
3. Filter channels (include/exclude)
4. Calculate timezone-adjusted time boundaries
5. Call slackdump to archive channels
6. Process slackdump output to dated folder structure
7. Rename .txt files to .md

### Step 6: Implement Slackdump Wrapper
File: `internal/export/slackdump.go`

Wrapper for slackdump CLI:
- `Archive(ctx, channelIDs []string, timeFrom, timeTo time.Time) (string, error)`
- `FormatText(ctx, archiveDir string) (string, error)`
- Parse slackdump output to get directory/zip paths

### Step 7: Implement CLI Commands
File: `cmd/slack-export/main.go`

Cobra commands:
- `rootCmd` - global flags (--config)
- `exportCmd` - export single date or date range (positional date arg, or --from/--to flags)
- `syncCmd` - auto-detect last export, sync to today (no args needed)
- `channelsCmd` - list active channels (--since flag)
- `configCmd` - show current config

Sync logic:
1. Scan output_dir for dated folders (YYYY-MM-DD pattern)
2. Find most recent date
3. Re-export that date (in case it was partial) through today

### Step 8: Create Supporting Files
- `config.example.yaml` - documented example config
- `Makefile` - build, install, clean targets
- `README.md` - installation and usage docs

## Key Dependencies

```go
require (
    github.com/spf13/cobra v1.8.0      // CLI framework
    github.com/spf13/viper v1.18.0     // Config management
    gopkg.in/yaml.v3 v3.0.1            // YAML parsing
)
```

## Timezone Handling

For a given export date (e.g., 2026-01-22) in timezone America/New_York:
- Start: 2026-01-22 00:00:00 EST → convert to UTC for slackdump
- End: 2026-01-22 23:59:59 EST → convert to UTC for slackdump

Use Go's `time.LoadLocation()` and `time.In()` for conversion.

## Error Handling

- Missing config file → create from example with prompts
- Invalid credentials → clear error message pointing to slackdump auth
- slackdump not found → error with install instructions
- No active channels → skip with informational message
- Partial export failure → continue with other channels, report at end

## Verification

1. **Unit tests**: Config loading, pattern matching, timezone conversion
2. **Integration test**:
   ```bash
   # Create test config
   cp config.example.yaml slack-export.yaml
   # Edit with real settings

   # Test channel listing
   slack-export channels --since 2026-01-20

   # Test single date export
   slack-export export 2026-01-22

   # Verify output structure
   ls -la slack-logs/2026-01-22/
   ```
3. **Verify markdown files** have correct content and headers
