# Validation and migration evidence

Historical migration baseline: `3209622034e03d25b3a53b0d4be06d1c245b88d7`.
The original **94 Swift tests passed** before that implementation was replaced.
The current application is pinned to **MyGo v0.4.6** and
Go 1.27.2; its framework review and native examples are linked in
[mygo-audit.md](mygo-audit.md).

## Automated regression coverage

The current Go suites cover the desktop application. The retired hosted service
and its Redis tests are no longer part of the repository or the validation gates.
The suites exercise these behaviors:

- **Compatibility:** legacy snake_case config and defaults; per-provider key
  isolation and unknown-provider rejection; Swift-epoch JSON import, out-of-range
  timestamps and archive failures; existing SQLite/GRDB
  schema and IDs; full history with a 500-row UI working set; local-date analytics,
  streaks, percentiles, categories and time estimates; one-time legacy preferences
  import through a fake command runner.
- **Images and files:** screenshot guards and watcher baseline/stability; PNG,
  JPEG EXIF orientation, WebP, macOS HEIC; bounded decoding, alpha compositing and
  vision resize; Unicode/unsafe model output; filename templates and collisions;
  rename/undo; Finder attributes, creation date and ACLs; source replacement during
  preparation/API work; exclusive publication and source preservation on failure;
  filesystem errors retain their details instead of being reported as network outages.
- **Controller:** complete filing → rename → relaunch → undo; duplicate intake;
  cancellation; rapid Stop/Start and directory changes during startup/rebind;
  retry of failed and missing files; undo under a running watcher with aliased folder
  paths and subsequent filesystem identity reuse; stale health responses;
  appearance versus credential changes;
  immutable snapshots; corrupt config preservation and independent persistence
  error recovery; crash journal recovery and
  conflicting source/same-location safety; recovery rejects symlinks, directory
  aliases, shared file identities and replaced entries while retaining images and
  pending journals; clipboard failure remains durable success.
- **Native views:** all main tabs in light/dark appearance; search and filters;
  keyboard focus and shortcuts; selection/open; rename/revert context actions;
  provider, prompt, template and appearance settings; onboarding; FAQ; error
  recovery and warning dismissal; tray actions and control bounds at constrained
  widths; bounded thumbnail work, same-path replacement, stale completion and
  shutdown cancellation. PNG artifact export also runs independently.
- **Vision transport:** actual model, text and image payload contents; parsed
  names; redirect rejection, bounded responses and in-flight cancellation using
  local HTTP servers. These checks do not contact a real provider.

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

CI runs the Go suites on macOS, Linux and Windows. Cross-compilation is a separate
job. No Redis installation or external provider credentials are needed.

Native bitmap renders can be reproduced with:

```sh
CHAOS_UI_ARTIFACTS=/tmp/chaos-ui-artifacts go test -count=1 ./internal/desktop
```

## Cleanup validation — 2026-10-15

Against audit baseline `9cb71b49b3b03840511cc2b83372dbe0327f826c`, local checks
passed for all four Go packages with `-race -count=1`. `go vet`, MyGo native vet,
Staticcheck v0.8.2, formatting, and the cgo-disabled production build also passed.
Production cross-builds passed for Linux/amd64, Windows/amd64 and Darwin/amd64.
The ad-hoc signed Apple Silicon app and DMG were rebuilt; `hdiutil verify` passed
and the packaged executable reports `Chaos 1.1.0 (MyGo native)`.
The packaged app also ran with a fresh isolated profile: switching to Dark visibly
updated the native window and persisted the setting, the native Help menu replayed
the welcome guide, completing it persisted onboarding, and Cmd+Q drained and exited
successfully. This smoke check did not use personal data or call a provider.

Thumbnail regressions use controlled completion and cancellation signals. Render
smoke tests assert control bounds at 1000×800 and 600×720; standalone and shuffled
exports into fresh directories passed. These are not pixel-perfect layout checks:
the narrow Dashboard still has excess vertical spacing before Today.
Production dead-code analysis with x/tools v0.52.0 reports no unreachable functions;
the analytics refactor's test-only entry points were removed as well as the old
file-operation wrappers.

The history benchmark retains full-history statistics while scanning only fields
needed by the accumulator. A 100,000-row run changed from about 101 ms / 131.6 MB
allocated to 76 ms / 29.5 MB. These are local measurements, not a latency guarantee;
reproduce with `go test -run '^$' -bench BenchmarkStoreInsights -benchmem ./internal/core`.
The tests cover more than 500 records, DST boundaries, streaks and percentiles.

The preview ZIP script was executed in an isolated fixture with controlled
build/sign/archive commands. A mismatched explicit version failed before the
build, and both omitted and matching versions used `mygo.json`.

## Historical dependency security check — 2026-10-13

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

## Historical packaged macOS checks — 2026-10-13

The ad-hoc signed, cgo-disabled Apple Silicon `.app` was launched with an isolated
`--data-dir`, synthetic image fixtures, and a local mock vision HTTP server. No
production screenshots or settings were used.

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
- The DMG is ad-hoc signed, not notarized. Release tags must match `mygo.json`;
  MyGo's packaged metadata is the single source of the application version. The
  preview ZIP script derives its version from the same file and rejects a
  conflicting explicit version before building.
