# Capture Late Thread Replies via slackdump v4 Resume

## Problem

Thread replies only appear in the export if they were posted on the day the
thread started. A reply posted several days after the thread started never
lands in any file.

Root cause, verified in slackdump `stream/conversation.go`. Two behaviors
combine.

1. Thread replies are only discovered through their parent message.
   `conversations.history` returns only parent messages, so when we export day
   N+2, a thread started on day N is invisible. Its parent falls outside the
   `-time-from`/`-time-to` window, so the thread fetch never fires.
2. Even when the parent is inside the window, the `conversations.replies` call
   is bounded by the same window. Both `Oldest` and `Latest` fall back to the
   stream-level bounds (`NVLTime(req.Oldest, cs.oldest)`), so replies outside
   the day window are dropped.

The `sync` command re-exports the last export date, which catches same-day
stragglers, but nothing ever recovers replies that arrive on later days.

## What Changed Upstream

slackdump v4 (current release v4.4.1) restructured around a persistent
archive and added a `resume` command that solves this exact problem.

- `archive` now writes a SQLite database by default instead of chunk files.
- `resume <archive>` continues an existing archive from per-channel and
  per-thread checkpoints.
- `resume -threads` re-fetches known threads and picks up new replies
  regardless of when the thread started.
- `-lookback p7d` (default 7 days) sets `oldest` for API requests, which also
  catches brand-new threads hanging off historical messages.
- `-skip-stale-threads p21d` drops dormant threads before any API call fires,
  keeping nightly runs fast and rate-limit friendly.
- `-skip-complete-threads` skips threads where the DB already has all replies
  (append-only optimization, misses edits/deletes inside those threads).
- `-dedupe` removes the duplicate rows that the lookback overlap re-fetches.
- `resume` accepts an optional entity list, so the caller can control which
  channels are touched.
- The `source` Go package (`github.com/rusq/slackdump/v4/source`) provides a
  read API over the database archive. `Load`, `Channels`, `Users`,
  `AllMessages`, `AllThreadMessages`, and `Sorted` are everything a renderer
  needs.

This flips the architecture. Fetching becomes slackdump's job entirely, and
slack-export becomes a renderer over the database.

## Solution Overview

Keep one long-lived slackdump database archive per workspace as the source of
truth. On each sync, run `resume` to pull new messages and late thread
replies into it, then render the day files from the database.

```
                 slackdump resume -threads
Slack API  ─────────────────────────────────►  DB archive (SQLite)
                                                    │
                                                    │  slack-export renderer
                                                    │  (source package, in-process)
                                                    ▼
                                          slack-logs/YYYY-MM-DD/
                                            YYYY-MM-DD-channel.md
```

`slackdump format text` and the zip extraction pipeline go away for sync.
slack-export imports the `source` package and renders markdown itself, which
gives it full control over thread placement.

### Sync flow

1. Bootstrap. If the archive does not exist, create it with
   `slackdump archive -files=false <filtered channels...>` using
   `-time-from` = the configured seed date (first run uses today unless
   configured).
2. Refresh channel list via the existing Edge API detection, apply
   include/exclude filters, and pass the result to resume as the entity list.
3. Run `slackdump resume -threads -lookback <lookback> -dedupe <archive>`,
   adding `-skip-stale-threads <staleness>` when configured.
4. Render all dates within the render window (lookback plus one day of
   margin) from the database. Write a file only when its content differs from
   what is on disk, so downstream change detection stays accurate.

A periodic full sweep (resume without the skip-stale flags) revisits dormant
threads and channels. This is a config knob, not a separate command, and runs
when the last full sweep is older than the configured interval.

### Why the render window is sufficient

Everything resume can change is bounded by the lookback. New replies are
recent, so they land on recent day files. Edits and re-fetches are bounded by
`oldest = now - lookback`. Rendering `lookback + 1 day` therefore covers every
file that can have changed. A `--full` flag on a new `render` command allows a
complete rebuild from the database when needed.

## Storage Format

