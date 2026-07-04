# Late Thread Replies Archive Renderer

This release changes Slack export fetching from per-day archive runs to a
persistent slackdump v4 archive plus an in-process renderer.

## Behavior Changes

* slackdump v4.4.1 or newer is required.
* `sync` maintains a workspace archive at `archive_dir/<workspace>` and renders
  recent day files from that archive.
* Late replies to older threads now appear in the reply-day file under
  `## Thread continuations`, with parent context marked by `[context]`.
* Day files inside the configured `lookback` window can change on later syncs
  as threads receive replies or recent messages are edited. Consumers should
  use mtime or content fingerprints instead of assuming those files are
  immutable.
* `export` renders from the local archive. Dates before `seed_date` fail with
  reseed guidance because `resume` cannot backfill earlier than the archive
  seed.
* `render` regenerates files without network calls; `render --full` rebuilds
  every date from `seed_date` through today.
