# Bundle Slackdump with slack-export

## Problem

slack-export depends on slackdump as an external binary. A bug in upstream slackdump requires a custom build until the fix is merged. We need to ship the custom slackdump with slack-export releases.

## Solution

Bundle the slackdump binary alongside slack-export in release archives. At runtime, prefer the bundled binary over system PATH.

## Release Archive Structure

Each platform release contains both binaries:

```
slack-export-v1.0.0-darwin-arm64.tar.gz
├── slack-export
├── slackdump
└── README.md (optional)
```

**Platforms:**
- darwin-arm64 (Apple Silicon)
- darwin-amd64 (Intel Mac)
- linux-amd64
- linux-arm64
- windows-amd64 (.zip with .exe files)

## Runtime Binary Resolution

`FindSlackdump()` priority order:

1. Bundled binary next to executable
2. System PATH (fallback for development)

```go
func FindSlackdump() (string, error) {
    // 1. Bundled binary next to executable
    if exe, err := os.Executable(); err == nil {
        bundled := filepath.Join(filepath.Dir(exe), "slackdump")
        if runtime.GOOS == "windows" {
            bundled += ".exe"
        }
        if _, err := os.Stat(bundled); err == nil {
            return bundled, nil
        }
    }

    // 2. System PATH (for development)
    if path, err := exec.LookPath("slackdump"); err == nil {
        return path, nil
    }

    return "", errors.New("slackdump not found - ensure it's installed alongside slack-export")
}
```

Removes the previous `configPath` parameter - no user configuration needed.

## GitHub Actions Release Workflow

Triggered on tag push (e.g., `v1.0.0`):

1. Check out slack-export
2. Check out slackdump fork
3. Build both binaries for each platform
4. Package into release archives
5. Create GitHub release with all archives

The workflow builds slackdump from the fork as part of the slack-export release process (no separate slackdump releases needed).

## Future: Upstream Transition

When upstream slackdump merges the bug fix, see **se-2g5** for the migration plan:
- Add version detection
- Use system slackdump if version >= minimum
- Fall back to bundled binary
- Eventually remove bundled binary from releases

## Implementation Checklist

**Prerequisites (manual):**
- [ ] Fork slackdump to GitHub (e.g., chrisedwards/slackdump)
- [ ] Push bug fix branch to the fork

**Code changes:**
- [ ] Update `FindSlackdump()` - remove configPath, add bundled-first logic
- [ ] Update all call sites of `FindSlackdump`
- [ ] Remove any config options for slackdump path
- [ ] Add `.exe` suffix handling for Windows

**Release infrastructure:**
- [ ] Create `.github/workflows/release.yml`
- [ ] Build matrix for all 5 platforms
- [ ] Package as .tar.gz (unix) / .zip (windows)
- [ ] Create GitHub release on tag push

**Testing:**
- [ ] Bundled binary detection works
- [ ] PATH fallback works for development
- [ ] Release workflow succeeds with test tag
