# Validation

This document describes checks for the current native Go application. Versions
come from `go.mod` and `mygo.json`; the framework review is in
[mygo-audit.md](mygo-audit.md). Dated migration results, including the retired
hosted service, are preserved in the
[2026-10-13 validation record](validation-2026-10-13.md). They do not establish
that a later revision passed the same checks.

## Current automated checks

Run from the repository root:

```sh
gofmt -l cmd internal
go vet ./...
go test -race -count=1 ./...
CGO_ENABLED=0 go build -tags mygo_noinspector ./...
```

`gofmt -l` should produce no output. CI runs formatting, vet, race tests and the
production build on macOS, Linux and Windows. A separate job cross-compiles
production binaries. Tests use temporary directories, generated image fixtures
and local fake providers. Do not point tests at a user's screenshot directory.
The current application has no Redis or hosted-service test dependency.

Additional checks when changing native views or platform code:

```sh
go run github.com/egoist/mygo/cmd/mygo vet .
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -tags mygo_noinspector ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags mygo_noinspector ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags mygo_noinspector ./...
```

## Behavior covered by the suites

- **Compatibility:** legacy config and defaults, provider credential isolation,
  retired-provider migration, Swift-epoch history import, SQLite/GRDB schema,
  preserved corrupt input and backups, unlimited stored history with a 500-row
  UI working set, and local-calendar analytics.
- **Images and files:** watcher baseline/stability and undo suppression, PNG and
  JPEG orientation, real WebP/HEIC fixtures, alpha compositing and vision resize,
  naming safety, collisions, source replacement, no-clobber publication, macOS
  metadata/ACL preservation, and source retention on failure.
- **Provider requests:** payload content, response parsing and recovery errors
  using local fake services. These tests do not prove a live provider's model
  availability or quality.
- **Controller:** filing → rename → relaunch → undo, duplicate intake, cancellation,
  watcher reconciliation, stale health responses, settings persistence and error
  ownership, detached snapshots, crash recovery, and clipboard failures after a
  durable filing.
- **Native views:** interactions, search/filtering, shortcuts, rename/undo,
  settings, onboarding, help, tray actions, render smoke coverage and selected
  layout bounds. Rendering an image alone is not a visual regression assertion.

To retain headless bitmap renders for manual review:

```sh
CHAOS_UI_ARTIFACTS=/tmp/chaos-ui-artifacts go test -count=1 ./internal/desktop
```

## Packaged macOS checks

Packaging requires macOS. These commands replace the local `.build` application
and DMG outputs:

```sh
./package-dmg.sh
hdiutil verify .build/Chaos.dmg
open .build/Chaos.app --args --data-dir /tmp/chaos-preview
```

Configure the isolated profile with a local/mock provider and synthetic images.
Manually check first-run setup; watched and manual filing; rename and undo while
watching; persisted Pipeline/Insights after reopening; connection checks and
appearance; close/reopen through the tray; native dialogs, clipboard and
notification delivery; keyboard navigation and accessibility. Native window and
OS integration require this separate check even when headless tests pass.

For a ZIP preview, `./scripts/package-preview-release.sh` derives its version
from `mygo.json` and architecture from `go env GOARCH`. An optional version
argument must match `mygo.json`; set the metadata first when preparing a new
preview version. The script validates metadata before building.

## Dependency checks

The dated vulnerability scan is recorded in
[the migration evidence](validation-2026-10-13.md#dependency-security-check).
Re-run against the current source and vulnerability database when evaluating
dependency changes:

```sh
go version
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -show=verbose ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -tags=mygo_noinspector -show=verbose ./...
```

A passing scan is not a guarantee against undisclosed issues or an audit of
native OS libraries. Record the revision, platform and actual commands with new
validation evidence rather than carrying forward an old test count.

## Platform limits

- macOS/Apple Silicon is the primary desktop. Linux/Windows builds and headless
  tests do not establish GPU, tray or accessibility parity on every desktop.
- HEIC conversion uses macOS `/usr/bin/sips`; other platforms report an
  unsupported-format error. PNG/JPEG/WebP decoding is portable Go.
- File contents and journal entries are flushed before publication. Windows
  skips unsupported directory-handle flushing; directory-entry power-loss
  durability depends on its OS/filesystem.
- Local API keys retain owner-only config permissions. They are not stored in an
  encrypted credential vault.
- The hosted service, device identity collection and Redis tests are retired.
  `vercel.json` disables automatic Git deployments.
- The DMG is ad-hoc signed and not notarized. Release tags must match
  `mygo.json`; packaged metadata supplies the application version.
