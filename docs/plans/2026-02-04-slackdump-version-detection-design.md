# Slackdump Version Detection Design

**Issue:** se-2g5 - Add slackdump version detection for upstream transition

**Goal:** Automatically use system slackdump when version >= 3.1.13 (the version containing the fix from PR #444), falling back to bundled binary for older versions.

**Context:** slack-export bundles slackdump because upstream had a bug. Now that the fix is released in v3.1.13, we want to prefer system slackdump when it's new enough, so users benefit from upstream improvements without waiting for slack-export releases.

---

## Architecture

**New priority order for `FindSlackdump()`:**
1. System PATH → check version → use if >= 3.1.13
2. Bundled binary (next to executable)
3. Error if none found

**New functions:**
- `SlackdumpVersion(binaryPath string) (string, error)` - runs `slackdump version`, parses output
- `CompareVersions(a, b string) (int, error)` - simple semver comparison

**Constants:**
- `MinSlackdumpVersion = "3.1.13"`

---

## Version Parsing

**`SlackdumpVersion` function:**

Input: path to slackdump binary
Output: version string (e.g., "3.1.13") or error

Implementation:
1. Run `slackdump version` with 5-second timeout
2. Parse output format: `Slackdump 3.1.13 (commit: abc12345) built on: 2024-01-15`
3. Extract version string between "Slackdump " and " ("
4. Return error if:
   - Binary doesn't exist or fails to run
   - Output doesn't match expected format
   - Version is "unknown" (dev builds)

---

## Version Comparison

**`CompareVersions` function:**

Input: two version strings (e.g., "3.1.13", "3.2.0")
Output: -1 if a < b, 0 if equal, 1 if a > b, or error for malformed input

Implementation:
1. Split both versions on "."
2. Parse each segment as integer
3. Compare major, then minor, then patch
4. Return error for non-numeric or malformed versions

No third-party semver library needed - slackdump uses simple X.Y.Z format without pre-release tags.

---

## FindSlackdump Flow

```
1. Check system PATH for slackdump (exec.LookPath)
   ├─ Found → get version via SlackdumpVersion()
   │   ├─ Version >= 3.1.13 → return PATH binary
   │   └─ Version < 3.1.13 or parse error → log reason, continue to step 2
   └─ Not found → continue to step 2

2. Check bundled binary (next to executable)
   ├─ Found → return bundled binary
   └─ Not found → continue to step 3

3. Return error: "slackdump not found"
```

**Logging:** When falling back to bundled, print message explaining why:
- "System slackdump version X.Y.Z is below minimum 3.1.13, using bundled binary"

**No version check on bundled:** We control the bundled version in releases.

---

## Testing Strategy

**Unit tests for `SlackdumpVersion`:**
- Mock binary outputting valid version
- Mock binary outputting "unknown"
- Mock binary with malformed output
- Non-existent binary path

**Unit tests for `CompareVersions`:**
- Equal: "3.1.13" vs "3.1.13"
- Greater: "3.2.0" vs "3.1.13", "4.0.0" vs "3.1.13"
- Less: "3.1.12" vs "3.1.13", "3.0.0" vs "3.1.13"
- Malformed: "abc", "3.1", ""

**Integration tests for `FindSlackdump`:**
- PATH has valid version → uses PATH
- PATH has old version → uses bundled
- No PATH binary → uses bundled
- Neither available → error

**Test approach:** Create shell scripts in temp directories that output controlled version strings.

---

## Files to Modify

- `internal/export/slackdump.go` - add version functions, update FindSlackdump
- `internal/export/slackdump_test.go` - add tests for new functions

---

## Future Considerations

Once the fix is widely adopted (6+ months), consider:
- Removing bundled binary from releases
- Simplifying FindSlackdump to just use PATH
- Updating documentation to require system slackdump
