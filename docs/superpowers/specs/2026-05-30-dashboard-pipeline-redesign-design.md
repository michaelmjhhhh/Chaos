# Dashboard & Pipeline Redesign — Design Spec

**Date:** 2026-05-30
**Status:** Draft for review
**Project:** VibeShot (macOS SwiftUI menu-bar app)

## 1. Goal

Redesign VibeShot's main window so that:

1. The Dashboard tab feels **graceful and detailed** — more like a daily edition of a small publication than a metrics screen.
2. A new **Pipeline** tab (replacing History) visualizes screenshots flowing through processing stages as a 4-column kanban.
3. The whole window reads as **one coherent publication**, not as a generic SaaS UI.

The visual direction is **"editorial paper, refined"** — a refinement of the existing warm cream / coral / serif palette executed with magazine-grade typography, asymmetric composition, hairline rules, marginalia, and a paper-grain surface.

## 2. Non-goals

- No new service-layer functionality (no new providers, no queueing logic, no new file operations). Pure UI redesign.
- No changes to `SettingsView` or `MenuBarView` in this work (they may be refreshed later as a separate effort).
- No new third-party dependencies. Pure SwiftUI, system frameworks only.
- No dark mode rework in this pass (current light theme stays the only theme).

## 3. Information architecture

Two tabs total, both sitting under a shared editorial masthead band:

| Tab | Was | Becomes |
|---|---|---|
| Dashboard | Status + 6 metric tiles + 2 dir cards + 3 recent files | Broadsheet composition: headline strip + 2-column body + colophon footer |
| History → **Pipeline** | Inset table of all processed files | 4-column kanban: `Caught · Reading · Setting · Filed`, where Filed *is* the history |

There is no separate History tab. The Filed column owns historical data and gains search, filter chips, and date grouping.

## 4. Shared shell

### 4.1 Masthead band

A 28pt-tall band at the top of every page, identical across both tabs.

Format:

```
VIBESHOT  ·  DAILY EDITION  ·  Sat, 30 May  ·  No. 14
```

- Set in 10pt small-caps serif (via `.uppercase()` + tracking +1.2pt).
- Color: `Theme.ink`.
- Dot separators ( · ) in `Theme.borderLight`.
- The "No. 14" is a **session counter** — incremented and persisted to `@AppStorage("vibeshot.sessionNumber")` on each app launch. Lets the user recognize this specific session.
- A 0.5pt hairline rule (`Theme.rule`) sits below the band.

### 4.2 Window sizing

- Minimum window: **760 × 540** (up from 640 × 440).
- macOS title bar stays empty/transparent; the masthead is the page identity.
- No sidebar; no toolbar controls. All actions live inside the page content.

### 4.3 Tab labels and icons

- `Dashboard` — SF Symbol `newspaper`
- `Pipeline` — SF Symbol `square.stack.3d.up`

### 4.4 Tab transition

A 180ms ease-in-out crossfade between Dashboard and Pipeline content. The masthead does **not** crossfade — it stays put, selling "two spreads of one publication."

## 5. Dashboard composition (Broadsheet)

Outer padding 32pt; gutter between columns 28pt. The page should fit at 760×540 in typical state without scrolling, but the body is `ScrollView`-wrapped for safety.

### 5.1 Headline strip (full width)

A single serif headline whose text is the current status:

| State | Headline | Color |
|---|---|---|
| Idle | *"Quietly Watching."* (italic) | `Theme.warmInk` |
| Watching, no activity | *"Watching for screenshots."* | `Theme.warmInk` |
| Processing | *"Reading a new screenshot…"* with ellipsis animation on the period | `Theme.warmInk` |
| Error | *"Trouble afoot."* | `Theme.coral` |

- Font: `displayHero` (32pt serif, regular, tracked -0.5). Italic states (idle copy) apply `.italic()` to the same font.
- Right side of strip: the Start / Stop pill button (refined — see §9.2).
- Below the headline: a dateline-style sub-headline in 11pt small-caps, e.g. `RUNNING 14M 22S · EASTERN TIME · PROVIDER · SILICONROUTER`.

### 5.2 Two-column body

The body is split 60% / 40% horizontally.

#### 5.2.1 Left column — "Now Playing" hero card

A single tall card occupying the full left column, approximately 340 × 310pt. Background `Theme.surfaceCard`, hairline border (`Theme.rule`, 0.5pt), corner radius `Theme.r10`.

