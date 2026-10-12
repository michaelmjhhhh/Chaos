# Native Go feature-parity audit

Baseline: the [Swift application at `3209622`](https://github.com/michaelmjhhhh/Chaos/tree/3209622034e03d25b3a53b0d4be06d1c245b88d7/Chaos), read alongside its tests before migration.
The replacement uses MyGo native `ui.View` surfaces. It contains no HTML, JavaScript,
WebView, or Swift application layer.

## Required surfaces and interactions

- **Shell:** shared application state, Dashboard/Pipeline/Insights, persistent tray,
  Settings, Help and replayable first-run guide. Preserve Cmd+, settings,
  Cmd+Shift+T connection check, Cmd+? help, Cmd+Q quit, and focus-aware Space start/stop.
- **Dashboard:** local date/status masthead; start/stop; current image and progress
  through analysis, renaming and optional clipboard; success/failure; actionable
  errors; current-day filed/error counts and timing; watched/output folders; API
  state; image drops and a native multiple-image picker.
- **Pipeline:** latest 500 lifetime-persisted attempts; original/generated name and
  error search; All/Errors/Today filters; local-day grouping; thumbnail, time and
  error detail; selection/open/reveal; rename preserving extension; undo to original
  path; failed-image retry and deduplicated retry-all; live queue/batch progress.
- **Insights:** lifetime totals, month/week volume, month-over-month change, success
  rate, estimated time saved (30s per success), activity calendar, current/longest
  streak, peak hour, category and file-type breakdowns, average/p95 duration,
  fastest/slowest images and trailing twelve-week volume.
- **Settings:** seven existing providers and their defaults; protected key input;
  provider descriptions/signup links; model override; custom endpoint; connection
  checks; hosted-trial allowance; native watch/output folder selection; English/
  Chinese filenames; tokenized filename template and live preview; none/day/month
  subfolders; custom prompt toggle/editor/default reset; image clipboard;
  start-on-launch; notifications; system/light/dark appearance; reveal config.
- **Welcome:** introduction, optional provider/key connection, folder choice, final
  start; Back/Continue/Skip; persistent completion; replay through Help.
- **Help:** searchable FAQ with expandable answers and website link.
- **Tray:** watcher state/control, latest five attempts, reveal file, output folder,
  Dashboard/Settings/Help, provider/model and Quit.
- **Native integration:** file and directory dialogs, Finder reveal, image opening,
  image pasteboard, success sound, opt-in notifications, device identity, desktop
  appearance, VoiceOver labels/announcements and reduced-motion preferences.

## Persistence contracts

- Bundle identifier remains `com.chaos.app`. Import/preserve UserDefaults keys
  `autoStart`, `hasCompletedOnboarding`, and `chaos.sessionNumber`.
- Keep `~/Library/Application Support/chaos/config.json`: `provider`, `api_key`,
  `model`, `base_url`, `watch_dir`, `output_dir`, `copy_to_clipboard`, `language`,
  `filename_template`, `subfolder_rule`, `notify_on_complete`, `appearance`,
  `use_custom_prompt`, `custom_prompt`. Missing fields resolve to baseline defaults.
- Import `vibe-shot/config.json` only when Chaos config does not exist. Keep source.
- Reuse `history.sqlite` and `processed_images`: `id`, `originalName`, `newName`,
  `path`, `sourcePath`, `timestamp`, `duration`, `isError`, `errorMessage`, `ext`.
  Keep full history and existing migration metadata. The UI working set alone is
  capped at 500. SQLite timestamps are UTC text; displayed calendar math is local.
- Legacy `history.json` uses Swift Date's 2001 epoch and synthesized result enums.
  Import transactionally and archive `.migrated` only after successful decoding.
- Hosted device identity remains SHA256 of `chaos.device.v1` plus exact hardware
  UUID string; preserve fallback Keychain service `com.chaos.app.device`, account
  `device-seed`. A new UI must not reset existing trial identity.

## Defects identified in the old implementation

1. Dashboard “TODAY” counters were session-only and never reset at midnight.
2. Ellipsis/cursor timers leaked across appearance; typing timers could overlap.
3. Current processing displayed a previous success's thumbnail and duration.
4. Updating watched-directory config did not rebind an active watcher.
5. Onboarding health checks could render stale results after credentials changed.
6. Help incorrectly instructed successful records to use failure-only Retry.
7. Tray Help relied on a main-window sheet which could be closed.
8. Config-save errors were swallowed; history failures replaced API-health state.
9. Stop cleared the UI without coordinating pending jobs; watchers/retries/batches
   could interleave and overwrite one current-file display.
10. Manual intake trusted extensions, accepting invalid paths and duplicate images;
    a second batch during another run was silently discarded.
11. An unmodified Space app command risked interfering with text editing.
12. Failed analytics queries silently presented an empty archive.

These are behavior corrections, not intended compatibility constraints.

## Implementation boundaries

`internal/core` owns domain, vision, naming, persistence, analytics and file handling.
`internal/app` owns lifecycle, queue, snapshots and platform effects. `internal/desktop`
consumes immutable snapshots on the UI thread and issues controller commands. Native
OS glue is isolated behind callbacks; image decoding is off the UI thread. Go tests
exercise real MyGo native elements using `ui.NewTester`, including keyboard input,
filtering, menus, settings and screenshots. App packaging and macOS integration are
validated separately from the headless view tests.

## Review scenarios

1. Open existing config/SQLite/legacy JSON; retain history, settings and trial ID.
2. Watch a new screenshot; manually select/drop each supported format; mixed batch.
3. Fail a request, repair provider, retry one/all; reject missing source gracefully.
4. Rename with collision then undo; never overwrite either existing file.
5. Submit while busy, stop while processing, restart and change watched directory.
6. Change provider during health request; display only current results.
7. Relaunch and close/reopen the main window while tray remains available.
8. Resize narrow/wide; light/dark/system; keyboard-only navigation and VoiceOver.
9. Advance through local midnight/DST; verify Today and Insights against durable rows.