Rule. Every message lives in the file for the work day it was posted in,
using the existing 3am-to-2:59:59am boundary (`GetDateBounds` in
`internal/export/timezone.go`) in the configured timezone. This matters for downstream consumers that read "today's"
file (daily briefs, consolidation). Folding a late reply back into a file from
three days ago would hide it from anything that processes days forward.

Each day file has two parts.

**Base section.** Messages posted that day, same as today. Threads started
that day render nested under the parent with the existing `|   ` prefix,
including only that day's replies.

**Thread continuations section.** Appended at the end of the file. Holds
replies posted that day to threads started on earlier days, one block per
thread, ordered by first new reply time. Each block quotes the parent as
labeled context and links back to the origin file.

```markdown
---

## Thread continuations
Replies posted this day in threads started on earlier days.
Lines marked [context] are repeated from the original day for readability.

### Thread started 2026-07-01 (see 2026-07-01/2026-07-01-ai-auto-remediation.md)
[context] > Jacob Mages-Haskins [U034W6GRVPD] @ 01/07/2026 14:22:10 Z:
[context] Deploy failed on smartfix backend after the image bump...

|   > Alex Corll [U0A084AH8AU] @ 03/07/2026 12:01:00 Z:
|   Fixed by bumping the base image tag, deploy is green now.
```

Properties this buys us.

- Today's activity is visible to anything that reads today's file.
- No double counting across days. Context lines are explicitly labeled, so an
  AI consumer can distinguish "said today" from "repeated for context."
- Deterministic and idempotent. The file is a pure function of the database
  and the date, so re-rendering after new replies arrive produces the same
  file plus the new block. No append bookkeeping, no state file.
- Search still finds everything, and each block links to the origin file.

If a channel has no base activity on a date but a tracked thread got replies
that day, the file is created containing only the continuations section.

### Rendering details

- Day bucketing reuses `GetDateBounds`, the 3am-to-2:59:59am work-day window
  in the configured timezone (also keeps se-2v1, configurable boundary, a
  one-place change). Timestamps inside message headers stay UTC with the `Z`
  suffix, matching the current format.
- Skip replies with subtype `thread_broadcast` in continuation blocks. They
  were "also sent to channel" and already appear in the base section as
  channel messages.
- Resolve `<@U...>` mentions from the archive's users table, matching what
  `format text` did. Reuse the existing user index and external-user cache
  for Slack Connect users.
- Parent context shows the full parent text. If threads with very large
  parents become a problem, truncate with a marker, but start simple.
- Message headers keep the existing shape,
  `> Display Name [U012345] @ DD/MM/YYYY HH:MM:SS Z:`.

## Configuration Changes

New keys, all optional with defaults.

```yaml
archive_dir: ~/.local/share/slack-export/archive   # DB archive location
seed_date: ""                # first-run archive start date (YYYY-MM-DD)
lookback: 7d                 # resume lookback window
skip_stale_threads: 21d      # skip threads idle longer than this ("" = off)
full_sweep_interval: 7d      # periodic resume without skip-stale flags
```

The archive is keyed by workspace under `archive_dir` so multiple workspaces
do not collide.

## Command Changes

- `sync` switches to the resume-plus-render flow described above.
- `export <date>` and `export --from/--to` render from the database. If the
  requested date predates archive coverage, fail with a message telling the
  user to reseed (see Migration). Dates newer than coverage trigger a resume
  first.
- New `render` command regenerates files from the database without touching
  the network. `render --full` rebuilds every date. Useful after format
  changes and for debugging.
- `init` gains the archive bootstrap step.

## Performance

Sync time is dominated by rate-limited Slack API calls in both the old and
new designs. Without further work, resume is roughly a wash. It replaces the
per-day channel scans with a single checkpointed pass (cheaper) but adds
`conversations.replies` calls for every non-stale thread (more expensive).
Upstream documents thread scanning as the slow path and the main source of
429 retries, hence the skip flags. The local pipeline changes (dropping
`format text` and zip extraction, rendering from SQLite) do not matter, since
local work was never the bottleneck.

