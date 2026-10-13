# Repository guide

Chaos is a native Go screenshot organizer. MyGo v0.4.6 renders the desktop with
`ui.View`; there is no WebView, TypeScript frontend, Swift target, or cgo requirement
for production builds. macOS 15+ is the primary supported desktop. The application requires Go 1.27.2,
including the October 2026 standard-library security fixes.

## Commands

```sh
go test ./...                         # core, controller, native views
go test -race -count=1 ./...           # concurrency and integration checks
go vet ./...
gofmt -w cmd internal
make build                           # desktop executable, cgo disabled
make dev                             # pinned MyGo development runner
./build-app.sh                       # sign .build/Chaos.app on macOS
./package-dmg.sh                     # .build/Chaos.dmg
open .build/Chaos.app --args --data-dir /tmp/chaos-preview
```

`--data-dir` isolates config, history, and preferences, and avoids
reading production UserDefaults. Give the preview a local/mock
provider before processing fixtures. Never use a user's screenshot directories
for automated tests.

CI runs formatting, vet, race tests and production builds on macOS, Linux and
Windows, plus cross-compilation. Native macOS packaging is checked separately.
Tagging `v*` builds the DMG; it does not notarize the app.

## Ownership and architecture

- `internal/core`: provider/config resolution, image decoding/preparation, vision
  requests, naming, watching, no-clobber file moves, SQLite and analytics.
- `internal/app`: controller, serialized queue, watch lifecycle, immutable
  snapshots, config/preference saves and durable file-operation journal.
- `internal/desktop`: MyGo views for Dashboard, Pipeline, Insights, Settings,
  Help, onboarding and tray. Views consume snapshots and call controller methods.
- `cmd/chaos`: native windows, tray, menus, dialogs, pasteboard, notifications and
  platform effects. Filesystem/image/network work stays off the UI thread.

All MyGo view state and handles belong to the UI thread. Marshal asynchronous
updates through `win.Update`; never retain a `ui.Element` beyond its build pass.
Use `ui.Handle` for persistent view references. Dialogs run in goroutines. Headless
view tests use MyGo's real `ui.NewTester`; native OS integrations still need a
packaged-app check. Consult the pinned framework audit before upgrading MyGo.

The controller owns worker lifetimes and cancellation. It waits for them before
closing SQLite. Watched/manual intake, retry and history mutations share one
queue. Watcher reconciliation serializes Start/Stop/rebind intent. Health
responses carry freshness revisions. Snapshots deep-copy maps
and slices before publication; UI callbacks reject out-of-order revisions.

## Persistence and file safety

Keep bundle ID `com.chaos.app` and macOS data at
`~/Library/Application Support/chaos/`. Existing `config.json` snake_case fields,
`processed_images` camelCase SQLite columns and GRDB migration marker are compatibility
contracts. Keep full history; only the UI working set is capped at 500. Store UTC
timestamps and calculate calendar statistics in the local timezone. Legacy JSON
uses Swift's 2001 date epoch; import transactionally and archive only after success.
Import legacy UserDefaults once into `preferences.json`.

Before publishing a file move, write a durable journal intent. Never overwrite
an existing destination or remove a source that changed during processing. Bind
analysis to the original file identity and content hash. Recovery checks content
before completing history or unlinking a remaining source. Undo suppresses its
restored path in the active watcher before publication. Clipboard/notification
failures are warnings after a durable success, never failed filing records.

Per-provider API keys must not leak to another provider or through HTTP redirects.
Do not log request credentials/image payloads. The retired hosted provider is
removed on config load/save; never carry its credentials or model to another provider.

## Review references

- [Framework and upstream audit](docs/migration/mygo-audit.md)
- [Feature parity and compatibility contracts](docs/migration/feature-parity.md)
- [Backend decisions](docs/migration/backend-audit.md)
- [Validation and remaining platform limits](docs/migration/validation.md)

`docs/superpowers/` records historical Swift designs, not current architecture.