**Idle state:**
- Engraving-style line drawing of a camera shutter (custom Canvas glyph, ~80pt, `Theme.textSoft`) centered in the top half.
- Below: italic serif text *"No captures yet today. The page will fill as you work."* in `Theme.textMuted`.

**Active state:**
- **Top half:** the screenshot thumbnail, rounded to `Theme.r10`, with a paper-grain overlay at 6% to "settle" it into the page. Thumbnail max ~280 × 160.
- **Below thumbnail:**
  - The proposed slug appearing letter-by-letter (mono 14pt, `Theme.ink`) — see §9.3 for typing motion.
  - Original filename in marginalia style to the right (italic, 10pt, `Theme.textSoft`).
- **Bottom:** `StageProgress` strip — three labels (`Analyzing · Renaming · Clipboard`):
  - Active label in `Theme.coral`, with a 1pt underline that draws left-to-right over 240ms when the stage starts.
  - Completed labels in `Theme.ink`, no underline.
  - Upcoming labels in `Theme.textSoft`.
  - Elapsed time underneath in 10pt mono (`Theme.textSoft`).
- **On success:** card holds final state for 3s with a faint `Theme.coral` checkmark in the top-right corner, then dissolves to idle state (240ms crossfade).

#### 5.2.2 Right column — Editorial blocks

Stacked vertically, hairline rules (`EditorialRule`) between each block. ~30pt vertical gap above each rule, 16pt below.

**Block 1 — "Today's Reading" (sparklines)**
- Label: small-caps serif `TODAY'S READING`.
- Three `Sparkline` components stacked, total ~70pt tall:
  - *Fig. 1 — Latency, last 24 captures* (line of last 24 latencies)
  - *Fig. 2 — Throughput, hourly* (bar/line of files-per-hour today)
  - *Fig. 3 — Success rate, session* (running success rate as window of last 20)
- Captions in 9pt serif italic.
- Last value of each line labeled at the right edge in 9pt mono.
- Lines: 1pt `Theme.ink`. Fills: `Theme.canvas` darkened slightly (no fill if the value is constant).

**Block 2 — "Numbers"**
- Label: small-caps `NUMBERS`.
- Three `MetricFigure` components inline: `47 PROCESSED · 45 SUCCESSFUL · 2 ERRORS`. Errors number in `Theme.error` when > 0.
- Below: a 9pt mono caption: `AVG 1.8S · P95 4.2S`.

**Block 3 — "Today's Vocabulary"**
- Label: small-caps `TODAY'S VOCABULARY`.
- A single line of serif italic, e.g. *"terminal, login, settings, error, dashboard."*
- Computed: top 5 nouns by frequency from today's successful slugs. Simple tokenizer (`Models/Tokenizer.swift`) — split on non-alphanumeric, lowercase, length ≥ 3, exclude a small stopword list (`the`, `and`, `for`, `with`, etc.).
- When fewer than 5 unique tokens exist today, show what's available. When zero, hide the block entirely (don't show empty editorial filler).

**Block 4 — "Directories"**
- Label: small-caps `DIRECTORIES`.
- Two rows: `Watch` and `Output`.
- Each row: a hairline left underline (the editorial equivalent of an icon row marker), small-caps label, then path in mono with `~` abbreviation, truncated middle.

### 5.3 Footer colophon

Full-width, just above the bottom edge. Hairline rule above, then a 10pt small-caps line:

```
API · OK    PROVIDER · SILICONROUTER    MODEL · GEMINI-3-FLASH    14M 22S UPTIME
```

- Each segment separated by 4 spaces.
- `API · OK` color: `Theme.success` (or `Theme.error` on FAIL).
- Other segments in `Theme.textSoft`.

## 6. Pipeline composition (Kanban)

Outer padding 24pt; gutter 16pt between columns.

### 6.1 Columns

Four columns, left to right:

| # | Header | Purpose | Width |
|---|---|---|---|
| 1 | `CAUGHT` | Files just landed in the watch dir, being checked by `ScreenshotGuard`. Briefly populated. | ~140pt |
| 2 | `READING` | Currently being analyzed by the vision API. | ~140pt |
| 3 | `SETTING` | Being renamed/moved, clipboard step included. (Typographic term — type was "set" on a press.) | ~140pt |
| 4 | `FILED` | Done. Successes AND errors. | Remaining width, scrollable. |

### 6.2 Column headers

- Small-caps serif 10pt, tracked +1.2.
- A 0.5pt hairline rule below.
- **Active column** (where the current file is right now): the rule is `Theme.coral` instead of `Theme.rule`. This is the only ambient indicator of "where in the press the file is."

