# Batch Image Organizer Design

**Date:** 2026-05-31
**Status:** Approved
**Project:** Chaos macOS screenshot organizer

## Goal

Add a visible Dashboard action for organizing existing screenshots. Users can
select multiple image files from a native macOS file picker, then send them
through the same AI naming and filing pipeline already used by watched and
dropped screenshots.

## Scope

This work includes:

- a Dashboard action labeled `Organize Existing Screenshots`;
- a native macOS open panel that supports selecting multiple image files;
- PNG, JPEG, HEIC, and WebP selection;
- sequential processing through the existing image intake path;
- existing Pipeline progress, history, naming policy, output directory, and
  retry behavior.

This work does not include:

- folder selection or recursive directory scanning;
- parallel API requests;
- a separate batch-management page;
- additional persisted queue state;
- changes to watched screenshot behavior.

## User Flow

1. The Dashboard presents `Organize Existing Screenshots` near the existing
   drag-and-drop intake area.
2. Selecting the action opens an `NSOpenPanel`.
3. The panel permits selecting multiple files and does not permit directories.
4. The panel limits visible selectable files to PNG, JPEG, HEIC, and WebP
   images.
5. Confirmed URLs enter the shared manual image intake path.
6. The app filters the URLs again through `ImageIntake`, preserving their
   selected order.
7. Accepted images process one at a time. Each image is analyzed, renamed, and
   moved according to the existing configured output and naming policy.
8. The Pipeline page continues to display the current in-flight image and
   records each success or failure in Filed history.

Canceling the panel leaves application state unchanged.

## Architecture

### Shared intake filtering

Extend `ImageIntake` with an order-preserving filter for URL arrays. Both
drag-and-drop and the picker-backed action use this helper. This keeps accepted
file rules in one place and avoids separate batch logic for each UI entry
point.

### App state

Keep sequential processing in `AppState`. Rename the current
`processDroppedURLs(_:)` entry point to a UI-neutral manual-intake method so it
can serve both drag-and-drop and picker selection. The method filters accepted
URLs, ignores empty results, and starts one task that awaits each image in
order.

Watched screenshots and retry continue to use their existing code paths.

### Dashboard

Add a secondary action near the Dashboard hero card. The action presents
`NSOpenPanel` with:

- `allowsMultipleSelection = true`;
- `canChooseFiles = true`;
- `canChooseDirectories = false`;
- allowed content types for PNG, JPEG, HEIC, and WebP.

When the user confirms, pass the selected URLs to the shared manual-intake
method. Keep the existing drop destination and route it through that same
method.

## Error Handling

- Canceling the picker performs no work.
- Unsupported URLs are ignored by `ImageIntake`, even if they reach the app
  state method unexpectedly.
- An empty accepted URL list performs no work.
- Individual processing failures retain the existing history record and retry
  behavior. A failed image does not prevent later selected images from being
  processed.

## Testing

Add focused unit coverage for the shared intake filter:

- accepts supported image formats case-insensitively;
- rejects unsupported extensions;
- filters mixed URL arrays while preserving the original selected order.

Run:

```bash
swift test
git diff --check
./build-app.sh
```

Launch the built app and inspect the Dashboard:

- the organizer action is visible near the manual intake area;
- the file picker supports selecting multiple images;
- directories cannot be selected;
- canceling the picker does not trigger processing;
- the existing drag-and-drop path remains wired to shared manual intake.
