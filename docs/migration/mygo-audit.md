# MyGo native migration audit

Audited 2026-10-13 against **MyGo v0.4.6**, commit
[`bc111b6d2cca221716ad99ddea1580449ad8c18e`](https://github.com/egoist/mygo/tree/bc111b6d2cca221716ad99ddea1580449ad8c18e).
The [official website](https://mygo.egoist.dev/) links to the same repository and
advertises this version. Context7 resolution was attempted but returned unrelated
projects named MyGo; this assessment therefore uses the official checked-out
source, its documentation and runnable examples. Conclusions below are source
findings, not a claim that every platform has been manually exercised.

## Architecture decision

Use `WindowOptions.Content: ui.View(app.view)`, with ordinary Go services called
directly from actions. A native window has no `Page()` and creates no webview.
There is no IPC, TypeScript binding generation, web frontend or JavaScript runtime
in this architecture. Rendering uses Metal on macOS, Direct3D 11 on Windows and
OpenGL on Linux, with CPU rendering available. Linux native apps require GTK 3
but not WebKitGTK; Windows native apps need no WebView2 runtime.

Sources: [native architecture](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/ui/README.md),
[native windows](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/ui/windows.md),
[renderer](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/ui/rendering.md).

Keep model, persistence, HTTP, image preparation and processing logic independent
of MyGo. Desktop integration owns windows and OS actions. This lets backend tests
run without starting the native event loop, and lets views use deterministic
sample state and actions in `ui.NewTester`.

## Threading and lifecycle rules

- Call `App.Run()` on the main goroutine. Create windows in `App.WhenReady`.
- Public MyGo operations marshal to the native main thread. Event callbacks,
  menu actions, global shortcuts and the view run there: no network, disk scan,
  image decode, channel wait or blocking dialog in those callbacks.
- Change UI state from a worker with `win.Update(func() { ... })`. **Update is
  asynchronous**; its return does not mean the state has changed. `Invalidate`
  schedules a frame but does not synchronize application state.
- `ui.Element` belongs to one build pass. Never retain it in app state or a
  goroutine. Use `ui.Handle` for persistent focus/control identity; keep list,
  selection, router and model state on the app object.
- Build passes may repeat in a frame. Use response queries or callbacks for
  effects, never unconditional work in `view`. Prefer `OnClick` when deleting
  or reordering the collection currently being built. Use `c.Key(item.ID)` for
  controls whose items can move; duplicate sibling keys alias widget state.
- Cancel workers and unsubscribe listeners at shutdown. Closing the last window
  quits unless `OnWindowAllClosed` has a listener; a tray app must explicitly
  implement its close/hide/quit policy.

```go
win := mygo.NewWindow(mygo.WindowOptions{Content: ui.View(app.view)})
go func() {
    result, err := service.Load()
    win.Update(func() { app.result, app.err = result, err })
}()
```

The implementation of `Window.Update` still invokes its closure after a window
has closed, although repaint becomes a no-op; application disposal guards and
worker cancellation remain necessary.

Sources: [view contracts](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/ui/views.md),
[actual dispatch and repaint](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/content.go#L144-L185),
[application lifecycle](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/app.md).

## Chaos feature mapping

| Existing capability | Native Go path | Important behavior |
| --- | --- | --- |
| Dashboard, pipeline, history | `Column`, `Row`, `Scroll`, `List`/`Table` | Lists virtualize rows; give them a size and stable keys. |
| Insights, heatmap, activity plot | `ui/chart`, or `Element.Draw` and `Painter` | All Go; supply accessible labels for drawn data. |
| Settings and provider keys | `TextInput`, `.Password()`, `Select`, `Switch`, `Checkbox`, `Field` | Configure disabled/password state before response queries. |
| System/light/dark appearance | `Theme.SetSource`, `c.Theme`, optionally `c.SetTheme` | Choose saved theme before `App.Run` to avoid initial flash. |
| Image preview | `DecodeBitmap` outside view, then `Image(...).Fit(Contain)` | Cache bitmaps; supported formats do not include HEIC/TIFF. |
| File/folder picker | `Dialog.Open(OpenDialogOptions{Directory: true})` | Blocking; use a goroutine and marshal result back. |
| Clipboard image intake | `Clipboard.ReadImage()` | Returns PNG or nil; requires app ready. |
| Clipboard file intake | `Clipboard.ReadFiles()` | Local absolute paths, independent of image representation. |
| File drops | `Window.OnFileDrop` or element `DroppedFiles()` | Copy semantics; framework never deletes source files. |
| Reveal/open a result | `Shell.ShowItemInFolder`, `Shell.OpenPath` | Handle OS failures, retain original path in history. |
| Notifications | `NewNotification(...).Show`, `.OnClick` | macOS needs an app bundle; Show may wait for permission. |
| Global clipboard shortcut | `GlobalShortcut.Register("CmdOrCtrl+...", fn)` | Callback is on main thread; registration may fail. |
| Start at login | `App.SetOpenAtLogin`, `OpenAtLogin`, `WasOpenedAtLogin` | macOS 13+ needs bundle; setting can fail. |
| Tray/menu bar | `NewTray`, `OnClick`, `OnRightClick`, `SetMenu` | Primary click with an attached macOS menu opens that menu. |
| Completion sound | `audio` Go package; platform sound compatibility separately | Decode/play WAV or synthesized samples; output failure is observable. |
| Persistent settings/history | Application Go packages | MyGo supplies paths, not a database or migration policy. |
| Stable device identity/key storage | Application platform adapter | MyGo does not provide Keychain or hardware identity APIs. |

Sources: [desktop APIs](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/native.md),
[login source](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/login.go),
[clipboard and transfer](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/data-transfer.md),
[images](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/ui/image.md),
[charts](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/ui/chart.md),
[audio](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/audio.md).

## Examples worth copying

- [counter-native](https://github.com/egoist/mygo/tree/bc111b6d2cca221716ad99ddea1580449ad8c18e/examples/counter-native):
  minimal Go-only view and headless interaction test.
- [menubar-native](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/examples/menubar-native/main.go):
  closest lifecycle match to Chaos. Accessory activation policy, hidden native
  panel, tray anchoring, outside-click hide, guarded close, and explicit quit.
  On macOS it temporarily attaches a menu for right-click, then removes it so
  primary click can keep toggling the panel. Linux AppIndicator only supports
  a menu, so the menu includes an Open action.
- [gallery](https://github.com/egoist/mygo/tree/bc111b6d2cca221716ad99ddea1580449ad8c18e/examples/gallery):
  navigation, styles, controls, virtualized data and chart patterns, with tests.
- [clipboard](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/examples/clipboard/main.go)
  and [drag-drop](https://github.com/egoist/mygo/tree/bc111b6d2cca221716ad99ddea1580449ad8c18e/examples/drag-drop):
  native representations and provider lifetimes.

Despite its name, `examples/native` demonstrates desktop integrations through
an HTML page. It is not a Go-only frontend template.

## Build, packaging and verification

The pinned module requires **Go 1.27.1**. Native projects use `mygo.json` and a
Go tool dependency; no npm package is required. `mygo build` defaults to
`CGO_ENABLED=0`, links app identity/version, and packages macOS app/DMG,
Windows executable/installer and Linux tarball/Debian package. Platform signing
tools still matter: macOS signing/DMG creation must run on macOS; NSIS is needed
for Windows installers. Preserve the released bundle identifier to retain OS
permission/login associations, and explicitly migrate existing user data paths.

Sources: [module](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/go.mod),
[JSON config](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/configuration.md#mygojson),
[distribution](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/distribution.md).

`ui.NewTester(view, width, height)` builds and renders without a window, supports
click/type/keyboard/scroll/context menus, and exposes visible text, focus,
clipboard, opened URLs and an image. `SetDark`, `SetSize`, `SetScale` and
`SetPreferences` cover environment variants. It detects duplicate keys and stale
elements. Run real packaged-app checks separately for tray, dialogs, clipboard,
global shortcuts, notifications, login items and GPU presentation: headless tests
do not prove OS integration. `mygo vet` checks build-lifetime misuse; normal Go
tests and race checks cover backend concurrency. `CapturePage` also captures
native content for visual review. `MYGO_GPU=0` selects CPU rendering, and
`MYGO_FRAME_STATS=1` exposes slow frames for investigation.

Sources: [tester](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/docs/ui/testing.md),
[gallery test examples](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/examples/gallery/main_test.go),
[framework testing contract](https://github.com/egoist/mygo/blob/bc111b6d2cca221716ad99ddea1580449ad8c18e/AGENTS.md).

## Maturity and remaining platform risks

The [repository metadata](https://api.github.com/repos/egoist/mygo) reports
creation on 2026-09-26; [v0.4.6](https://github.com/egoist/mygo/releases/tag/v0.4.6)
was released on 2026-10-12. This is a very young, pre-1.0 dependency. Pinning the
release and keeping services independent makes upgrades and regression review
tractable; successful compilation alone is insufficient evidence of parity.

- [Issue #246](https://github.com/egoist/mygo/issues/246) reports a Linux native
  crash after GPU fallback with multiple windows. It remained open at audit
  time. `MYGO_GPU=0` is the reporter's workaround, not proof the bug is fixed.
- [Issue #68](https://github.com/egoist/mygo/issues/68) requests nonactivating
  macOS panels. The audited API does not expose that option; a native tray panel
  can activate the app. Test focus-sensitive clipboard flows explicitly.
- [Roadmap #105](https://github.com/egoist/mygo/issues/105) records gaps in deep
  text/collection accessibility, locale-aware built-ins, RTL layout and system
  text services. Some unchecked transfer/motion items already exist in v0.4.6,
  so source takes precedence over the checklist. Standard widget labels and
  keyboard behavior are useful but are not a claim of full AppKit parity.
- Image decoding needs application handling for HEIC/TIFF and resource limits.
  MyGo's clipboard reads normalize native image representations to PNG, while
  `ui.DecodeBitmap` does not decode every format accepted by the old macOS app.
- Notification permission denial, unavailable tray services and shortcut
  conflicts need visible recoverable errors. Notification/login tests must use
  a packaged app. Wayland global shortcuts require a supporting portal and an
  installed desktop entry; the desktop may ask for permission or remap keys.
- Pure Go calls into native libraries through `purego`; absence of cgo does not
  remove OS ABI, GPU-driver or desktop-library risks. Native libraries still
  require real platform QA.

## Hosted backend is independent of the desktop framework

The hosted trial service uses `net/http` and Redis REST, without MyGo or a
JavaScript SDK. The [official Vercel Go runtime documentation](https://vercel.com/docs/functions/runtimes/go)
and Context7's Vercel documentation confirm a root `go.mod`, Go framework preset,
custom build command and `PORT` listener. Vercel downloads the selected Go
toolchain. Its Go runtime is marked beta; deployment compatibility is documented
but requires a preview build before production rollout. No production deployment
is part of this migration.
