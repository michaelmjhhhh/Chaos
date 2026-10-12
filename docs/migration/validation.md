# Migration validation

Baseline: `3209622034e03d25b3a53b0d4be06d1c245b88d7` (`origin/main` when the
migration branch was created). The original **94 Swift tests passed** before the
implementation was replaced. The new application is pinned to **MyGo v0.4.6** and
Go 1.27.2; its framework review and native examples are linked in
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

## Dependency security check

On 2026-10-13, official `govulncheck` **v1.9.0** flagged ten standard-library
advisories through the Go 1.27.1 call graph:
[GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617),
[6613](https://pkg.go.dev/vuln/GO-2026-6613),
[6612](https://pkg.go.dev/vuln/GO-2026-6612),
[6611](https://pkg.go.dev/vuln/GO-2026-6611),
[6610](https://pkg.go.dev/vuln/GO-2026-6610),
[6609](https://pkg.go.dev/vuln/GO-2026-6609),
[6608](https://pkg.go.dev/vuln/GO-2026-6608),
[6607](https://pkg.go.dev/vuln/GO-2026-6607),
[6605](https://pkg.go.dev/vuln/GO-2026-6605), and
[6603](https://pkg.go.dev/vuln/GO-2026-6603).
It also reported three without a reachable call:
[GO-2026-6604](https://pkg.go.dev/vuln/GO-2026-6604),
[6600](https://pkg.go.dev/vuln/GO-2026-6600), and
[6599](https://pkg.go.dev/vuln/GO-2026-6599).
Static reachability does not establish exploitability of every reported path.

All thirteen are fixed by [Go 1.27.2, released October 8](https://go.dev/doc/devel/release#go1.27.2),
which is now required by `go.mod`. Both default and production-tag rescans used the actual
downloaded `go1.27.2 darwin/arm64` toolchain and checked six application packages,
15 modules, and the standard library. Both exited successfully with **no known
vulnerabilities found**. MyGo remains pinned to v0.4.6; third-party dependency
versions were unchanged. `go mod tidy` completed successfully.

```sh
go version
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -show=verbose ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -tags=mygo_noinspector -show=verbose ./...
```

This is a check against the current Go vulnerability database, not a guarantee
against undisclosed issues or an audit of native OS libraries.

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
