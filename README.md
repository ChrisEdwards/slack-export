# slack-export

A CLI tool that exports Slack channel logs to dated markdown files. It uses the Slack Edge API for fast channel detection and [slackdump](https://github.com/rusq/slackdump) for message export.

## Features

- **Fast channel discovery** via Slack's Edge API
- **Glob-based filtering** to include/exclude channels by name patterns
- **Timezone-aware** date boundaries for accurate daily exports
- **Automatic sync** from last export date to today
- **Clean markdown output** organized by date and channel
- **Human-readable DM names** (e.g., `dm_alice` instead of `dm_U015ANT8LLD`)
- **Slack Connect support** with automatic external user resolution and caching

## Quick Start

```bash
# Install
go install github.com/chrisedwards/slack-export/cmd/slack-export@latest

# Run guided setup
slack-export init

# List your channels
slack-export channels

# Export today's messages
slack-export sync
```

## Prerequisites

1. **Go 1.21+** - for building from source
2. **slackdump** - must be installed and authenticated
   ```bash
   go install github.com/rusq/slackdump/v3/cmd/slackdump@latest
   slackdump auth
   ```
3. **Slack workspace access** - you must be a member of the workspace you want to export

## Installation

### From source

```bash
git clone https://github.com/chrisedwards/slack-export.git
cd slack-export
make build
```

This creates a `slack-export` binary in the current directory.

### Using go install

```bash
go install github.com/chrisedwards/slack-export/cmd/slack-export@latest
```

## Getting Started

The easiest way to get started is with the interactive setup wizard:

```bash
slack-export init
```

This walks you through:
1. **Installing slackdump** - offers to install if not found
2. **Authenticating with Slack** - runs `slackdump auth` if needed
3. **Configuring output directory and timezone** - with sensible defaults
4. **Verifying the setup** - connects to Slack and shows sample channels

You can re-run `slack-export init --force` to reconfigure at any time.

## Configuration

Configuration is stored at `~/.config/slack-export/slack-export.yaml`:

```yaml
# Output directory for exported logs (default: ./slack-logs)
output_dir: ./slack-logs

# Timezone for date calculations (default: America/New_York)
timezone: America/New_York

# Channel name patterns to include (glob syntax, empty = all channels)
include:
  - "engineering-*"
  - "team-*"
  - "project-*"

# Channel name patterns to exclude (glob syntax)
exclude:
  - "*alarms"
  - "*-alerts"
  - "*-notifications"
  - "bot-*"

# Path to slackdump binary (optional, defaults to PATH lookup)
slackdump_path: /usr/local/bin/slackdump
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `output_dir` | `./slack-logs` | Directory where exports are saved |
| `timezone` | `America/New_York` | Timezone for date boundary calculations |
| `include` | `[]` | Glob patterns for channels to include (empty = all) |
| `exclude` | `[]` | Glob patterns for channels to exclude |
| `slackdump_path` | _(PATH lookup)_ | Explicit path to slackdump binary |

### Environment Variables

All options can be overridden via environment variables with the `SLACK_EXPORT_` prefix:

```bash
SLACK_EXPORT_OUTPUT_DIR=/data/slack-logs
SLACK_EXPORT_TIMEZONE=UTC
```

### Pattern Matching

Patterns use glob syntax and match against both channel **names** and **IDs**:

| Pattern | Matches |
|---------|---------|
| `*` | Any sequence of characters |
| `?` | Any single character |

Matching is **case-insensitive**.

**Examples:**

| Pattern | Matches | Doesn't Match |
|---------|---------|---------------|
| `*alarms` | `prod-alarms`, `devalarms`, `ALARMS` | `alarms-oncall` |
| `*-alerts` | `prod-alerts`, `staging-alerts` | `alerts`, `alertsbot` |
| `bot-*` | `bot-deploy`, `bot-notifications` | `mybot-test` |
| `team-?` | `team-a`, `team-b` | `team-ab`, `team` |
| `CFAU264UU` | Channel with ID `CFAU264UU` | Other channels |

**Filter logic:**
1. If a channel matches ANY exclude pattern (by name or ID), it is skipped
2. If include list is empty, all non-excluded channels are included
3. If include list is non-empty, only channels matching an include pattern are included

## Usage

### Interactive Setup

```bash
slack-export init
```

Guided wizard for first-time setup. Checks prerequisites, authenticates with Slack, and creates configuration.

### View Configuration

```bash
slack-export config
```

Shows current settings and the config file being used.

### List Channels

```bash
# List all active channels (with include/exclude patterns applied)
slack-export channels

# List channels with activity since a specific date
slack-export channels --since 2026-01-20
```

Use this to discover channel names for configuring patterns.

### Export Single Date

```bash
slack-export export 2026-01-22
```

### Export Date Range

```bash
# From a specific date to today
slack-export export --from 2026-01-15

# Specific date range
slack-export export --from 2026-01-15 --to 2026-01-20
```

### Sync (Automatic Date Detection)

```bash
slack-export sync
```

The sync command:
1. Scans the output directory for existing exports
2. Finds the most recent date (YYYY-MM-DD folder)
3. Re-exports from that date through today

The last export date is re-exported because it may have been incomplete.

### Global Flags

```bash
slack-export --config /path/to/config.yaml export 2026-01-22
slack-export --version
slack-export --help
```

## Output Structure

Exports are organized by date and channel:

```
slack-logs/
├── 2026-01-20/
│   ├── engineering-general.md
│   ├── team-backend.md
│   └── project-alpha.md
├── 2026-01-21/
│   ├── engineering-general.md
│   └── team-backend.md
└── 2026-01-22/
    └── engineering-general.md
```

Each markdown file contains the messages from that channel for that date.

### Direct Message Naming

Direct messages (DMs) are named using the other participant's username:
- `dm_alice.smith` - DM with Alice Smith
- `dm_bob` - DM with Bob

For external users from Slack Connect, names are resolved via the Slack API and cached locally for performance.

## Data Storage

slack-export stores data in standard locations:

| Data | Location | Purpose |
|------|----------|---------|
| Configuration | `~/.config/slack-export/slack-export.yaml` | User settings |
| User cache | `~/.cache/slack-export/users.json` | Cached external user info |
| Exports | Configured `output_dir` (default: `./slack-logs`) | Exported messages |

The user cache stores information about external Slack Connect users to avoid repeated API calls.

## How It Works

1. **Channel Discovery**: Uses Slack's Edge API (`/api/client.userBoot` and `/api/conversations.list`) to get active channels. This is much faster than the standard API.

2. **User Resolution**: Fetches workspace users and resolves DM names to human-readable usernames. External Slack Connect users are looked up via the `users.info` API and cached to disk.

3. **Filtering**: Applies include/exclude glob patterns to the channel list.

4. **Export**: Calls slackdump to archive messages for the specified time range.

5. **Format**: Uses slackdump's `convert` command to transform the archive into readable text.

6. **Organize**: Extracts and renames files into the dated directory structure.

## Troubleshooting

### "Slackdump credentials not found"

Run `slackdump auth` to authenticate with your Slack workspace.

### "Failed to decrypt credentials"

Credentials are machine-specific. If you authenticated on a different machine, run `slackdump auth` again.

### "No active channels found"

- Check that your include patterns match existing channels
- Try running `slack-export channels` to see what channels are available
- Verify your Slack authentication is still valid

### "All channels filtered out"

Your include/exclude patterns are too restrictive. Run `slack-export channels` to see available channels and adjust your patterns.

### Timezone Issues

Exports use the configured timezone for date boundaries. If messages appear on the wrong date:
1. Check your `timezone` setting matches your Slack workspace's primary timezone
2. Use `slack-export config` to verify the current setting

### DM shows user ID instead of name

If a DM appears as `dm_U015ANT8LLD` instead of `dm_alice`, the user couldn't be resolved. This can happen with:
- Deactivated users
- Users from Slack Connect organizations that restrict the `users.info` API

The user cache at `~/.cache/slack-export/users.json` can be manually edited if needed.

## Development

```bash
# Build
make build

# Run tests
make test

# Run linter
make check

# Run both
make check-test

# Verbose output
make test VERBOSE=1
```

## License

MIT
