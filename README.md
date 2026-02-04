# slack-export

Export your Slack conversations to markdown files that AI agents can read.

**The problem:** AI coding agents like Claude Code can't access Slack. They can't see what your team discussed, what decisions were made, or what tasks were assigned.

**The solution:** Run `slack-export sync` daily to export your Slack channels to local markdown files. Point your AI agent at the folder, and it can now search and reference your team's conversations.

```
~/slack-logs/
├── 2025-01-25/
│   ├── 2025-01-25-engineering.md
│   ├── 2025-01-25-team-backend.md
│   └── 2025-01-25-dm_alice.md
├── 2025-01-26/
│   ├── 2025-01-26-engineering.md
│   ├── 2025-01-26-project-atlas.md
│   └── 2025-01-26-dm_bob.md
└── 2025-01-27/
    └── 2025-01-27-engineering.md
```

Each file contains that day's messages in clean, readable markdown. Filenames include the date so they stay unique when you copy multiple days to one folder or upload to AI tools like Gemini or NotebookLM.

## Features

- **Daily sync** - exports from your last export date through today
- **Glob filtering** - include/exclude channels by pattern (e.g., `team-*`, `*-alerts`)
- **Human-readable DMs** - `dm_alice` instead of `dm_U015ANT8LLD`
- **Timezone-aware** - accurate date boundaries for your location
- **Slack Connect support** - resolves external users automatically

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/ChrisEdwards/slack-export/main/install.sh | sh
```

This auto-detects your platform and installs both `slack-export` and `slackdump` to `~/.local/bin`. Run the same command to upgrade. The installer tells you what to do next.

To install to a different directory:
```bash
INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/ChrisEdwards/slack-export/main/install.sh | sh
```

See [Alternative Installation](#alternative-installation) for manual download or building from source.

## Getting Started

### 1. Authenticate with Slack

```bash
slackdump workspace new <workspace-url>
```

For example:
```bash
slackdump workspace new https://mycompany.slack.com
```

This opens a browser to authenticate with your Slack workspace. Your credentials are stored locally and encrypted. For Enterprise Grid workspaces, use the individual workspace URL (e.g., `team.slack.com`), not the enterprise URL.

### 2. Run the setup wizard

```bash
slack-export init
```

Walks you through configuration:
- Output directory for exported logs
- Timezone for date boundaries
- Verifies connection to Slack

Re-run with `--force` to reconfigure anytime.

### 3. Export your messages

```bash
slack-export sync
```

Exports all channels from your last export date through today. On first run, exports today's messages.

Sync always re-exports the most recent date in your output directory. This ensures you get a complete day even if you ran a previous export mid-day.

Run `slack-export sync` daily (or add it to a cron job) to keep your logs up to date.

### Backfilling history

`sync` always continues from the most recent date in your output directory—it doesn't fill gaps. To backfill history, use `export` for the full date range you need:

```bash
# Export from a specific date through today
slack-export export --from 2025-01-01

# Export a specific date range
slack-export export --from 2025-01-01 --to 2025-01-15
```

**Typical workflow:**
1. Run `slack-export sync` to start exporting from today
2. To add history, run `slack-export export --from <start-date>` to backfill from that date through today
3. Future `sync` calls continue from the most recent date

### Configuring channels

By default, all channels you're a member of are exported. To see all your channels:

```bash
slack-export channels
```

This shows every channel you're a member of (or have been) with any activity ever. To see which channels have recent activity:

```bash
slack-export channels --since 2025-01-20
```

Note: The `channels` command shows what *could* be exported. When you run `sync` or `export`, only channels with actual messages on each specific day are included in that day's export.

To change which channels are exported, edit `~/.config/slack-export/slack-export.yaml`:

```yaml
include:
  - "engineering-*"    # glob pattern for channels you're a member of
  - "team-*"
  - "C01ABC123DE"      # channel ID for a channel you're NOT a member of