### 6.3 Empty state for columns 1–3

A single em-dash (`—`) centered in the column, in `Theme.ink` at 30% opacity. No words.

### 6.4 Card ("clipping") design

Flat block on cream — no shadow. Top has a 0.5pt rule the full width of the card (perforation-like).

- **Layout:** 40 × 40 thumbnail (rounded `Theme.r6`, paper-grain overlay) on the left. To the right: slug in mono 12pt, then original filename below in italic 10pt `Theme.textSoft`.
- **Footer line:** timestamp in small-caps mono (10pt, `Theme.textSoft`).
- **Error cards:** instead of the top rule, a `Theme.coral` 1pt rule on the **left edge**. The error message replaces the slug, set in italic `Theme.ink`.
- **Successful cards:** a subtle ink-colored ✓ glyph in the top-right corner of the card.
- **Hover:** background tone shifts `Theme.surfaceCard` → `Theme.surfaceMuted` over 80ms. No scale, no shadow.
- **Padding:** 10pt all sides.

Cards have no drop shadow at any state. They are clippings on paper, not floating tiles.

### 6.5 Filed column extras

#### 6.5.1 Inline header (above the cards)

- Hair-thin search field on the left, placeholder *"Search filings…"* in serif italic, `Theme.textSoft`. 0.5pt bottom hairline only — no box.
- On the right, three small-caps chips: `ALL · ERRORS · TODAY`.
- Filter active state: chip weight transitions regular → medium (100ms), and a 1.5pt `Theme.coral` underline draws over 160ms. Other chip underlines fade in parallel.

#### 6.5.2 Card grouping under date dividers

- `DateDivider` rule with a small-caps label: `TODAY · 30 MAY`, `YESTERDAY · 29 MAY`, `EARLIER · 28 MAY AND BEFORE`.
- Cards beneath flow without further per-card separators (the perforation rule on each card is enough).

#### 6.5.3 Interactions

- **Single click:** select.
- **Double click / Enter:** open the file (`NSWorkspace.shared.open`).
- **Right click:** existing "Reveal in Finder" context menu preserved.
- **Cmd-R:** reveal selected in Finder.
- **↑ / ↓:** move selection between visible cards.

### 6.6 Reads-when-idle

When the watcher is stopped, columns 1–3 show the em-dash placeholder; Filed shows full history with all interactions available. The page should still read as a full publication, not as "the app is off."

## 7. Motion

Principle: motion should feel like paper — pages turning, leaves settling. No glows, no pulses, no scaling.

### 7.1 Stage transitions on the kanban

- Card glide between columns uses `matchedGeometryEffect` keyed by `RecentFile.id`.
- Spring: `response: 0.45, damping: 0.85`. No overshoot.
- On entering Filed: an 80ms 1pt y-shift downward to "settle," then still.

### 7.2 New card appearance

- Fade-in to Caught: 200ms ease-in, opacity 0 → 1, no scale.

### 7.3 Hero card — slug typing

