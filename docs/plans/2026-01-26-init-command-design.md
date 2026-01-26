# Init Command Design

**Issue:** se-1d1 - Add init command for guided setup wizard
**Date:** 2026-01-26
**Status:** Approved

## Overview

Add an `init` command that walks users through first-time setup interactively. The wizard adapts based on existing state: first-time users get the full guided flow, returning users get a quick pass with pre-populated values they can change.

## Key Decisions

| Decision | Choice |
|----------|--------|
| Authentication | Shell out to `slackdump auth` |
| UI Framework | charmbracelet/huh for polished forms |
| Error Recovery | Exit with clear instructions (no retry/checkpoint) |
| Config Location | User config only: `~/.config/slack-export/slack-export.yaml` |
| Timezone | Detect system TZ, confirm with user, show list if declined |
| Existing Config | Pre-populate form with current values |
| Existing Auth | Skip auth step if credentials valid |
| `--config` Flag | Keep for explicit overrides (useful for testing) |
| Verification | Full connection test - fetch channels |
| TTY Check | Immediately on command start |
| Summary | Config path, sample channels, example commands, help hint |

## Command Structure

### Definition

```go
var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Set up slack-export with guided wizard",
    Long:  `Interactive setup wizard for first-time configuration.

Walks through:
  - Installing slackdump (if needed)
  - Authenticating with Slack (if needed)
  - Creating configuration file
  - Verifying the setup works`,
    RunE:  runInit,
}
```

### Flags

- `--force` - Skip "config exists" warning, still shows form with current values pre-populated
- `--config` - Write to specific path instead of user config (inherited from root)

### TTY Requirement

```go
func runInit(cmd *cobra.Command, args []string) error {
    if !term.IsTerminal(int(os.Stdin.Fd())) {
        return fmt.Errorf("init requires an interactive terminal")
    }
    // ... rest of wizard
}
```

## Step-by-Step Flow

### Step 1: Slackdump Binary Check

**Flow:**
1. Display "Step 1/4: Checking for slackdump..."
2. Use `export.FindSlackdump("")` to locate binary
3. If found: show path, continue
4. If not found: prompt to install

**Install Prompt:**
```
slackdump not found

slackdump is required to export Slack data. Install it now?

○ Yes, install
○ No, I'll install manually
```

**If user accepts:**
- Run `go install github.com/rusq/slackdump/v3/cmd/slackdump@latest`
- Show spinner while installing
- Verify with `exec.LookPath("slackdump")`
- If fails: explain `$GOPATH/bin` must be in PATH, exit

**If user declines:**
- Print manual install instructions, exit

**Success Output:**
```
Step 1/4: Checking for slackdump...
✓ Found slackdump at /Users/chris/go/bin/slackdump
```

### Step 2: Authentication Check

**Flow:**
1. Display "Step 2/4: Checking Slack authentication..."
2. Call `slack.LoadCredentials()`
3. If valid: show workspace name, continue
4. If invalid: prompt to authenticate

**Auth Prompt:**
```
Slack authentication required

slackdump needs to authenticate with your Slack workspace.
This will open a browser for you to sign in.

○ Authenticate now
○ Skip for now
```

**If user accepts:**
- Print "Running slackdump auth... (follow the prompts)"
- Run `slackdump auth` with stdin/stdout connected to terminal
- Verify with `slack.LoadCredentials()`
- If still fails: print error, exit

**If user declines:**
- Print "You can authenticate later with: slackdump auth"
- Continue (verification will fail later)

**Success Output:**
```
Step 2/4: Checking Slack authentication...
✓ Authenticated to workspace: my-company
```

### Step 3: Configuration Form

**Flow:**
1. Display "Step 3/4: Configuring slack-export..."
2. Show: "Config will be saved to: ~/.config/slack-export/slack-export.yaml"
3. Load existing config for defaults (if present)
4. Present form with pre-populated values
5. Validate and write config

**Form Fields:**

1. **Output Directory**
   - Text input with placeholder `./slack-logs`
   - Pre-populated with existing value if config exists
   - Expanded to absolute path before display/save

2. **Timezone**
   - First: confirm detected timezone "Use detected timezone? (America/New_York)"
   - If declined: show searchable select with common options
   - Options: America/New_York, America/Chicago, America/Denver, America/Los_Angeles, Europe/London, Europe/Paris, Asia/Tokyo, UTC

**Output Directory Validation:**

After user enters path, expand to absolute and validate:

```
Output directory: ./slack-logs

Expanded path: /Users/chris/projects/myproject/slack-logs
This directory doesn't exist.

○ Create it now
○ Proceed without creating (you'll need to create it before exporting)
○ Enter a different path
```