exclude:
  - "*-alerts"         # channel name or ID works for excludes
  - "bot-*"
```

**Including channels you're not a member of:** Use the channel ID (e.g., `C01ABC123DE`), not the name. Find the channel ID in Slack by opening the channel, clicking the channel name, and scrolling to the bottom of the "About" tab.

**Excluding channels:** Use either the channel name or ID. Names only work for channels you're a member of.

After editing, run `slack-export channels` again to verify your changes.

### Day boundaries

Exports use a 3am-to-3am day boundary instead of midnight. This keeps late-night work sessions together—if you're doing customer support until 2am, those messages stay with the previous day rather than splitting at midnight.

The boundary uses your configured timezone.

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
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `output_dir` | `./slack-logs` | Directory where exports are saved |
| `timezone` | `America/New_York` | Timezone for date boundary calculations |
| `include` | `[]` | Glob patterns for channels to include (empty = all) |
| `exclude` | `[]` | Glob patterns for channels to exclude |

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
│   ├── 2026-01-20-engineering-general.md
│   ├── 2026-01-20-team-backend.md
│   └── 2026-01-20-dm_alice.md
├── 2026-01-21/
│   ├── 2026-01-21-engineering-general.md
│   └── 2026-01-21-team-backend.md
└── 2026-01-22/
    └── 2026-01-22-engineering-general.md
```

Direct messages use the other participant's username (e.g., `dm_alice`). External Slack Connect users are resolved via the API and cached locally.

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

4. **Export**: Calls slackdump to archive messages for the specified time range. If you have slackdump >= 3.1.13 installed on your system, slack-export uses it automatically. Otherwise, it uses the bundled version.

5. **Format**: Uses slackdump's `convert` command to transform the archive into readable text.

6. **Organize**: Extracts and renames files into the dated directory structure.

## Troubleshooting

### "Slackdump credentials not found"

Run `slackdump workspace new <workspace-url>` to authenticate with your Slack workspace.

### "Failed to decrypt credentials"

Credentials are machine-specific. If you authenticated on a different machine, run `slackdump workspace new <workspace-url>` again.

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

## Known Issues

### Thread replies only appear on the thread's creation date

Slack threads are exported on the date the thread was created, not when replies are added. If someone replies to a thread days later, that reply will not appear in the export for the reply date - it remains with the original thread's date.

This is a limitation of how slackdump organizes thread data.

## Alternative Installation

### Manual Download

Download the appropriate archive from [Releases](https://github.com/ChrisEdwards/slack-export/releases):

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `slack-export-vX.X.X-darwin-arm64.tar.gz` |
| macOS (Intel) | `slack-export-vX.X.X-darwin-amd64.tar.gz` |
| Linux (x86_64) | `slack-export-vX.X.X-linux-amd64.tar.gz` |
| Linux (ARM64) | `slack-export-vX.X.X-linux-arm64.tar.gz` |
| Windows | `slack-export-vX.X.X-windows-amd64.zip` |

Extract and install:

```bash
# macOS/Linux
tar -xzf slack-export-*.tar.gz
mv slack-export slackdump ~/.local/bin/

# Windows (PowerShell)
Expand-Archive slack-export-*.zip -DestinationPath .
# Move slack-export.exe and slackdump.exe to a directory in your PATH
```

### From Source

Requires Go 1.21+:

```bash
git clone https://github.com/chrisedwards/slack-export.git
cd slack-export
make build

# Also install slackdump separately
go install github.com/rusq/slackdump/v3/cmd/slackdump@latest
```

### Uninstall

```bash
# Remove binaries
rm ~/.local/bin/slack-export ~/.local/bin/slackdump

# Remove config and cache (optional)
rm -rf ~/.config/slack-export ~/.cache/slack-export
```

## Development

```bash
make build       # Build
make test        # Run tests
make check       # Run linter
make check-test  # Run both
```

## License

MIT
