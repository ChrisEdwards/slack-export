# External User Resolution for DM Channels

**Issue:** se-374
**Date:** 2026-01-26

## Problem

DM channels with Slack Connect users (from external organizations) display as `dm_U123456` instead of `dm_username` because the `users.list` API only returns users from the current workspace.

External users have a different `team_id` and aren't included in the workspace user list.

## Solution

Fetch external users on-demand via `users.info` API with persistent disk caching.

### Behavior

- **Lookup order:**
  1. Workspace users (from `users.list`, already fetched)
  2. Disk cache (previously fetched external users)
  3. API call via `users.info` → cache result for future sessions

- **Caching:**
  - Location: `~/.cache/slack-export/users.json`
  - Scope: Global (user IDs are globally unique in Slack)
  - Expiration: Never (delete file to reset)

- **Error handling:**
  - Fail-fast: any fetch error aborts the operation
  - Errors propagate to caller for visibility

### API Details

- `users.info` is Tier 4 rate limited (100+ requests/min)
- Typical workspace has few Slack Connect DMs, so impact is minimal

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    UserResolver                         │
├─────────────────────────────────────────────────────────┤
│  Username(userID) → (string, error)                     │
│                                                         │
│  1. Check primary UserIndex (workspace users)           │
│  2. Check UserCache (disk cache)                        │
│  3. Call UserFetcher.FetchUserInfo() → cache result     │
└─────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
   ┌──────────┐        ┌───────────┐        ┌────────────┐
   │UserIndex │        │ UserCache │        │ EdgeClient │
   │(in-memory)│       │  (disk)   │        │(UserFetcher)│
   └──────────┘        └───────────┘        └────────────┘
```

### New Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `UserCache` | `internal/slack/usercache.go` | Persistent cache for external users |
| `UserFetcher` | `internal/slack/edge_types.go` | Interface for fetching single user |
| `UserResolver` | `internal/slack/edge_types.go` | Combines index + cache + fetcher |
| `FetchUserInfo` | `internal/slack/edge.go` | Implements `UserFetcher` on EdgeClient |

### Cache File Format

```json
{
  "version": 1,
  "users": {
    "U03A0EQBAS3": {
      "user": {
        "id": "U03A0EQBAS3",
        "name": "steve",
        "real_name": "Steve Smith",
        "deleted": true,
        "profile": { "display_name": "Steve" }
      },
      "fetched_at": 1737900000
    }
  }
}
```

## Files to Modify

| File | Change |
|------|--------|
| `internal/slack/usercache.go` | New file: persistent cache |
| `internal/slack/edge_types.go` | Add `UserFetcher`, `UserResolver` |
| `internal/slack/edge.go` | Add `FetchUserInfo`, update `GetActiveChannelsWithUsers` |
| `cmd/slack-export/main.go` | Wire up cache and resolver |

## Acceptance Criteria

- [ ] DM channels with Slack Connect users show `dm_username` not `dm_U123456`
- [ ] External users cached to `~/.cache/slack-export/users.json`
- [ ] Cache persists across sessions
- [ ] Fetch errors fail-fast with clear error message
- [ ] No performance regression for workspaces without Slack Connect