### Counts-driven scoping

The Edge API's `client.counts` endpoint returns the latest-activity timestamp
for every channel, DM, and MPIM in a single call, and slack-export already
calls it for active-channel detection. Use it to scope resume.

1. Call `client.counts` once per sync.
2. Compare each tracked channel's `latest` against the archive's checkpoint
   for that channel (`source.Resumer.Latest`).
3. Pass only channels that moved to resume as the entity list.

On a quiet day with ~120 tracked channels and ~15 active, this is ~16
rate-limited calls instead of 120 plus threads. A no-change sync becomes a
handful of calls.

**Open question to verify during implementation.** The counts request already
sets `thread_counts_by_channel: true`, but the response parsing drops the
thread data, and it is unknown whether a thread-only reply moves a channel's
`latest`. Test empirically (reply to an old thread in a quiet channel, call
counts). If thread replies move `latest` or the per-channel thread counts
expose activity, thread refresh can also be scoped to channels with movement,
and the nightly thread sweep collapses to near zero on quiet days. If not,
thread scoping falls back to the staleness flags below.

### Other levers

- `-skip-complete-threads` caps each checked thread at one replies page.
- Tiered staleness. Nightly `skip_stale_threads` (e.g. p14d) plus the weekly
  full sweep for resurrections.
- slackdump supports custom API limit configuration, and session tokens
  behave like the official client. Raising tier-3 pacing cuts wall time
  linearly until 429s push back. Optional and riskier, off by default.
- Callers can move sync off their critical path entirely (scheduled
  background syncs), which is an operational choice outside this tool but
  worth documenting in the README.
- Future work, not this design. A websocket daemon receiving events in real
  time would eliminate polling, at the cost of significant complexity.

## Compatibility

What existing users can rely on staying the same, and what changes.

**Unchanged.**
- Command surface. `sync`, `export`, `channels`, `config`, and `init` keep
  their names and arguments. `render` is additive.
- Output layout. Dated folders containing `YYYY-MM-DD-channel.md` files.
- Configuration. Same YAML file, and `SLACK_EXPORT_*` env overrides continue
  to work (viper auto-maps the new keys the same way).
- Auth. Same slackdump credential flow.

**Changed by design.**
- Day files gain the Thread continuations section.
- Any file inside the render window can change on a later sync. Today only
  the last export date is ever rewritten. Downstream consumers that treat day
  files as immutable after the following day will miss updates and must
  switch to change detection (fingerprint or mtime). This is the biggest
  contract change and belongs in the README and release notes.

**Regression risks.**
- Base-section rendering moves from `slackdump format text` to our renderer.
  `format text` covers many message shapes (attachments, file placeholders,
  bot messages, edited markers). Guard with golden-file tests comparing our
  output against `format text` on fixture archives, and treat any diff in the
  base section as a bug unless deliberately accepted.
- `export <date>` for dates before the seed date errors instead of exporting.
  Recovering older history requires reseeding the archive from an earlier
  `-time-from`, documented in the error message.
- First sync after upgrade regenerates files in the render window, producing
  a one-time formatting diff for those dates.

## Migration

1. **slackdump v4 upgrade.** Bump the bundled binary to v4.4.1 and update
   `MinSlackdumpVersion` and the version parser (`Slackdump 4.x.y` output
   format needs verification). The v3-era `archive` + `format text` + zip
   pipeline is deleted once the renderer lands.
2. **Seeding.** On first run against an existing output directory, default
   the seed date to the earliest dated folder found (or let the user set
   `seed_date`). The archive then covers everything the renderer will be
   asked for. History older than the seed date stays as the existing static
   files and is never rewritten.
3. **Existing files.** The first render regenerates files within the render
   window. Formatting may differ slightly from `format text` output (mention
   resolution, spacing). Dates older than the window are untouched unless
   `render --full` is run after reseeding.
