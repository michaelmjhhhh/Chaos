# Chaos Functional Workflow Expansion Design

**Date:** 2026-05-30
**Status:** Approved by delegated product direction
**Project:** Chaos macOS screenshot organizer

## Goal

Strengthen Chaos as a daily screenshot workflow tool before investing further in
release mechanics. Add durable local history, explicit retry, drag-and-drop
intake for existing images, filename templates, and simple automatic subfolder
organization while preserving the quiet editorial interface.

## Scope

This work has three sequential functional slices:

1. **Reliability:** persist the most recent 500 processing attempts and allow a
   failed attempt to be retried explicitly.
2. **Broader input:** allow the user to drop existing PNG, JPEG, HEIC, or WebP
   images onto the Dashboard for processing through the same AI naming flow.
3. **Organization:** allow a compact filename template and optional date-based
   subfolders.

Release notarization, launch-at-login, onboarding, arbitrary custom prompts,
and semantic AI-generated folder routing remain separate work.

## Product Decisions

### Durable history

- Store history locally as JSON at
  `~/Library/Application Support/chaos/history.json`.
- Keep the newest 500 attempts, including successes and failures.
- Load history when the app starts.
- Save history after every completed attempt.
- Preserve the existing in-memory session metrics as session metrics. Loading
  durable history must not inflate current-session throughput or latency.
- Treat malformed or missing history files as empty history so the watcher can
  still start.

### Retry

- Failed cards gain a small `Retry` action in their context menu.
- Retrying processes the original source image again and appends a new history
  record. The failed record remains as an audit trail.
- Failures retain a source path so retry can locate the original image.
- If the source file no longer exists, retry records a new failure explaining
  that the source image is unavailable.
- Retry uses the current provider, model, naming policy, and output settings.

### Drag-and-drop intake

- The Dashboard hero area accepts dropped image file URLs.
- Supported extensions are PNG, JPEG, JPG, HEIC, and WebP.
- Dropped files bypass the macOS screenshot filename guard but use the same
  processing pipeline, history, metrics, and error behavior.
- Directory-watcher screenshots continue to use the existing screenshot guard.
- The source image is moved into the configured output directory on success,
  matching the existing screenshot behavior.
- The idle hero card adds one subtle line of copy: `Drop an image to file it.`
  No separate import screen or persistent drop-zone chrome is added.

### Filename templates

- Add a settings text field with the default template:

  ```text
  {slug}_{time}
  ```

- Supported tokens are `{slug}`, `{time}`, and `{date}`.
- `{time}` formats as `HHmmss`; `{date}` formats as `yyyy-MM-dd`.
- The generated extension is appended by Chaos and is not part of the template.
- Empty templates fall back to `{slug}_{time}`.
- Unsupported tokens are left as literal text, then sanitized as part of the
  generated filename. The UI shows the supported tokens beside the field.
- Collision suffixes remain supported.

### Simple subfolder rules

- Add a settings picker named `Organize Into` with:
  - `No Subfolders`
  - `By Day` using `yyyy-MM-dd`
  - `By Month` using `yyyy-MM`
- Date folders are computed from the processing time and created inside the
  configured output directory.
- Keep this deliberately narrow. Semantic folders and user-authored rule
  expressions are later product work.

## Architecture

### History store

Create `HistoryStore` as a small filesystem service. `RecentFile` becomes
`Codable` with a stable persisted `id`, a source path, and a codable result
representation. `AppState` loads records through the store and funnels all
history inserts through one bounded persistence helper.

### Reusable intake

Refactor `AppState.handleNewFile` into a guarded watcher entry point and a
shared `processInput(url:)` path. Retry and drag-and-drop call the shared path
directly. Add `ImageIntake` for pure extension validation so the accepted file
types are testable without SwiftUI.

### Naming policy

Create `NamingPolicy` as a pure model that renders templates and resolves the
optional dated output directory. Extend `AppConfig` with optional
`filename_template` and `subfolder_rule` keys so older configs continue to
decode. Pass the policy into `FileProcessor`, then into `FileRenamer`.

### UI

Keep UI edits contained:

- `DashboardView` receives dropped file URLs and forwards accepted images to
  `AppState`.
- `HeroCard` adds quiet idle-state helper copy.
- `FiledColumn` exposes a retry closure and adds the action only for failures.
- `SettingsView` adds a compact `Organization` section.

## Error Handling

- History load failure produces an empty history.
- History save failure does not abort image processing; `AppState` exposes a
  short history persistence error in the existing status channel.
- Dropped unsupported files are ignored without creating history noise.
- Retry of a missing source records a failed attempt with the missing path.
- Invalid filename templates fall back to the default only when empty. Rendered
  names still pass through the filename sanitizer.

## Testing

Add focused XCTest coverage for:

- history round-trip, malformed history fallback, and newest-500 trimming;
- image extension acceptance;
- filename token rendering, empty-template fallback, collision suffixes, and
  day/month output directory resolution;
- `RecentFile` codable round-trip including errors and source paths;
- existing processing behavior after the naming-policy integration.

Run `swift test`, `git diff --check`, and `./build-app.sh`. Launch the built app
for a final visual smoke check of the editorial UI.