- Slug appears with a typewriter effect: 28ms per character, slight per-character y-jitter ±0.5pt (seeded so it's deterministic across redraws).
- Cursor: single `|` glyph in `Theme.coral`, blinking once per second (50% duty cycle).

### 7.4 Stage progress underline

- The active label's 1pt underline draws left-to-right over 240ms when the stage starts.
- Completing a stage: underline fades to nothing over 160ms.

### 7.5 Sparklines

- New data point: line extends rightward over 320ms with ease-out.
- No animated fill — fill is static.

### 7.6 Tab crossfade

180ms ease-in-out, content-only (masthead stays).

### 7.7 Start/Stop button

- Label change: instant.
- Background color transition: 240ms.
- Click "stamp": 1pt y-shift down over 80ms, then back.

### 7.8 Idle screens

- No animation.
- Exception: the "…" on `"Reading a new screenshot…"` oscillates `. → .. → …` at 600ms intervals **only while reading**.

### 7.9 Reduce Motion

Honor `accessibilityReduceMotion`:
- `matchedGeometryEffect` disabled — cards snap between columns.
- Typewriter disabled — slug appears whole.
- Sparkline draw-in disabled — line appears instantly.
- All crossfades shortened to 80ms or instant.

## 8. Editorial design system

### 8.1 Typography additions

Added to `Views/Theme.swift`:

```swift
static let displayHero = Font.system(size: 32, weight: .regular, design: .serif)
static let serifItalicLg = Font.system(size: 24, weight: .regular, design: .serif).italic()
static let serifItalicSm = Font.system(size: 11, weight: .regular, design: .serif).italic()
static let smallCapsSm = Font.system(size: 10, weight: .medium)  // pair with .uppercased() + tracking
```

Tracking and uppercasing applied at the call site or via a `.smallCaps()` view modifier helper.

### 8.2 Color additions

```swift
static let warmInk = Color(hex: 0x1F1E1B)      // softer black for serif italics
static let rule = Color(hex: 0xD5CFC4)         // hairline color, used everywhere
static let paperTint = Color(hex: 0x141413).opacity(0.06)  // thumbnail grain overlay
```

`Theme.coral` remains the only accent. No new accent colors are introduced.

### 8.3 Paper grain

A 256 × 256 PNG noise asset (`paper-grain.png`), added to `Assets.xcassets/paper-grain.imageset/` with `@1x`, `@2x`, `@3x`. Drawn at 5% opacity as a `.tiled` background behind `canvas` on every page via `PaperBackground`. Falls back gracefully — if missing, the page is just cream.

Asset can be generated procedurally once (e.g., with a small one-off Python script using Pillow) and committed.

### 8.4 Iconography

- **Tab icons:** SF Symbols (system consistency).
- **In-page decorative icons:** replaced with custom hairline `Canvas` glyphs in `EditorialIcons.swift`:
  - Watch directory: an eye glyph (one ellipse + one inner circle, 0.5pt ink strokes).
  - Output directory: a small "arrow into tray" glyph.
  - Idle hero placeholder: an octagonal aperture (shutter) drawn at 80pt.
- **System icons preserved:** Start/Stop button (`play.fill` / `stop.fill`), context menu items, tab bar, footer status dots.

### 8.5 Spacing scale

Added to `Theme`:

```swift
static let sMicro: CGFloat = 4
static let sSmall: CGFloat = 8
static let sMed: CGFloat = 16
static let sLg: CGFloat = 24
static let sSec: CGFloat = 32
static let sBreak: CGFloat = 48
```

### 8.6 View modifiers

- Existing `.card()` modifier preserved for backwards compatibility (Settings, MenuBar still use it).
- New `.clipping()` modifier — flat, no shadow, hairline top perforation. Used by `PipelineCard`.
- New `.smallCaps()` modifier — uppercases text and applies tracking +1.2pt.
- New `.marginalia(_ side: HorizontalEdge)` modifier — italic + textSoft + small font + edge alignment.

## 9. Keyboard shortcuts

Global within the main window:

- `⌘1` — Dashboard tab
- `⌘2` — Pipeline tab
- `⌘F` — focus Filed search (Pipeline tab only)
- `␣` (Space) — toggle Start/Stop, only when no text field has focus
- `⌘,` — Settings (existing, unchanged)
- `⌘R` — reveal selected Filed card in Finder

## 10. State and data flow

### 10.1 New state on `AppState`

```swift
var sessionNumber: Int               // from SessionMeta, set on init
var sessionStartedAt: Date           // set on init
var vocabularyToday: [String]        // top 5 nouns, recomputed when recentFiles or day changes
var latencyHistory: [Double]         // last 24 latencies for Fig. 1
var hourlyThroughput: [Int]          // bucketed file counts per hour today for Fig. 2
var successRateWindow: [Double]      // running rate over last 20 for Fig. 3
```

### 10.2 New model files

- `Models/SessionMeta.swift` — owns the `@AppStorage("vibeshot.sessionNumber")` integer; provides `nextSessionNumber()` on launch.
- `Models/Tokenizer.swift` — `static func topNouns(from slugs: [String], limit: Int) -> [String]` with a small built-in stopword list.

### 10.3 Recomputation

- `vocabularyToday` and `successRateWindow` recompute whenever `recentFiles` changes (cheap — capped at 50 entries).
- `hourlyThroughput` recomputes on each new file and once per minute via a Timer (to roll bucket boundaries).
- `latencyHistory` is a sliding window of the existing `latencies` array.

### 10.4 No service-layer changes

All new state is derived from data the app already produces. `FileProcessor`, `VisionAPIClient`, `DirectoryWatcher`, etc. are untouched.

## 11. File plan

### 11.1 New files

```
VibeShot/
├── Models/
│   ├── SessionMeta.swift
│   └── Tokenizer.swift
└── Views/
    └── Editorial/
        ├── Masthead.swift
        ├── EditorialRule.swift
        ├── Sparkline.swift
        ├── MetricFigure.swift
        ├── Marginalia.swift
        ├── DropCap.swift
        ├── PaperBackground.swift
        ├── PipelineCard.swift
        ├── DateDivider.swift
        ├── EditorialIcons.swift
        ├── HeroCard.swift
        ├── StageProgress.swift
        ├── PipelineColumn.swift
        └── FiledColumn.swift

VibeShot/Assets.xcassets/
└── paper-grain.imageset/
    ├── Contents.json
    ├── paper-grain.png
    ├── paper-grain@2x.png
    └── paper-grain@3x.png
```

### 11.2 Modified files

| File | Change |
|---|---|
| `Views/ContentView.swift` | Rename History tab → Pipeline; update icons; bump minWidth/minHeight |
| `Views/Theme.swift` | Add new tokens (colors, fonts, spacing, modifiers) |
| `Views/DashboardView.swift` | Rewritten to broadsheet composition |
| `Views/HistoryView.swift` → `Views/PipelineView.swift` | Rename file and rewrite as 4-column kanban |
| `VibeShotApp.swift` | Wire `SessionMeta` increment on launch |
| `AppState.swift` | Add fields per §10.1 |

### 11.3 Unchanged files

- All `Services/*`
- All existing `Models/*` files (date-bucket grouping for Filed is computed in-view from `RecentFile.timestamp`; the model itself is untouched)
- `Views/SettingsView.swift`
- `Views/MenuBarView.swift`

`Views/Theme.swift` is in the modified list (§11.2); existing tokens are preserved, new ones added — no breaking changes.

## 12. Build sequence

Each step independently testable via SwiftUI Previews.

1. **Design system foundation** — extend `Theme`, add `PaperBackground`, `EditorialRule`, `Masthead`, `DropCap`.
2. **Editorial primitives** — `MetricFigure`, `Sparkline`, `Marginalia`, `EditorialIcons`, `DateDivider`.
3. **Hero & stage progress** — `HeroCard`, `StageProgress` driven by mocked state.
4. **Dashboard rewrite** — broadsheet composition assembled from primitives. Visual parity check against the original (no information lost).
5. **Pipeline primitives** — `PipelineCard`, `PipelineColumn`, `FiledColumn`.
6. **Pipeline view** — 4-column board, `matchedGeometryEffect` motion, search + filter chips. Replace `HistoryView`.
7. **Session metadata & derived state** — `SessionMeta`, `Tokenizer`, derived sparkline arrays. Wire into Dashboard.
8. **ContentView wiring** — rename tab, update icons, bump window minimum.
9. **Reduce Motion + keyboard pass** — accessibility + shortcut handlers.
10. **Polish pass** — paper grain asset, hover states, final spacing audit.

## 13. Risk & open questions

- **Custom small-caps:** SwiftUI's font feature for true small-caps is limited; the spec uses `.uppercased()` + tracking which is consistently readable across system fonts but not "true" small caps. Acceptable trade-off; revisit if a chosen serif ships with proper small-cap glyphs.
- **Paper grain at full opacity might create visible repetition.** Mitigation: 256×256 tile at 5% should be invisible-but-felt; if banding appears, regenerate with finer noise.
- **Vocabulary tokenizer false positives:** trivial nouns will leak through (e.g., "screen", "image"). Stopword list is the primary defense; ship a reasonable starting list and accept that this is a low-stakes editorial flourish, not a feature surface.
- **`matchedGeometryEffect` across column boundaries with `ScrollView` inside Filed:** verify smooth animation when the card crosses from a non-scrolling column into a scrollable one. Fallback: snap on entry to Filed with a fade-in instead.

## 14. Acceptance criteria

- Both tabs render under the shared masthead with the same date and session number.
- The Dashboard headline strip reflects watcher state with the correct copy per §5.1.
- The hero card cycles through `Analyzing → Renaming → Clipboard → Success → Idle` with all motion specified in §7.
- All three sparklines render with real data once 2+ data points exist; render quietly with placeholder when no data exists.
- Today's Vocabulary appears only when ≥1 successful slug today.
- The Pipeline view shows 4 columns; active-column hairline is coral; cards transit columns with `matchedGeometryEffect`.
- Filed column groups under date dividers and supports search, filter chips, double-click open, Cmd-R reveal, Reduce Motion fallback, and ↑/↓ keyboard navigation.
- Footer colophon shows `API · {OK|FAIL}` with the correct color.
- Reduce Motion preference disables all animations per §7.9.
- Window minimum: 760 × 540. Resizing remains smooth.
- No regressions: settings, menu bar popover, and watcher logic work identically to before.