4. **Downstream pipelines.** No changes required. Consumers that fingerprint
   date folders and re-consolidate changed dates (e.g. work-log
   `sync-slack.sh`) pick up regenerated files automatically. Write-only-on-
   change keeps those diffs honest.

## Edge Cases and Tradeoffs

- **Threads started before the seed date** are not in the database and never
  get continuation tracking. Accepted. The gap is bounded by choosing a good
  seed date.
- **Resurrected threads.** A thread idle past `skip_stale_threads` that gets
  a new reply is caught by the next full sweep, not the nightly run. The
  reply still lands on the day it was posted since rendering happens after
  the sweep. Tighter guarantees mean removing the skip-stale flag at the
  cost of slower syncs.
- **Edits and deletes.** Resume re-fetches within the lookback, so edits to
  recent messages flow into re-rendered files. `-skip-complete-threads` is
  off by default because it trades edit detection for speed.
- **Same-day late replies** (posted after sync ran) are caught by the next
  resume, and the day file regenerates.
- **Archive growth.** Messages only (`-files=false`), SQLite, plus `-dedupe`
  keeps it compact. Years of a personal workspace log should stay in the tens
  of megabytes.
- **Resume is beta upstream.** Keep downstream fingerprint checks as the
  safety net. If resume corrupts an archive, the recovery path is reseed from
  `seed_date`, which is cheap for months-scale history.
- **Entity list merge semantics.** Verify during implementation how resume
  merges a passed entity list with the archive's known entities, and that
  excluded channels already present in the archive are not re-fetched.

## Testing

- Unit tests for the renderer. Day bucketing across timezone boundaries,
  continuation block selection (reply date after thread start date),
  broadcast dedupe, mention resolution, write-only-on-change.
- Golden-file tests comparing renderer output to `slackdump format text`
  output for the same fixture archive, covering attachments, bot messages,
  file placeholders, and edited messages. Base sections should be
  near-identical, with any accepted differences documented in the test.
- Integration tests against a fixture database archive built from recorded
  chunks (slackdump's `chunktest` patterns or a checked-in fixture DB).
- Manual verification against the live workspace. Start a thread, reply the
  next day from another account, confirm the reply appears in the second
  day's file with correct context, and that the first day's file is
  unchanged.

## Implementation Checklist

**Phase 1, slackdump v4 upgrade**
- [ ] Bump bundled slackdump to v4.4.1 in release workflow
- [ ] Update `MinSlackdumpVersion` and version parsing for v4 output
- [ ] Verify auth/credentials flow is unchanged under v4

**Phase 2, archive lifecycle**
- [ ] Config keys (`archive_dir`, `seed_date`, `lookback`,
      `skip_stale_threads`, `full_sweep_interval`)
- [ ] Bootstrap archive on first sync (`slackdump archive -files=false`)
- [ ] Resume invocation with entity list from filtered channel detection
- [ ] Counts-driven scoping (compare `client.counts` latest vs archive
      checkpoints, pass only moved channels)
- [ ] Verify empirically whether thread-only replies move counts `latest`
      or per-channel thread counts, and scope thread refresh if so
- [ ] Full-sweep scheduling and last-sweep tracking
- [ ] Verify entity list merge semantics with excluded channels

**Phase 3, renderer**
- [ ] Import `slackdump/v4/source`, open archive read-only
- [ ] Day bucketing via `GetDateBounds` work-day windows, base section
      rendering to match current format
- [ ] Mention resolution from archive users plus external-user cache
- [ ] Write-only-on-change file output
- [ ] `render` command with `--full`
- [ ] Golden-file tests against `format text` output on fixture archives

**Phase 4, thread continuations**
- [ ] Continuation selection rule (reply local-date == D, thread start < D)
- [ ] Block rendering with `[context]` parent and origin link
- [ ] `thread_broadcast` dedupe
- [ ] Channel files created for continuation-only days

**Phase 5, cleanup**
- [x] Delete the legacy text-format zip pipeline
- [ ] Update `export` semantics (render from DB, reseed error path)
- [ ] README and config.example.yaml updates
