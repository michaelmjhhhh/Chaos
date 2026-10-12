# Migration validation

Baseline: `3209622034e03d25b3a53b0d4be06d1c245b88d7` (`origin/main` when the
migration branch was created). The original **94 Swift tests passed** before the
implementation was replaced. The new application is pinned to **MyGo v0.4.6** and
Go 1.27.1; its framework review and native examples are linked in
[mygo-audit.md](mygo-audit.md).

## Automated regression coverage

Final local gates passed on 2026-10-13: **74 top-level Go tests** (plus nested
cases) under the race detector, including real Redis; `go vet`; MyGo native
`vet`; cgo-disabled native builds and Linux/Windows/macOS Intel cross-builds;
signed app packaging; and `hdiutil verify` for the 13 MB DMG. The packaged
`--version` reports `Chaos 1.1.0 (MyGo native)`.

The Go suites exercise application behavior, not only compilation:

- **Compatibility:** legacy snake_case config and defaults; per-provider key
  isolation; Swift-epoch JSON import and archive failures; existing SQLite/GRDB
  schema and IDs; full history with a 500-row UI working set; local-date analytics,
  streaks, percentiles, categories and time estimates; exact device hash and
  Keychain fallback semantics.
- **Images and files:** screenshot guards and watcher baseline/stability; PNG,
  JPEG EXIF orientation, WebP, macOS HEIC; bounded decoding, alpha compositing and
  vision resize; Unicode/unsafe model output; filename templates and collisions;
  rename/undo; Finder attributes, creation date and ACLs; source replacement during
  preparation/API work; exclusive publication and source preservation on failure.
- **Controller:** complete filing → rename → relaunch → undo; duplicate intake;
  cancellation; rapid Stop/Start; undo under a running watcher with aliased folder
  paths; stale health/usage responses; appearance versus credential changes;
  immutable snapshots; corrupt config preservation; crash journal recovery and
  conflicting source/same-location safety; clipboard failure remains durable success.
- **Native views:** all main tabs in light/dark appearance; search and filters;
  keyboard focus and shortcuts; selection/open; rename/revert context actions;
  provider, prompt, template and appearance settings; onboarding; FAQ; error
  recovery; tray actions/layout; constrained widths and reduced-motion preferences.
- **Hosted API:** authentication, method/body validation, model pinning, bounded
  upstream responses, error translation, administrative reset and origin headers.
  Actual local Redis executes the production Lua for concurrent device/global
  limits, UTC-month boundaries, rollback/idempotency and reset generations. A
  global budget of one permits only one upstream call even when repeated timeout,
  truncation or oversized-response failures refund the device allowance.

Reproduce the principal checks:

```sh
go test -race -count=1 ./...
go vet ./...
go run github.com/egoist/mygo/cmd/mygo vet .
CGO_ENABLED=0 go build -tags mygo_noinspector ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags mygo_noinspector ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags mygo_noinspector ./...
./package-dmg.sh
hdiutil verify .build/Chaos.dmg
```

Install Redis locally or set `REDIS_SERVER_BIN=/absolute/path/to/redis-server` to
include Lua integration tests. Tests start an isolated process on a loopback port;
they never connect to production Redis. CI installs Redis for the Linux job and
runs the Go suites on macOS, Linux and Windows. Cross-compilation is a separate job.

Native bitmap renders can be reproduced with:

```sh
CHAOS_UI_ARTIFACTS=/tmp/chaos-ui-artifacts go test -count=1 ./internal/desktop
```

## Packaged macOS checks

The ad-hoc signed, cgo-disabled Apple Silicon `.app` was launched with an isolated
`--data-dir`, synthetic image fixtures, and a local mock vision HTTP server. No
production screenshots, settings, Keychain identity or hosted allowance were used.

Observed in the real native app:

1. Complete the first-run guide and start watching the isolated directory.
2. Create a new screenshot fixture; observe analysis, filing, history and counts.
3. Rename through the native context menu, then undo while watching remains active.
   The image is restored and does not immediately re-enter processing.
4. Use Cmd+O and the native image picker to file the restored fixture manually.
5. Inspect persisted Pipeline and Insights, including heatmap, categories and timing.
6. Check the configured connection and switch light/dark appearance.
7. Close the main window without terminating the process; reactivate it and quit.
8. Relaunch using the same profile and confirm the saved history is retained.
9. File and undo a new fixture with an explicit deny-write ACL; inspect the
   filesystem to confirm that both the filed and restored image retain that ACL.

Headless renders of Dashboard, Pipeline, Insights, Settings, Help, welcome and the
370×540 tray panel were also visually reviewed. Native accessibility inspection
exposed the expected control labels, secure key field and history actions.

## Explicit limits and deployment notes

- Live desktop QA was on macOS/Apple Silicon. Linux and Windows build and test
  jobs do not establish equivalent GPU, tray or accessibility behavior on every
  desktop environment. MyGo's upstream Linux second-window/GPU issue and other
  framework limits are documented in the audit. `MYGO_GPU=0` is a diagnostic option.
- The real menu-bar popover and OS notification-permission/delivery UI were not
  exhaustively automated. Tray view actions/layout and effect error handling are
  covered by tests; notification delivery still depends on OS permission. A full
  VoiceOver listening pass was not performed. No new OS permission was granted.
- HEIC uses macOS `/usr/bin/sips`; on other systems Chaos returns an actionable
  unsupported-format error. PNG/JPEG/WebP decoding is portable Go.
- File contents and journal files are flushed before publication. Windows skips
  unsupported directory-handle flushing, so directory-entry power-loss durability
  is controlled by the OS/filesystem. macOS security ACLs are preserved through
  public native APIs without cgo. Unsupported nonempty ACL transfer fails safely.
- The existing local config continues to contain API keys; it is written with
  owner-only permissions, not migrated into an encrypted credential vault.
- The hosted Go service was exercised locally, including actual Redis, but **was
  not deployed**. A Vercel rollout must change the project root from `hosted/` to
  the repository root and preserve existing secrets/Redis data. See
  [hosted deployment instructions](../../hosted/README.md).
- The DMG is ad-hoc signed, not notarized. Release tags must match `mygo.json`;
  MyGo's packaged metadata is the single source of the application version.
