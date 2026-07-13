# Late Thread Replies Archive Renderer

This release changes Slack export fetching from per-day archive runs to a
persistent slackdump v4 archive plus an in-process renderer.

## Behavior Changes

* slackdump v4.4.1 or newer is required.
* `sync` maintains a workspace archive at `archive_dir/<workspace>` and renders
  channel/day files for rows written by the latest resume.
* `sync --full` is the explicit bounded sweep mode. It uses a 90-day lookback,
  a 90-day stale-thread bound, archive dedupe, the bundled Slack API limits
  config, and the same written-row renderer.
* Daily and full syncs share an archive lock at `<archiveDir>.lock`; a daily
  sync that collides with an active sweep exits successfully without rendering.
* Late replies to older threads now appear in the reply-day file under
  `## Thread continuations`, with parent context marked by `[context]`.
* Day files touched by later syncs can change as threads receive replies or
  recent messages are edited. Consumers should use mtime or content
  fingerprints instead of assuming those files are immutable.
* `export` renders from the local archive. Dates before `seed_date` fail with
  reseed guidance because `resume` cannot backfill earlier than the archive
  seed.
* `render` regenerates files without network calls; `render --full` rebuilds
  every date from `seed_date` through today.
* `skip_stale_channels` is no longer supported. Stale YAML keys or
  `SLACK_EXPORT_SKIP_STALE_CHANNELS` values are ignored so revived channels are
  not dropped before their new messages sync.