- "Create it now": create directory with 0750 permissions
- "Proceed without creating": warn and continue
- "Enter a different path": loop back to input

**Include/Exclude Patterns:**
- Skip in wizard (use empty defaults)
- Mention in summary that user can edit config file

**Write Config:**
- Create `~/.config/slack-export/` if needed (0750)
- Write YAML file (0600 permissions)

### Step 4: Verification

**Flow:**
1. Display "Step 4/4: Verifying setup..."
2. Load newly written config
3. Load credentials
4. Fetch channels via Edge API
5. Display sample channel names

**Verification Code:**
```go
cfg, err := config.Load("")
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

creds, err := slack.LoadCredentials()
if err != nil {
    return fmt.Errorf("credentials not available: %w", err)
}

client := edge.New(creds.Token, creds.Cookies)
channels, err := client.ListChannels(ctx)
if err != nil {
    return fmt.Errorf("failed to connect to Slack: %w", err)
}
```

**Success Output:**
```
Step 4/4: Verifying setup...
✓ Config loaded from ~/.config/slack-export/slack-export.yaml
✓ Connected to workspace: my-company
✓ Found 47 channels (showing first 5):
    #general
    #random
    #engineering
    #product
    #announcements
```

### Success Summary

```
════════════════════════════════════════════════════════════════

Setup complete!

Config saved to: ~/.config/slack-export/slack-export.yaml
Output directory: /Users/chris/projects/myproject/slack-logs
Timezone: America/New_York
Workspace: my-company

To customize include/exclude patterns, edit the config file.

Try these commands:
  slack-export channels          List your Slack channels
  slack-export export 2026-01-25 Export a specific date
  slack-export sync              Sync recent activity

Run 'slack-export --help' to see all available commands.

════════════════════════════════════════════════════════════════
```

### Partial Success (Auth Skipped)

```
════════════════════════════════════════════════════════════════

Setup partially complete.

⚠ Authentication skipped - run 'slackdump auth' before exporting

Config saved to: ~/.config/slack-export/slack-export.yaml
Output directory: /Users/chris/projects/myproject/slack-logs
Timezone: America/New_York

To customize include/exclude patterns, edit the config file.

Run 'slack-export --help' to see all available commands.

════════════════════════════════════════════════════════════════
```

## Config Package Changes

### Remove Local Config Support

Modify `internal/config/config.go`:

**Before (search order):**
1. Explicit `--config` path
2. `./slack-export.yaml`
3. `~/.config/slack-export/slack-export.yaml`

**After (search order):**
1. Explicit `--config` path
2. `~/.config/slack-export/slack-export.yaml`

### Add Save Method

```go
func (c *Config) Save(path string) error {
    // If path empty, use default user config location
    if path == "" {
        home, err := os.UserHomeDir()
        if err != nil {
            return fmt.Errorf("cannot determine home directory: %w", err)
        }
        path = filepath.Join(home, ".config", "slack-export", "slack-export.yaml")
    }

    // Create parent directory if needed
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0750); err != nil {
        return fmt.Errorf("cannot create config directory: %w", err)
    }

    // Marshal and write
    data, err := yaml.Marshal(c)
    if err != nil {
        return fmt.Errorf("cannot marshal config: %w", err)
    }

    if err := os.WriteFile(path, data, 0600); err != nil {
        return fmt.Errorf("cannot write config: %w", err)
    }

    return nil
}
```

## Dependencies

**Direct (promote from indirect):**
- `github.com/charmbracelet/huh` - Form/prompt library

**Existing (already used):**
- `github.com/spf13/cobra` - CLI framework
- `golang.org/x/term` - TTY detection

## Testing Approach

1. **Unit tests:**
   - TTY check logic
   - Path expansion and validation
   - Config Save method

2. **Integration tests:**
   - Test individual helper functions
   - Mock filesystem for directory creation tests

3. **Manual testing:**
   - Full interactive flow (difficult to automate huh forms)
   - Test on fresh system (no config, no slackdump)
   - Test with existing config (pre-population)
   - Test with existing auth (skip auth step)

## Error Handling

All errors exit immediately with clear instructions:

- **slackdump install fails:** Explain GOPATH/bin needs to be in PATH
- **slackdump auth fails:** Print manual auth instructions
- **Config write fails:** Print permissions error and path
- **Verification fails:** Print what failed and how to debug

No partial state is left behind. User can re-run `init` after fixing issues.

## File Changes

| File | Change |
|------|--------|
| `cmd/slack-export/main.go` | Add initCmd and runInit function |
| `internal/config/config.go` | Remove local config search, add Save method |
| `go.mod` | Promote huh to direct dependency |
