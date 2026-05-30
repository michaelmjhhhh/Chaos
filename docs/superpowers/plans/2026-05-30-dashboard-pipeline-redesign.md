# Dashboard & Pipeline Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild VibeShot's main window as an editorial-paper publication: a broadsheet Dashboard and a 4-column Pipeline kanban that replaces the History tab.

**Architecture:** Pure SwiftUI/AppKit UI rework. New `Views/Editorial/` directory holds reusable design primitives. Two new model files (`SessionMeta`, `Tokenizer`). `AppState` gains a few derived fields. No service-layer changes. New XCTest target added for pure-logic tests; visual components verified via SwiftUI Previews + `swift build` compile checks.

**Tech Stack:** Swift 6.0, SwiftUI, AppKit (for `NSWorkspace`), Swift Package Manager. macOS 15+ minimum. No third-party dependencies.

**Spec:** [docs/superpowers/specs/2026-05-30-dashboard-pipeline-redesign-design.md](../specs/2026-05-30-dashboard-pipeline-redesign-design.md)

---

## Conventions for this plan

- **TDD for logic:** Tokenizer and SessionMeta have XCTests. Write the failing test first, run it red, implement, run it green, commit.
- **TDD for visual components:** SwiftUI views are verified by:
  1. A `#Preview` block in the file (exercises the component with realistic data).
  2. `swift build` succeeding — Swift's strong type system catches most regressions.
  3. The final manual verification step at the end of the plan (build the .app and run it).
  We cannot write meaningful pixel assertions without adding a snapshot framework (out of scope), so the discipline is: write the preview FIRST as the failing case (file doesn't compile yet), then implement.
- **Commit cadence:** every task ends with a commit. Use Conventional Commits style.
- **File paths:** all paths in this plan are relative to repo root.

---

## Phase 0 — Test infrastructure

### Task 0.1: Add an XCTest target to Package.swift

**Files:**
- Modify: `Package.swift`
- Create: `Tests/VibeShotTests/SmokeTests.swift`

- [ ] **Step 1: Update Package.swift to add the test target**

Replace the contents of `Package.swift` with:

```swift
// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "VibeShot",
    platforms: [.macOS(.v15)],
    targets: [
        .executableTarget(
            name: "VibeShot",
            path: "VibeShot",
            exclude: ["Info.plist"],
            resources: [
                .process("Resources"),
            ],
            swiftSettings: [
                .swiftLanguageMode(.v5),
            ]
        ),
        .testTarget(
            name: "VibeShotTests",
            dependencies: ["VibeShot"],
            path: "Tests/VibeShotTests",
            swiftSettings: [
                .swiftLanguageMode(.v5),
            ]
        ),
    ]
)
```

- [ ] **Step 2: Write the smoke test**

Create `Tests/VibeShotTests/SmokeTests.swift`:

```swift
import XCTest
@testable import VibeShot

final class SmokeTests: XCTestCase {
    func testTrueIsTrue() {
        XCTAssertTrue(true)
    }
}
```

- [ ] **Step 3: Run the smoke test**

Run: `swift test --filter SmokeTests`
Expected: 1 test passes (`testTrueIsTrue`).

- [ ] **Step 4: Commit**

```bash
git add Package.swift Tests/VibeShotTests/SmokeTests.swift
git commit -m "test: add XCTest target with smoke test"
```

---

### Task 0.2: Create the Resources directory

The `Package.swift` declares `.process("Resources")` but the directory doesn't exist. Create it with a placeholder so the build is robust.

**Files:**
- Create: `VibeShot/Resources/.gitkeep`

- [ ] **Step 1: Create the directory and placeholder**

```bash
mkdir -p VibeShot/Resources
touch VibeShot/Resources/.gitkeep
```

- [ ] **Step 2: Verify the build still works**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Resources/.gitkeep
git commit -m "chore: add Resources directory placeholder"
```

---

## Phase 1 — Design system foundation

### Task 1.1: Extend Theme with editorial tokens

**Files:**
- Modify: `VibeShot/Views/Theme.swift`

- [ ] **Step 1: Add new color, font, and spacing tokens**

Open `VibeShot/Views/Theme.swift` and add the following inside the `enum Theme` (after the existing `// MARK: - Shadows` block):

```swift
    // MARK: - Editorial additions

    // Softer black for serif italics; takes the edge off pure ink.
    static let warmInk = Color(hex: 0x1F1E1B)

    // Single hairline color used across rules, dividers, card perforations.
    static let rule = Color(hex: 0xD5CFC4)

    // Subtle paper-tint overlay applied to thumbnails so they sit in the page.
    static let paperTint = Color.black.opacity(0.06)

    // MARK: - Editorial type

    static let displayHero = Font.system(size: 32, weight: .regular, design: .serif)
    static let serifItalicLg = Font.system(size: 24, weight: .regular, design: .serif).italic()
    static let serifItalicSm = Font.system(size: 11, weight: .regular, design: .serif).italic()
    static let smallCapsSm = Font.system(size: 10, weight: .medium)

    // MARK: - Editorial spacing

    static let sMicro: CGFloat = 4
    static let sSmall: CGFloat = 8
    static let sMed: CGFloat = 16
    static let sLg: CGFloat = 24
    static let sSec: CGFloat = 32
    static let sBreak: CGFloat = 48
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Theme.swift
git commit -m "feat(theme): add editorial color, type, and spacing tokens"
```

---

### Task 1.2: Add editorial view modifiers to Theme.swift

**Files:**
- Modify: `VibeShot/Views/Theme.swift`

- [ ] **Step 1: Add SmallCaps, Clipping, and Marginalia modifiers**

In `VibeShot/Views/Theme.swift`, append after the existing `SectionHeader` modifier and before the `extension View`:

```swift
struct SmallCaps: ViewModifier {
    func body(content: Content) -> some View {
        content
            .font(Theme.smallCapsSm)
            .tracking(1.2)
            .textCase(.uppercase)
    }
}

struct ClippingCard: ViewModifier {
    var padding: CGFloat = 10

    func body(content: Content) -> some View {
        VStack(spacing: 0) {
            Rectangle()
                .fill(Theme.rule)
                .frame(height: 0.5)
            content
                .padding(padding)
        }
        .background(Theme.surfaceCard)
    }
}

struct Marginalia: ViewModifier {
    func body(content: Content) -> some View {
        content
            .font(Theme.serifItalicSm)
            .foregroundStyle(Theme.textSoft)
    }
}
```

Then update the `extension View` block to add the three new modifiers:

```swift
extension View {
    func card(padding: CGFloat = 16) -> some View {
        modifier(CardModifier(padding: padding))
    }
    func sectionHead() -> some View {
        modifier(SectionHeader())
    }
    func smallCaps() -> some View {
        modifier(SmallCaps())
    }
    func clipping(padding: CGFloat = 10) -> some View {
        modifier(ClippingCard(padding: padding))
    }
    func marginalia() -> some View {
        modifier(Marginalia())
    }
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Theme.swift
git commit -m "feat(theme): add smallCaps, clipping, marginalia view modifiers"
```

---

### Task 1.3: Create PaperBackground component

**Files:**
- Create: `VibeShot/Views/Editorial/PaperBackground.swift`

- [ ] **Step 1: Create the Editorial directory**

```bash
mkdir -p VibeShot/Views/Editorial
```

- [ ] **Step 2: Write PaperBackground.swift**

```swift
import SwiftUI

/// A canvas-tinted background with a procedural noise overlay,
/// approximating the feel of warm paper. Used behind every page.
struct PaperBackground: View {
    var grainOpacity: Double = 0.05

    var body: some View {
        ZStack {
            Theme.canvas
            Canvas { context, size in
                let cellSize: CGFloat = 2
                let columns = Int(size.width / cellSize) + 1
                let rows = Int(size.height / cellSize) + 1
                var rng = SeededRandom(seed: 0xA1B2C3)

                for x in 0..<columns {
                    for y in 0..<rows {
                        let n = rng.nextDouble()
                        guard n > 0.6 else { continue }
                        let opacity = (n - 0.6) * 0.5
                        let rect = CGRect(
                            x: CGFloat(x) * cellSize,
                            y: CGFloat(y) * cellSize,
                            width: cellSize,
                            height: cellSize
                        )
                        context.fill(
                            Path(rect),
                            with: .color(Theme.ink.opacity(opacity * grainOpacity * 8))
                        )
                    }
                }
            }
            .allowsHitTesting(false)
        }
        .ignoresSafeArea()
    }
}

/// Deterministic LCG so the grain pattern is stable across redraws.
private struct SeededRandom {
    private var state: UInt64

    init(seed: UInt64) {
        self.state = seed
    }

    mutating func nextDouble() -> Double {
        state = state &* 6364136223846793005 &+ 1442695040888963407
        return Double(state >> 11) / Double(UInt64(1) << 53)
    }
}

#Preview {
    PaperBackground()
        .frame(width: 400, height: 300)
}
```

- [ ] **Step 3: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 4: Commit**

```bash
git add VibeShot/Views/Editorial/PaperBackground.swift
git commit -m "feat(editorial): add PaperBackground with procedural grain"
```

---

### Task 1.4: Create EditorialRule component

**Files:**
- Create: `VibeShot/Views/Editorial/EditorialRule.swift`

- [ ] **Step 1: Write EditorialRule.swift**

```swift
import SwiftUI

/// A 0.5pt hairline rule, optionally interrupted by a centered typographic ornament.
struct EditorialRule: View {
    enum Ornament: String {
        case none = ""
        case section = "§"
        case asterism = "⁂"
        case dot = "·"
    }

    var ornament: Ornament = .none
    var color: Color = Theme.rule

    var body: some View {
        HStack(spacing: Theme.sSmall) {
            Rectangle()
                .fill(color)
                .frame(height: 0.5)
            if ornament != .none {
                Text(ornament.rawValue)
                    .font(.system(size: 11, design: .serif))
                    .foregroundStyle(color)
                Rectangle()
                    .fill(color)
                    .frame(height: 0.5)
            }
        }
    }
}

#Preview {
    VStack(spacing: 24) {
        EditorialRule()
        EditorialRule(ornament: .dot)
        EditorialRule(ornament: .section)
        EditorialRule(ornament: .asterism)
    }
    .padding(40)
    .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/EditorialRule.swift
git commit -m "feat(editorial): add EditorialRule with optional ornament"
```

---

## Phase 2 — Editorial primitives

### Task 2.1: Create SessionMeta model + tests (needed by Masthead)

**Files:**
- Create: `VibeShot/Models/SessionMeta.swift`
- Create: `Tests/VibeShotTests/SessionMetaTests.swift`

- [ ] **Step 1: Write the failing test**

Create `Tests/VibeShotTests/SessionMetaTests.swift`:

```swift
import XCTest
@testable import VibeShot

final class SessionMetaTests: XCTestCase {
    private let defaultsKey = "vibeshot.test.sessionNumber"

    override func setUp() {
        super.setUp()
        UserDefaults.standard.removeObject(forKey: defaultsKey)
    }

    override func tearDown() {
        UserDefaults.standard.removeObject(forKey: defaultsKey)
        super.tearDown()
    }

    func testFirstLaunchReturnsOne() {
        let meta = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertEqual(meta.sessionNumber, 1)
    }

    func testSubsequentLaunchesIncrement() {
        _ = SessionMeta(defaultsKey: defaultsKey)
        let second = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertEqual(second.sessionNumber, 2)

        let third = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertEqual(third.sessionNumber, 3)
    }

    func testStartedAtIsRecent() {
        let meta = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertLessThan(abs(meta.startedAt.timeIntervalSinceNow), 2.0)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `swift test --filter SessionMetaTests`
Expected: FAIL with "Cannot find 'SessionMeta' in scope".

- [ ] **Step 3: Implement SessionMeta**

Create `VibeShot/Models/SessionMeta.swift`:

```swift
import Foundation

/// Tracks how many times the app has been launched (incremented at init)
/// and when this session started. The session number appears in the
/// masthead so the user can recognize this specific session.
struct SessionMeta {
    let sessionNumber: Int
    let startedAt: Date

    init(defaultsKey: String = "vibeshot.sessionNumber",
         defaults: UserDefaults = .standard,
         now: Date = Date()) {
        let previous = defaults.integer(forKey: defaultsKey)
        let next = previous + 1
        defaults.set(next, forKey: defaultsKey)
        self.sessionNumber = next
        self.startedAt = now
    }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `swift test --filter SessionMetaTests`
Expected: all 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add VibeShot/Models/SessionMeta.swift Tests/VibeShotTests/SessionMetaTests.swift
git commit -m "feat(model): add SessionMeta with persistent session counter"
```

---

### Task 2.2: Create Masthead component

**Files:**
- Create: `VibeShot/Views/Editorial/Masthead.swift`

- [ ] **Step 1: Write Masthead.swift**

```swift
import SwiftUI

/// The shared editorial masthead band that sits at the top of every page.
/// Format: VIBESHOT  ·  DAILY EDITION  ·  Sat, 30 May  ·  No. 14
struct Masthead: View {
    let sessionNumber: Int
    let date: Date

    private var dateText: String {
        let f = DateFormatter()
        f.dateFormat = "EEE, d MMM"
        return f.string(from: date)
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                segment("VIBESHOT")
                separator
                segment("DAILY EDITION")
                separator
                segment(dateText.uppercased())
                separator
                segment("NO. \(sessionNumber)")
                Spacer()
            }
            .padding(.horizontal, 24)
            .padding(.vertical, 8)
            .frame(height: 28)

            EditorialRule()
        }
    }

    @ViewBuilder
    private func segment(_ text: String) -> some View {
        Text(text)
            .font(Theme.smallCapsSm)
            .tracking(1.2)
            .foregroundStyle(Theme.ink)
    }

    @ViewBuilder
    private var separator: some View {
        Text(" · ")
            .font(Theme.smallCapsSm)
            .foregroundStyle(Theme.borderLight)
            .padding(.horizontal, 8)
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 0) {
        Masthead(sessionNumber: 14, date: Date())
        Spacer()
    }
    .frame(width: 760, height: 540)
    .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/Masthead.swift
git commit -m "feat(editorial): add Masthead band"
```

---

### Task 2.3: Create EditorialIcons (Canvas glyphs)

**Files:**
- Create: `VibeShot/Views/Editorial/EditorialIcons.swift`

- [ ] **Step 1: Write EditorialIcons.swift**

```swift
import SwiftUI

/// Hairline custom glyphs drawn via Canvas. Used as decorative in-page
/// markers in place of SF Symbols where editorial tone matters.
enum EditorialIcon {
    /// Open eye — used as a watch-directory marker.
    struct Eye: View {
        var size: CGFloat = 14
        var color: Color = Theme.textSoft

        var body: some View {
            Canvas { ctx, canvasSize in
                let w = canvasSize.width, h = canvasSize.height
                var outer = Path()
                outer.move(to: CGPoint(x: 0, y: h / 2))
                outer.addQuadCurve(to: CGPoint(x: w, y: h / 2), control: CGPoint(x: w / 2, y: 0))
                outer.addQuadCurve(to: CGPoint(x: 0, y: h / 2), control: CGPoint(x: w / 2, y: h))
                ctx.stroke(outer, with: .color(color), lineWidth: 0.5)

                let pupilRadius: CGFloat = h * 0.18
                let pupil = Path(ellipseIn: CGRect(
                    x: w / 2 - pupilRadius,
                    y: h / 2 - pupilRadius,
                    width: pupilRadius * 2,
                    height: pupilRadius * 2
                ))
                ctx.fill(pupil, with: .color(color))
            }
            .frame(width: size * 1.4, height: size * 0.8)
        }
    }

    /// Arrow descending into a tray — used as an output-directory marker.
    struct TrayArrow: View {
        var size: CGFloat = 14
        var color: Color = Theme.textSoft

        var body: some View {
            Canvas { ctx, canvasSize in
                let w = canvasSize.width, h = canvasSize.height
                var arrow = Path()
                arrow.move(to: CGPoint(x: w / 2, y: 0))
                arrow.addLine(to: CGPoint(x: w / 2, y: h * 0.65))
                arrow.move(to: CGPoint(x: w * 0.3, y: h * 0.45))
                arrow.addLine(to: CGPoint(x: w / 2, y: h * 0.65))
                arrow.addLine(to: CGPoint(x: w * 0.7, y: h * 0.45))
                ctx.stroke(arrow, with: .color(color), lineWidth: 0.5)

                var tray = Path()
                tray.move(to: CGPoint(x: 0, y: h * 0.8))
                tray.addLine(to: CGPoint(x: w, y: h * 0.8))
                ctx.stroke(tray, with: .color(color), lineWidth: 0.5)
            }
            .frame(width: size, height: size)
        }
    }

    /// Octagonal camera aperture — used as the idle hero placeholder.
    struct Shutter: View {
        var size: CGFloat = 80
        var color: Color = Theme.textSoft

        var body: some View {
            Canvas { ctx, canvasSize in
                let w = canvasSize.width, h = canvasSize.height
                let cx = w / 2, cy = h / 2
                let radius = min(w, h) / 2 - 2
                let sides = 8

                var outer = Path()
                for i in 0..<sides {
                    let angle = (Double(i) / Double(sides)) * 2 * .pi - .pi / 2
                    let pt = CGPoint(
                        x: cx + radius * CGFloat(cos(angle)),
                        y: cy + radius * CGFloat(sin(angle))
                    )
                    if i == 0 { outer.move(to: pt) } else { outer.addLine(to: pt) }
                }
                outer.closeSubpath()
                ctx.stroke(outer, with: .color(color), lineWidth: 0.5)

                for i in 0..<sides {
                    let angle = (Double(i) / Double(sides)) * 2 * .pi - .pi / 2
                    let outerPt = CGPoint(
                        x: cx + radius * CGFloat(cos(angle)),
                        y: cy + radius * CGFloat(sin(angle))
                    )
                    let inner = CGPoint(x: cx, y: cy)
                    var blade = Path()
                    blade.move(to: outerPt)
                    blade.addLine(to: inner)
                    ctx.stroke(blade, with: .color(color.opacity(0.35)), lineWidth: 0.5)
                }
            }
            .frame(width: size, height: size)
        }
    }
}

#Preview {
    HStack(spacing: 40) {
        EditorialIcon.Eye(size: 18)
        EditorialIcon.TrayArrow(size: 18)
        EditorialIcon.Shutter(size: 80)
    }
    .padding(40)
    .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/EditorialIcons.swift
git commit -m "feat(editorial): add Eye, TrayArrow, and Shutter glyphs"
```

---

### Task 2.4: Create DropCap component

**Files:**
- Create: `VibeShot/Views/Editorial/DropCap.swift`

- [ ] **Step 1: Write DropCap.swift**

```swift
import SwiftUI

/// Renders the first character of a string at 2.5x the body size,
/// inline with the rest of the text. Used for section headers
/// that should read like editorial leads.
struct DropCap: View {
    let text: String
    var baseFont: Font = Theme.body
    var capSize: CGFloat = 32

    private var firstChar: String {
        guard let c = text.first else { return "" }
        return String(c)
    }

    private var rest: String {
        guard !text.isEmpty else { return "" }
        return String(text.dropFirst())
    }

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 2) {
            Text(firstChar)
                .font(.system(size: capSize, weight: .regular, design: .serif))
                .foregroundStyle(Theme.ink)
                .baselineOffset(-2)
            Text(rest)
                .font(baseFont)
                .foregroundStyle(Theme.ink)
        }
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 20) {
        DropCap(text: "TODAY'S READING")
        DropCap(text: "NUMBERS")
        DropCap(text: "DIRECTORIES")
    }
    .padding(40)
    .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/DropCap.swift
git commit -m "feat(editorial): add DropCap component"
```

---

### Task 2.5: Create MetricFigure component

**Files:**
- Create: `VibeShot/Views/Editorial/MetricFigure.swift`

- [ ] **Step 1: Write MetricFigure.swift**

```swift
import SwiftUI

/// A serif number paired with a small-caps caption beneath it.
/// E.g.   47
///        PROCESSED
struct MetricFigure: View {
    let value: String
    let label: String
    var accent: Color? = nil

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            Text(value)
                .font(.system(size: 22, weight: .regular, design: .serif))
                .foregroundStyle(accent ?? Theme.ink)
                .monospacedDigit()
                .tracking(-0.3)
            Text(label)
                .smallCaps()
                .foregroundStyle(Theme.textSoft)
        }
    }
}

#Preview {
    HStack(spacing: 32) {
        MetricFigure(value: "47", label: "Processed")
        MetricFigure(value: "45", label: "Successful")
        MetricFigure(value: "2", label: "Errors", accent: Theme.error)
    }
    .padding(40)
    .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/MetricFigure.swift
git commit -m "feat(editorial): add MetricFigure"
```

---

### Task 2.6: Create Sparkline component

**Files:**
- Create: `VibeShot/Views/Editorial/Sparkline.swift`

- [ ] **Step 1: Write Sparkline.swift**

```swift
import SwiftUI

/// A small line chart with a faint fill, a serif italic caption,
/// and a tiny last-value label on the right.
struct Sparkline: View {
    let values: [Double]
    let caption: String
    var lastValueText: String? = nil
    var height: CGFloat = 28

    private var minValue: Double { values.min() ?? 0 }
    private var maxValue: Double { values.max() ?? 1 }
    private var range: Double {
        let r = maxValue - minValue
        return r > 0 ? r : 1
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            HStack(alignment: .firstTextBaseline) {
                Text(caption)
                    .font(Theme.serifItalicSm)
                    .foregroundStyle(Theme.textSoft)
                Spacer(minLength: Theme.sMed)
                if let last = lastValueText {
                    Text(last)
                        .font(Theme.codeSm)
                        .foregroundStyle(Theme.textMuted)
                }
            }

            chart
                .frame(height: height)
        }
    }

    @ViewBuilder
    private var chart: some View {
        Canvas { ctx, size in
            guard values.count >= 2 else {
                drawFlat(ctx: ctx, size: size)
                return
            }

            let stepX = size.width / CGFloat(values.count - 1)
            let pointFor: (Int) -> CGPoint = { i in
                let normalized = (values[i] - minValue) / range
                let x = CGFloat(i) * stepX
                let y = size.height - CGFloat(normalized) * size.height
                return CGPoint(x: x, y: y)
            }

            var fill = Path()
            fill.move(to: CGPoint(x: 0, y: size.height))
            for i in 0..<values.count {
                fill.addLine(to: pointFor(i))
            }
            fill.addLine(to: CGPoint(x: size.width, y: size.height))
            fill.closeSubpath()
            ctx.fill(fill, with: .color(Theme.ink.opacity(0.05)))

            var line = Path()
            line.move(to: pointFor(0))
            for i in 1..<values.count {
                line.addLine(to: pointFor(i))
            }
            ctx.stroke(line, with: .color(Theme.ink), lineWidth: 1)
        }
    }

    private func drawFlat(ctx: GraphicsContext, size: CGSize) {
        var p = Path()
        p.move(to: CGPoint(x: 0, y: size.height / 2))
        p.addLine(to: CGPoint(x: size.width, y: size.height / 2))
        var dashed = ctx
        dashed.stroke(p, with: .color(Theme.textSoft.opacity(0.4)), style: StrokeStyle(lineWidth: 0.5, dash: [2, 3]))
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 16) {
        Sparkline(values: [1.2, 1.8, 1.4, 2.1, 2.6, 1.9, 1.7, 2.2, 2.8, 1.5],
                  caption: "Fig. 1 — Latency, last 10 captures",
                  lastValueText: "1.5s")
        Sparkline(values: [3, 4, 5, 7, 6, 9, 11, 8, 6, 5, 4],
                  caption: "Fig. 2 — Throughput, hourly",
                  lastValueText: "4")
        Sparkline(values: [], caption: "Fig. 3 — No data yet")
    }
    .padding(40)
    .background(Theme.canvas)
    .frame(width: 380)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/Sparkline.swift
git commit -m "feat(editorial): add Sparkline component"
```

---

### Task 2.7: Create DateDivider component

**Files:**
- Create: `VibeShot/Views/Editorial/DateDivider.swift`

- [ ] **Step 1: Write DateDivider.swift**

```swift
import SwiftUI

/// A hairline rule with a small-caps date label on the leading edge.
/// Used to separate Filed-column cards into temporal sections.
struct DateDivider: View {
    let label: String

    var body: some View {
        HStack(spacing: Theme.sSmall) {
            Text(label)
                .smallCaps()
                .foregroundStyle(Theme.textMuted)
            Rectangle()
                .fill(Theme.rule)
                .frame(height: 0.5)
        }
        .padding(.vertical, Theme.sSmall)
    }
}

#Preview {
    VStack(spacing: 8) {
        DateDivider(label: "TODAY · 30 MAY")
        DateDivider(label: "YESTERDAY · 29 MAY")
        DateDivider(label: "EARLIER · 28 MAY AND BEFORE")
    }
    .padding(40)
    .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/DateDivider.swift
git commit -m "feat(editorial): add DateDivider component"
```

---

## Phase 3 — Data layer

### Task 3.1: Implement Tokenizer with tests

**Files:**
- Create: `Tests/VibeShotTests/TokenizerTests.swift`
- Create: `VibeShot/Models/Tokenizer.swift`

- [ ] **Step 1: Write the failing tests**

Create `Tests/VibeShotTests/TokenizerTests.swift`:

```swift
import XCTest
@testable import VibeShot

final class TokenizerTests: XCTestCase {
    func testExtractsTopNounsByFrequency() {
        let slugs = [
            "terminal-git-log",
            "terminal-vim-config",
            "settings-dialog",
            "login-screen",
            "login-error"
        ]
        let top = Tokenizer.topNouns(from: slugs, limit: 3)
        XCTAssertEqual(top.first, "terminal")
        XCTAssertEqual(top.dropFirst().first, "login")
        XCTAssertTrue(top.contains("settings") || top.contains("screen") || top.contains("config") || top.contains("dialog") || top.contains("error"))
        XCTAssertLessThanOrEqual(top.count, 3)
    }

    func testStripsStopwords() {
        let slugs = ["the-and-of-terminal", "the-the-and"]
        let top = Tokenizer.topNouns(from: slugs, limit: 5)
        XCTAssertFalse(top.contains("the"))
        XCTAssertFalse(top.contains("and"))
        XCTAssertFalse(top.contains("of"))
        XCTAssertTrue(top.contains("terminal"))
    }

    func testStripsShortTokens() {
        let slugs = ["a-b-c-terminal"]
        let top = Tokenizer.topNouns(from: slugs, limit: 5)
        XCTAssertFalse(top.contains("a"))
        XCTAssertFalse(top.contains("b"))
        XCTAssertEqual(top.first, "terminal")
    }

    func testStripsNumericSuffixes() {
        let slugs = ["screenshot_143022", "terminal_120000"]
        let top = Tokenizer.topNouns(from: slugs, limit: 5)
        XCTAssertFalse(top.contains("143022"))
        XCTAssertFalse(top.contains("120000"))
    }

    func testEmptyInputReturnsEmptyArray() {
        XCTAssertEqual(Tokenizer.topNouns(from: [], limit: 5), [])
    }

    func testHonorsLimit() {
        let slugs = ["alpha-beta-gamma-delta-epsilon"]
        let top = Tokenizer.topNouns(from: slugs, limit: 2)
        XCTAssertEqual(top.count, 2)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `swift test --filter TokenizerTests`
Expected: FAIL with "Cannot find 'Tokenizer' in scope".

- [ ] **Step 3: Implement Tokenizer**

Create `VibeShot/Models/Tokenizer.swift`:

```swift
import Foundation

/// Extracts the most frequent meaningful tokens from a set of slug strings.
/// Used by the Dashboard's "Today's Vocabulary" editorial callout.
enum Tokenizer {
    private static let stopwords: Set<String> = [
        "the", "and", "for", "with", "from", "this", "that", "into",
        "onto", "your", "you", "are", "but", "not", "all", "any",
        "was", "were", "has", "have", "had", "out", "its", "his",
        "her", "their", "them", "they", "she", "him", "who", "how",
        "why", "what", "when", "where", "which", "ourselves", "png"
    ]

    static func topNouns(from slugs: [String], limit: Int) -> [String] {
        guard limit > 0 else { return [] }

        var counts: [String: Int] = [:]
        for slug in slugs {
            for token in tokens(in: slug) {
                counts[token, default: 0] += 1
            }
        }

        return counts
            .sorted { lhs, rhs in
                if lhs.value != rhs.value {
                    return lhs.value > rhs.value
                }
                return lhs.key < rhs.key
            }
            .prefix(limit)
            .map(\.key)
    }

    private static func tokens(in slug: String) -> [String] {
        slug
            .lowercased()
            .components(separatedBy: CharacterSet.alphanumerics.inverted)
            .filter { token in
                guard token.count >= 3 else { return false }
                guard !stopwords.contains(token) else { return false }
                guard token.contains(where: { $0.isLetter }) else { return false }
                return true
            }
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `swift test --filter TokenizerTests`
Expected: all 6 tests pass.

- [ ] **Step 5: Commit**

```bash
git add VibeShot/Models/Tokenizer.swift Tests/VibeShotTests/TokenizerTests.swift
git commit -m "feat(model): add Tokenizer for vocabulary extraction"
```

---

### Task 3.2: Add caught stage to ProcessingStage

**Files:**
- Modify: `VibeShot/Models/ProcessingEvent.swift`

- [ ] **Step 1: Add the .caught case**

Edit `VibeShot/Models/ProcessingEvent.swift` to read:

```swift
import Foundation

enum ProcessingStage: Equatable {
    case caught
    case analyzing
    case renaming
    case clipboard
    case success(String)
    case error(String)
}

enum WatcherStatus: Equatable {
    case stopped
    case starting
    case running
    case error(String)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds (`.caught` is unused so far, no warnings on enums).

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Models/ProcessingEvent.swift
git commit -m "feat(model): add .caught stage to ProcessingStage"
```

---

### Task 3.3: Extend AppState with derived editorial state

**Files:**
- Modify: `VibeShot/AppState.swift`

- [ ] **Step 1: Add new stored properties to AppState**

Open `VibeShot/AppState.swift`. After the `var apiStatus: String = "N/A"` line, add:

```swift
    var session = SessionMeta()
    var hourlyThroughput: [Int] = Array(repeating: 0, count: 24)
    var successWindow: [Double] = []
```

- [ ] **Step 2: Add derived computed properties**

Inside the `AppState` class, after `var p95Latency: TimeInterval { ... }`, add:

```swift
    var vocabularyToday: [String] {
        let calendar = Calendar.current
        let today = calendar.startOfDay(for: Date())
        let slugs = recentFiles
            .filter { !$0.isError }
            .filter { calendar.startOfDay(for: $0.timestamp) == today }
            .map { $0.newName }
        return Tokenizer.topNouns(from: slugs, limit: 5)
    }

    var latencyHistory: [Double] {
        Array(latencies.suffix(24))
    }

    var successRateHistory: [Double] {
        Array(successWindow.suffix(20))
    }
```

- [ ] **Step 3: Mark `.caught` in handleNewFile before eligibility check**

Locate the `handleNewFile` function in `AppState.swift`. Replace its current implementation with:

```swift
    private func handleNewFile(url: URL, watcherStartedAt: Date) async {
        currentFile = url.lastPathComponent
        currentStage = .caught

        guard ScreenshotGuard.isEligible(url: url, watcherStartedAt: watcherStartedAt) else {
            currentFile = nil
            currentStage = nil
            return
        }

        let originalName = url.lastPathComponent
        currentFile = originalName
        currentStage = .analyzing
        totalProcessed += 1

        let hourBucket = Calendar.current.component(.hour, from: Date())
        hourlyThroughput[hourBucket] += 1

        do {
            currentStage = .renaming

            let result = try await processor.process(
                screenshotURL: url,
                outputDir: URL(fileURLWithPath: resolvedOutputDir),
                baseURL: resolvedBaseURL,
                apiKey: config.apiKey ?? "",
                model: resolvedModel,
                language: resolvedLanguage,
                copyToClipboard: resolvedCopyToClipboard
            )

            if resolvedCopyToClipboard {
                currentStage = .clipboard
                try? await Task.sleep(nanoseconds: 200_000_000)
            }

            successes += 1
            latencies.append(result.duration)
            if latencies.count > 100 { latencies.removeFirst() }

            successWindow.append(1.0)
            if successWindow.count > 100 { successWindow.removeFirst() }

            let entry = RecentFile(
                originalName: result.originalName,
                newName: result.destinationURL.lastPathComponent,
                path: result.destinationURL.path,
                timestamp: Date(),
                duration: result.duration,
                result: .success
            )
            recentFiles.insert(entry, at: 0)
            if recentFiles.count > 50 { recentFiles = Array(recentFiles.prefix(50)) }

            currentStage = .success(result.destinationURL.lastPathComponent)
            currentFile = result.destinationURL.lastPathComponent
        } catch {
            errors += 1
            currentStage = .error(error.localizedDescription)

            successWindow.append(0.0)
            if successWindow.count > 100 { successWindow.removeFirst() }

            let entry = RecentFile(
                originalName: originalName,
                newName: "",
                path: "",
                timestamp: Date(),
                duration: 0,
                result: .error(error.localizedDescription)
            )
            recentFiles.insert(entry, at: 0)
            if recentFiles.count > 50 { recentFiles = Array(recentFiles.prefix(50)) }
        }
    }
```

- [ ] **Step 4: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 5: Commit**

```bash
git add VibeShot/AppState.swift
git commit -m "feat(state): add session, vocabulary, sparkline, and caught-stage tracking"
```

---

## Phase 4 — Dashboard composition

### Task 4.1: Create StageProgress component

**Files:**
- Create: `VibeShot/Views/Editorial/StageProgress.swift`

- [ ] **Step 1: Write StageProgress.swift**

```swift
import SwiftUI

/// Three-label horizontal strip showing pipeline stages.
/// Active label is coral with an animated underline; completed labels
/// are ink; upcoming labels are textSoft.
struct StageProgress: View {
    let stage: ProcessingStage?
    let includesClipboard: Bool

    private var labels: [String] {
        includesClipboard
            ? ["Analyzing", "Renaming", "Clipboard"]
            : ["Analyzing", "Renaming"]
    }

    private func state(for label: String) -> LabelState {
        guard let stage else { return .upcoming }
        let activeIdx: Int
        switch stage {
        case .caught:    activeIdx = -1
        case .analyzing: activeIdx = 0
        case .renaming:  activeIdx = 1
        case .clipboard: activeIdx = includesClipboard ? 2 : 1
        case .success:   activeIdx = labels.count
        case .error:     return .upcoming
        }
        guard let idx = labels.firstIndex(of: label) else { return .upcoming }
        if idx < activeIdx { return .completed }
        if idx == activeIdx { return .active }
        return .upcoming
    }

    var body: some View {
        HStack(spacing: Theme.sLg) {
            ForEach(labels, id: \.self) { label in
                VStack(alignment: .leading, spacing: 2) {
                    Text(label)
                        .smallCaps()
                        .foregroundStyle(color(for: state(for: label)))
                    Rectangle()
                        .fill(state(for: label) == .active ? Theme.coral : Color.clear)
                        .frame(height: 1)
                        .animation(.easeInOut(duration: 0.24), value: stage)
                }
            }
            Spacer()
        }
    }

    private enum LabelState { case upcoming, active, completed }

    private func color(for s: LabelState) -> Color {
        switch s {
        case .active:    return Theme.coral
        case .completed: return Theme.ink
        case .upcoming:  return Theme.textSoft
        }
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 20) {
        StageProgress(stage: .analyzing, includesClipboard: true)
        StageProgress(stage: .renaming, includesClipboard: true)
        StageProgress(stage: .clipboard, includesClipboard: true)
        StageProgress(stage: nil, includesClipboard: true)
        StageProgress(stage: .renaming, includesClipboard: false)
    }
    .padding(40)
    .background(Theme.canvas)
    .frame(width: 380)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/StageProgress.swift
git commit -m "feat(editorial): add StageProgress strip"
```

---

### Task 4.2: Create HeroCard component

**Files:**
- Create: `VibeShot/Views/Editorial/HeroCard.swift`

- [ ] **Step 1: Write HeroCard.swift**

```swift
import SwiftUI
import AppKit

/// The "now playing" centerpiece on the Dashboard's left column.
/// Idle: shutter glyph + reassuring copy. Active: thumbnail, slug-being-typed,
/// stage progress, elapsed time.
struct HeroCard: View {
    let stage: ProcessingStage?
    let currentFile: String?
    let thumbnailPath: String?
    let proposedSlug: String?
    let elapsedSeconds: TimeInterval
    let includesClipboard: Bool

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var typedCount: Int = 0
    @State private var blink: Bool = false

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            if isActive {
                activeView
            } else {
                idleView
            }
        }
        .padding(Theme.sLg)
        .frame(maxWidth: .infinity, alignment: .topLeading)
        .frame(minHeight: 310)
        .background(Theme.surfaceCard)
        .clipShape(.rect(cornerRadius: Theme.r10))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.r10)
                .stroke(Theme.rule, lineWidth: 0.5)
        )
        .onChange(of: proposedSlug ?? "") { _, newSlug in
            startTyping(for: newSlug)
        }
        .onAppear {
            startTyping(for: proposedSlug ?? "")
            Timer.scheduledTimer(withTimeInterval: 0.5, repeats: true) { _ in
                blink.toggle()
            }
        }
    }

    private var isActive: Bool {
        guard let stage else { return false }
        switch stage {
        case .caught, .analyzing, .renaming, .clipboard: return true
        case .success: return true
        case .error: return false
        }
    }

    @ViewBuilder
    private var idleView: some View {
        VStack(spacing: Theme.sMed) {
            Spacer()
            EditorialIcon.Shutter(size: 80, color: Theme.textSoft.opacity(0.6))
            Text("No captures yet. The page will fill as you work.")
                .font(Theme.serifItalicSm)
                .foregroundStyle(Theme.textMuted)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 280)
            Spacer()
        }
        .frame(maxWidth: .infinity)
    }

    @ViewBuilder
    private var activeView: some View {
        thumbnail
            .frame(maxWidth: .infinity, maxHeight: 160)
            .clipShape(.rect(cornerRadius: Theme.r10))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.r10)
                    .fill(Theme.paperTint)
            )

        VStack(alignment: .leading, spacing: Theme.sMicro) {
            HStack(alignment: .firstTextBaseline, spacing: Theme.sSmall) {
                Text(displayedSlug)
                    .font(.system(size: 14, design: .monospaced))
                    .foregroundStyle(Theme.ink)
                if showCursor {
                    Text("|")
                        .font(.system(size: 14, design: .monospaced))
                        .foregroundStyle(Theme.coral)
                        .opacity(blink ? 1 : 0)
                }
                Spacer()
            }
            if let original = currentFile, !original.isEmpty {
                Text(original)
                    .marginalia()
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
        }

        StageProgress(stage: stage, includesClipboard: includesClipboard)

        Text(elapsedText)
            .font(Theme.codeSm)
            .foregroundStyle(Theme.textSoft)
    }

    @ViewBuilder
    private var thumbnail: some View {
        if let path = thumbnailPath,
           !path.isEmpty,
           let image = NSImage(contentsOfFile: path) {
            Image(nsImage: image)
                .resizable()
                .scaledToFill()
        } else {
            Rectangle()
                .fill(Theme.surfaceMuted)
                .overlay(
                    EditorialIcon.Shutter(size: 60, color: Theme.textSoft.opacity(0.4))
                )
        }
    }

    private var displayedSlug: String {
        guard let slug = proposedSlug, !slug.isEmpty else { return "—" }
        if reduceMotion { return slug }
        return String(slug.prefix(typedCount))
    }

    private var showCursor: Bool {
        guard !reduceMotion else { return false }
        guard let slug = proposedSlug, !slug.isEmpty else { return false }
        return typedCount < slug.count
    }

    private var elapsedText: String {
        if elapsedSeconds <= 0 { return "—" }
        if elapsedSeconds < 1 { return String(format: "%.0fms", elapsedSeconds * 1000) }
        return String(format: "%.1fs elapsed", elapsedSeconds)
    }

    private func startTyping(for slug: String) {
        guard !reduceMotion else { typedCount = slug.count; return }
        typedCount = 0
        guard !slug.isEmpty else { return }

        let total = slug.count
        let interval: TimeInterval = 0.028
        Timer.scheduledTimer(withTimeInterval: interval, repeats: true) { timer in
            DispatchQueue.main.async {
                if typedCount < total {
                    typedCount += 1
                } else {
                    timer.invalidate()
                }
            }
        }
    }
}

#Preview("Idle") {
    HeroCard(stage: nil, currentFile: nil, thumbnailPath: nil,
             proposedSlug: nil, elapsedSeconds: 0, includesClipboard: true)
        .padding(40)
        .frame(width: 460, height: 360)
        .background(Theme.canvas)
}

#Preview("Active") {
    HeroCard(stage: .renaming,
             currentFile: "Screenshot 2026-05-30 at 11.42.13.png",
             thumbnailPath: nil,
             proposedSlug: "terminal-git-log_114213",
             elapsedSeconds: 1.4,
             includesClipboard: true)
        .padding(40)
        .frame(width: 460, height: 360)
        .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/HeroCard.swift
git commit -m "feat(editorial): add HeroCard with typewriter slug and shutter idle"
```

---

### Task 4.3: Rewrite DashboardView as broadsheet

**Files:**
- Modify: `VibeShot/Views/DashboardView.swift`

- [ ] **Step 1: Replace DashboardView.swift contents**

Replace the entire contents of `VibeShot/Views/DashboardView.swift` with:

```swift
import SwiftUI

struct DashboardView: View {
    @Environment(AppState.self) private var appState
    @State private var uptimeTick = false
    @State private var ellipsisCount = 0

    var body: some View {
        ZStack {
            PaperBackground()

            VStack(spacing: 0) {
                Masthead(sessionNumber: appState.session.sessionNumber, date: Date())

                ScrollView {
                    VStack(alignment: .leading, spacing: Theme.sSec) {
                        headlineStrip
                        bodyColumns
                    }
                    .padding(.horizontal, Theme.sSec)
                    .padding(.vertical, Theme.sLg)
                }

                colophon
            }
        }
        .frame(minWidth: 760, minHeight: 540)
        .onAppear {
            Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { _ in
                uptimeTick.toggle()
            }
            Timer.scheduledTimer(withTimeInterval: 0.6, repeats: true) { _ in
                ellipsisCount = (ellipsisCount + 1) % 4
            }
        }
    }

    // MARK: - Headline strip

    @ViewBuilder
    private var headlineStrip: some View {
        HStack(alignment: .top) {
            VStack(alignment: .leading, spacing: Theme.sSmall) {
                headlineText
                datelineText
            }
            Spacer()
            controlButton
        }
    }

    @ViewBuilder
    private var headlineText: some View {
        switch appState.watcherStatus {
        case .stopped:
            Text("Quietly Watching.")
                .font(Theme.displayHero)
                .italic()
                .foregroundStyle(Theme.warmInk)
                .tracking(-0.5)
        case .starting:
            Text("Waking up\(ellipsisDots)")
                .font(Theme.displayHero)
                .foregroundStyle(Theme.warmInk)
                .tracking(-0.5)
        case .error(let msg):
            Text(msg)
                .font(Theme.displayHero)
                .foregroundStyle(Theme.coral)
                .tracking(-0.5)
        case .running:
            switch appState.currentStage {
            case .analyzing, .renaming, .clipboard, .caught:
                Text("Reading a new screenshot\(ellipsisDots)")
                    .font(Theme.displayHero)
                    .foregroundStyle(Theme.warmInk)
                    .tracking(-0.5)
            case .error:
                Text("Trouble afoot.")
                    .font(Theme.displayHero)
                    .foregroundStyle(Theme.coral)
                    .tracking(-0.5)
            case .success, .none:
                Text("Watching for screenshots.")
                    .font(Theme.displayHero)
                    .foregroundStyle(Theme.warmInk)
                    .tracking(-0.5)
            }
        }
    }

    private var ellipsisDots: String {
        switch ellipsisCount {
        case 1: return "."
        case 2: return ".."
        case 3: return "..."
        default: return ""
        }
    }

    @ViewBuilder
    private var datelineText: some View {
        let _ = uptimeTick
        let uptime: String = {
            guard let t = appState.watcherStartedAt, appState.isWatching else { return "—" }
            return fmtUptime(t)
        }()
        Text("RUNNING \(uptime) · PROVIDER · \(appState.resolvedProvider.displayName.uppercased())")
            .smallCaps()
            .foregroundStyle(Theme.textSoft)
    }

    @ViewBuilder
    private var controlButton: some View {
        if appState.isWatching {
            Button { appState.stop() } label: {
                HStack(spacing: 6) {
                    Image(systemName: "stop.fill").font(.system(size: 8))
                    Text("Stop").font(Theme.button)
                }
                .foregroundStyle(.white)
                .padding(.horizontal, 16)
                .padding(.vertical, 7)
                .background(Theme.error)
                .clipShape(.rect(cornerRadius: Theme.r6))
            }
            .buttonStyle(.plain)
        } else {
            Button { appState.start() } label: {
                HStack(spacing: 6) {
                    Image(systemName: "play.fill").font(.system(size: 9))
                    Text("Start Watching").font(Theme.button)
                }
                .foregroundStyle(.white)
                .padding(.horizontal, 18)
                .padding(.vertical, 7)
                .background(Theme.coral)
                .clipShape(.rect(cornerRadius: Theme.r6))
            }
            .buttonStyle(.plain)
        }
    }

    // MARK: - Body columns

    @ViewBuilder
    private var bodyColumns: some View {
        HStack(alignment: .top, spacing: 28) {
            heroColumn
                .frame(maxWidth: .infinity)
                .layoutPriority(2)
            editorialColumn
                .frame(width: 280)
        }
    }

    @ViewBuilder
    private var heroColumn: some View {
        HeroCard(
            stage: appState.currentStage,
            currentFile: appState.currentFile,
            thumbnailPath: latestThumbnailPath,
            proposedSlug: proposedSlug,
            elapsedSeconds: heroElapsed,
            includesClipboard: appState.resolvedCopyToClipboard
        )
    }

    private var latestThumbnailPath: String? {
        appState.recentFiles.first { !$0.isError && !$0.path.isEmpty }?.path
    }

    private var proposedSlug: String? {
        switch appState.currentStage {
        case .success(let name): return name
        default: return appState.currentFile
        }
    }

    private var heroElapsed: TimeInterval {
        guard let last = appState.recentFiles.first else { return 0 }
        return last.duration
    }

    @ViewBuilder
    private var editorialColumn: some View {
        VStack(alignment: .leading, spacing: Theme.sLg) {
            todaysReadingBlock
            EditorialRule()
            numbersBlock
            EditorialRule()
            if !appState.vocabularyToday.isEmpty {
                vocabularyBlock
                EditorialRule()
            }
            directoriesBlock
        }
    }

    @ViewBuilder
    private var todaysReadingBlock: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            Text("TODAY'S READING").smallCaps().foregroundStyle(Theme.textMuted)
            Sparkline(
                values: appState.latencyHistory,
                caption: "Fig. 1 — Latency, last \(appState.latencyHistory.count)",
                lastValueText: appState.latencyHistory.last.map { String(format: "%.1fs", $0) }
            )
            Sparkline(
                values: appState.hourlyThroughput.map(Double.init),
                caption: "Fig. 2 — Throughput, hourly",
                lastValueText: appState.hourlyThroughput.last.map(String.init)
            )
            Sparkline(
                values: appState.successRateHistory,
                caption: "Fig. 3 — Success, last \(appState.successRateHistory.count)",
                lastValueText: appState.successRateHistory.last.map { String(format: "%.0f%%", $0 * 100) }
            )
        }
    }

    @ViewBuilder
    private var numbersBlock: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text("NUMBERS").smallCaps().foregroundStyle(Theme.textMuted)
            HStack(spacing: Theme.sMed) {
                MetricFigure(value: "\(appState.totalProcessed)", label: "Processed")
                MetricFigure(value: "\(appState.successes)", label: "Successful")
                MetricFigure(
                    value: "\(appState.errors)",
                    label: "Errors",
                    accent: appState.errors > 0 ? Theme.error : nil
                )
            }
            Text("AVG \(fmtDur(appState.avgLatency)) · P95 \(fmtDur(appState.p95Latency))")
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textSoft)
        }
    }

    @ViewBuilder
    private var vocabularyBlock: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text("TODAY'S VOCABULARY").smallCaps().foregroundStyle(Theme.textMuted)
            Text(appState.vocabularyToday.joined(separator: ", ") + ".")
                .font(Theme.serifItalicLg)
                .foregroundStyle(Theme.warmInk)
                .lineLimit(3)
        }
    }

    @ViewBuilder
    private var directoriesBlock: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            Text("DIRECTORIES").smallCaps().foregroundStyle(Theme.textMuted)
            directoryRow(icon: AnyView(EditorialIcon.Eye(size: 16)),
                         label: "WATCH",
                         path: appState.resolvedWatchDir)
            directoryRow(icon: AnyView(EditorialIcon.TrayArrow(size: 16)),
                         label: "OUTPUT",
                         path: appState.resolvedOutputDir)
        }
    }

    @ViewBuilder
    private func directoryRow(icon: AnyView, label: String, path: String) -> some View {
        HStack(alignment: .center, spacing: Theme.sSmall) {
            icon
            VStack(alignment: .leading, spacing: 2) {
                Text(label).smallCaps().foregroundStyle(Theme.textSoft)
                Text(abbrev(path))
                    .font(Theme.codeSm)
                    .foregroundStyle(Theme.textMuted)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
        }
    }

    // MARK: - Colophon

    @ViewBuilder
    private var colophon: some View {
        VStack(spacing: 0) {
            EditorialRule()
            HStack(spacing: Theme.sLg) {
                Text("API · \(appState.apiStatus.uppercased())")
                    .smallCaps()
                    .foregroundStyle(apiColor)
                Text("PROVIDER · \(appState.resolvedProvider.displayName.uppercased())")
                    .smallCaps()
                    .foregroundStyle(Theme.textSoft)
                Text("MODEL · \(appState.resolvedModel.uppercased())")
                    .smallCaps()
                    .foregroundStyle(Theme.textSoft)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Spacer()
                if let t = appState.watcherStartedAt, appState.isWatching {
                    let _ = uptimeTick
                    Text("\(fmtUptime(t)) UPTIME")
                        .smallCaps()
                        .foregroundStyle(Theme.textSoft)
                }
            }
            .padding(.horizontal, Theme.sSec)
            .padding(.vertical, Theme.sSmall)
        }
    }

    private var apiColor: Color {
        switch appState.apiStatus {
        case "OK": return Theme.success
        case "FAIL": return Theme.error
        default: return Theme.textSoft
        }
    }

    // MARK: - Formatting

    private func fmtDur(_ s: TimeInterval) -> String {
        if s <= 0 { return "—" }
        if s < 1 { return String(format: "%.0fms", s * 1000) }
        return String(format: "%.1fs", s)
    }

    private func fmtUptime(_ d: Date) -> String {
        let i = Int(Date().timeIntervalSince(d))
        let h = i / 3600, m = (i % 3600) / 60, s = i % 60
        if h > 0 { return "\(h)H \(m)M" }
        return "\(m)M \(s)S"
    }

    private func abbrev(_ p: String) -> String {
        p.replacingOccurrences(of: NSHomeDirectory(), with: "~")
    }
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/DashboardView.swift
git commit -m "feat(dashboard): rewrite as editorial broadsheet"
```

---

## Phase 5 — Pipeline composition

### Task 5.1: Create PipelineCard component

**Files:**
- Create: `VibeShot/Views/Editorial/PipelineCard.swift`

- [ ] **Step 1: Write PipelineCard.swift**

```swift
import SwiftUI

/// A flat "clipping" representing a screenshot in the pipeline.
/// 40x40 thumbnail on the left, slug + original filename on the right,
/// timestamp footer. Errors get a coral left-edge rule instead of the top perforation.
struct PipelineCard: View {
    let file: RecentFile
    var isInFlight: Bool = false

    var body: some View {
        ZStack(alignment: .topTrailing) {
            HStack(alignment: .top, spacing: Theme.sSmall) {
                thumbnail
                content
            }
            .padding(Theme.sSmall + 2)
            .background(Theme.surfaceCard)
            .overlay(alignment: .top) {
                if !file.isError {
                    Rectangle()
                        .fill(Theme.rule)
                        .frame(height: 0.5)
                }
            }
            .overlay(alignment: .leading) {
                if file.isError {
                    Rectangle()
                        .fill(Theme.coral)
                        .frame(width: 1)
                }
            }

            if !file.isError && !isInFlight {
                Text("✓")
                    .font(.system(size: 10, design: .serif))
                    .foregroundStyle(Theme.success)
                    .padding(.top, 6)
                    .padding(.trailing, 8)
            }
        }
    }

    @ViewBuilder
    private var thumbnail: some View {
        ZStack {
            if !file.path.isEmpty,
               let image = NSImage(contentsOfFile: file.path) {
                Image(nsImage: image)
                    .resizable()
                    .scaledToFill()
            } else {
                Rectangle().fill(Theme.surfaceMuted)
            }
        }
        .frame(width: 40, height: 40)
        .clipShape(.rect(cornerRadius: Theme.r6))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.r6)
                .fill(Theme.paperTint)
        )
    }

    @ViewBuilder
    private var content: some View {
        VStack(alignment: .leading, spacing: 2) {
            if file.isError {
                Text(file.resultText)
                    .font(.system(size: 12, design: .serif).italic())
                    .foregroundStyle(Theme.ink)
                    .lineLimit(2)
                    .truncationMode(.tail)
            } else {
                Text(file.newName.isEmpty ? file.originalName : file.newName)
                    .font(.system(size: 12, design: .monospaced))
                    .foregroundStyle(Theme.ink)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }

            Text(file.originalName)
                .font(Theme.serifItalicSm)
                .foregroundStyle(Theme.textSoft)
                .lineLimit(1)
                .truncationMode(.middle)

            Text(file.timestamp, format: .dateTime.hour().minute().second())
                .font(.system(size: 10, design: .monospaced))
                .foregroundStyle(Theme.textSoft)
                .tracking(0.5)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 0) {
        PipelineCard(file: RecentFile(
            originalName: "Screenshot 2026-05-30 at 11.42.13.png",
            newName: "terminal-git-log_114213.png",
            path: "",
            timestamp: Date(),
            duration: 1.4,
            result: .success
        ))
        PipelineCard(file: RecentFile(
            originalName: "Screenshot 2026-05-30 at 11.55.02.png",
            newName: "",
            path: "",
            timestamp: Date(),
            duration: 0,
            result: .error("API timeout after 30s")
        ))
        PipelineCard(file: RecentFile(
            originalName: "Screenshot.png",
            newName: "in-flight-card.png",
            path: "",
            timestamp: Date(),
            duration: 0,
            result: .success
        ), isInFlight: true)
    }
    .padding(20)
    .background(Theme.canvas)
    .frame(width: 320)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/PipelineCard.swift
git commit -m "feat(editorial): add PipelineCard clipping"
```

---

### Task 5.2: Create PipelineColumn component

**Files:**
- Create: `VibeShot/Views/Editorial/PipelineColumn.swift`

- [ ] **Step 1: Write PipelineColumn.swift**

```swift
import SwiftUI

/// A single kanban column: header with small-caps label, optional active-state coral rule,
/// then content area. Used for Caught / Reading / Setting (Filed has its own component).
struct PipelineColumn<Content: View>: View {
    let title: String
    let isActive: Bool
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            content()
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        }
    }

    @ViewBuilder
    private var header: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text(title)
                .smallCaps()
                .foregroundStyle(Theme.ink)
            Rectangle()
                .fill(isActive ? Theme.coral : Theme.rule)
                .frame(height: 0.5)
                .animation(.easeInOut(duration: 0.2), value: isActive)
        }
        .padding(.bottom, Theme.sSmall)
    }
}

/// Em-dash placeholder shown when a column has no items.
struct EmptyColumnDash: View {
    var body: some View {
        Text("—")
            .font(.system(size: 18, design: .serif))
            .foregroundStyle(Theme.ink.opacity(0.3))
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

#Preview {
    HStack(alignment: .top, spacing: 16) {
        PipelineColumn(title: "CAUGHT", isActive: false) { EmptyColumnDash() }
        PipelineColumn(title: "READING", isActive: true) { EmptyColumnDash() }
        PipelineColumn(title: "SETTING", isActive: false) { EmptyColumnDash() }
    }
    .frame(width: 460, height: 240)
    .padding(20)
    .background(Theme.canvas)
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/PipelineColumn.swift
git commit -m "feat(editorial): add PipelineColumn and EmptyColumnDash"
```

---

### Task 5.3: Create FiledColumn component

**Files:**
- Create: `VibeShot/Views/Editorial/FiledColumn.swift`

- [ ] **Step 1: Write FiledColumn.swift**

```swift
import SwiftUI
import AppKit

/// The right-most kanban column. Contains a search field, three filter chips,
/// and date-grouped PipelineCards. Owns the historical record.
struct FiledColumn: View {
    let files: [RecentFile]
    @Binding var searchText: String
    @Binding var filter: Filter
    @Binding var selection: RecentFile.ID?
    @FocusState.Binding var searchFocused: Bool

    enum Filter: String, CaseIterable {
        case all = "ALL"
        case errors = "ERRORS"
        case today = "TODAY"
    }

    private var visible: [RecentFile] {
        var result = files
        if filter == .errors { result = result.filter(\.isError) }
        if filter == .today {
            let today = Calendar.current.startOfDay(for: Date())
            result = result.filter { Calendar.current.startOfDay(for: $0.timestamp) == today }
        }
        if !searchText.isEmpty {
            let q = searchText.lowercased()
            result = result.filter {
                $0.newName.lowercased().contains(q) ||
                $0.originalName.lowercased().contains(q) ||
                ($0.isError && $0.resultText.lowercased().contains(q))
            }
        }
        return result
    }

    private var grouped: [(label: String, items: [RecentFile])] {
        let cal = Calendar.current
        let today = cal.startOfDay(for: Date())
        let yesterday = cal.date(byAdding: .day, value: -1, to: today) ?? today

        var todayItems: [RecentFile] = []
        var yesterdayItems: [RecentFile] = []
        var earlierItems: [RecentFile] = []

        for f in visible {
            let day = cal.startOfDay(for: f.timestamp)
            if day == today { todayItems.append(f) }
            else if day == yesterday { yesterdayItems.append(f) }
            else { earlierItems.append(f) }
        }

        let df = DateFormatter()
        df.dateFormat = "d MMM"

        var sections: [(String, [RecentFile])] = []
        if !todayItems.isEmpty {
            sections.append(("TODAY · \(df.string(from: today).uppercased())", todayItems))
        }
        if !yesterdayItems.isEmpty {
            sections.append(("YESTERDAY · \(df.string(from: yesterday).uppercased())", yesterdayItems))
        }
        if !earlierItems.isEmpty {
            sections.append(("EARLIER", earlierItems))
        }
        return sections
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            content
        }
    }

    @ViewBuilder
    private var header: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            HStack {
                Text("FILED")
                    .smallCaps()
                    .foregroundStyle(Theme.ink)
                Spacer()
            }
            Rectangle()
                .fill(Theme.rule)
                .frame(height: 0.5)

            HStack(spacing: Theme.sMed) {
                searchField
                Spacer()
                filterChips
            }
            .padding(.top, Theme.sSmall)
        }
        .padding(.bottom, Theme.sSmall)
    }

    @ViewBuilder
    private var searchField: some View {
        ZStack(alignment: .bottom) {
            TextField("", text: $searchText, prompt: Text("Search filings…")
                .font(Theme.serifItalicSm)
                .foregroundColor(Theme.textSoft))
                .textFieldStyle(.plain)
                .font(Theme.body)
                .foregroundStyle(Theme.ink)
                .focused($searchFocused)
                .frame(maxWidth: 220)
            Rectangle()
                .fill(searchFocused ? Theme.coral : Theme.rule)
                .frame(height: searchFocused ? 1 : 0.5)
                .animation(.easeInOut(duration: 0.14), value: searchFocused)
        }
    }

    @ViewBuilder
    private var filterChips: some View {
        HStack(spacing: Theme.sMed) {
            ForEach(Filter.allCases, id: \.self) { f in
                chip(label: f.rawValue, isActive: filter == f) {
                    filter = f
                }
            }
        }
    }

    @ViewBuilder
    private func chip(label: String, isActive: Bool, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            VStack(spacing: 2) {
                Text(label)
                    .font(.system(size: 10, weight: isActive ? .medium : .regular))
                    .tracking(1.2)
                    .foregroundStyle(isActive ? Theme.ink : Theme.textSoft)
                Rectangle()
                    .fill(isActive ? Theme.coral : Color.clear)
                    .frame(height: 1.5)
                    .animation(.easeInOut(duration: 0.16), value: isActive)
            }
        }
        .buttonStyle(.plain)
    }

    @ViewBuilder
    private var content: some View {
        if visible.isEmpty {
            VStack {
                Spacer()
                Text(filter == .errors ? "No errors filed." : "No filings yet.")
                    .font(Theme.serifItalicLg)
                    .foregroundStyle(Theme.textMuted)
                Spacer()
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(grouped, id: \.label) { section in
                        DateDivider(label: section.label)
                        ForEach(section.items) { file in
                            cardRow(file: file)
                        }
                    }
                }
            }
        }
    }

    @ViewBuilder
    private func cardRow(file: RecentFile) -> some View {
        PipelineCard(file: file)
            .background(
                Rectangle()
                    .fill(selection == file.id ? Theme.surfaceMuted : Color.clear)
            )
            .onTapGesture(count: 2) {
                openFile(file)
            }
            .onTapGesture {
                selection = file.id
            }
            .contextMenu {
                if !file.path.isEmpty {
                    Button("Reveal in Finder") {
                        NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: file.path)])
                    }
                }
            }
    }

    private func openFile(_ file: RecentFile) {
        guard !file.path.isEmpty else { return }
        NSWorkspace.shared.open(URL(fileURLWithPath: file.path))
    }
}

#Preview {
    struct PreviewWrapper: View {
        @State var search = ""
        @State var filter: FiledColumn.Filter = .all
        @State var selection: RecentFile.ID? = nil
        @FocusState var focus: Bool

        var body: some View {
            FiledColumn(
                files: [
                    RecentFile(originalName: "Screenshot.png", newName: "alpha.png",
                               path: "", timestamp: Date(), duration: 1.1, result: .success),
                    RecentFile(originalName: "Screenshot.png", newName: "",
                               path: "", timestamp: Date(), duration: 0, result: .error("API failed"))
                ],
                searchText: $search,
                filter: $filter,
                selection: $selection,
                searchFocused: $focus
            )
            .padding(20)
            .frame(width: 360, height: 360)
            .background(Theme.canvas)
        }
    }
    return PreviewWrapper()
}
```

- [ ] **Step 2: Compile-check**

Run: `swift build`
Expected: build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/Editorial/FiledColumn.swift
git commit -m "feat(editorial): add FiledColumn with search, filter chips, date grouping"
```

---

### Task 5.4: Create PipelineView (replaces HistoryView)

**Files:**
- Create: `VibeShot/Views/PipelineView.swift`
- Delete: `VibeShot/Views/HistoryView.swift`

- [ ] **Step 1: Write PipelineView.swift**

Create `VibeShot/Views/PipelineView.swift`:

```swift
import SwiftUI

struct PipelineView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    @State private var searchText: String = ""
    @State private var filter: FiledColumn.Filter = .all
    @State private var selection: RecentFile.ID?
    @FocusState private var searchFocused: Bool

    @Namespace private var pipelineNS

    var body: some View {
        ZStack {
            PaperBackground()

            VStack(spacing: 0) {
                Masthead(sessionNumber: appState.session.sessionNumber, date: Date())

                board
                    .padding(.horizontal, Theme.sLg)
                    .padding(.vertical, Theme.sLg)
            }
        }
        .frame(minWidth: 760, minHeight: 540)
        .focusable()
        .focusEffectDisabled()
        .onKeyPress(.return) {
            if let id = selection,
               let file = appState.recentFiles.first(where: { $0.id == id }),
               !file.path.isEmpty {
                NSWorkspace.shared.open(URL(fileURLWithPath: file.path))
                return .handled
            }
            return .ignored
        }
    }

    @ViewBuilder
    private var board: some View {
        HStack(alignment: .top, spacing: Theme.sMed) {
            PipelineColumn(title: "CAUGHT", isActive: activeColumn == .caught) {
                liveCardArea(showCard: activeColumn == .caught)
            }
            .frame(width: 140)

            PipelineColumn(title: "READING", isActive: activeColumn == .reading) {
                liveCardArea(showCard: activeColumn == .reading)
            }
            .frame(width: 140)

            PipelineColumn(title: "SETTING", isActive: activeColumn == .setting) {
                liveCardArea(showCard: activeColumn == .setting)
            }
            .frame(width: 140)

            FiledColumn(
                files: appState.recentFiles,
                searchText: $searchText,
                filter: $filter,
                selection: $selection,
                searchFocused: $searchFocused
            )
            .frame(maxWidth: .infinity)
        }
    }

    @ViewBuilder
    private func liveCardArea(showCard: Bool) -> some View {
        if showCard, let card = inFlightCard {
            PipelineCard(file: card, isInFlight: true)
                .matchedGeometryEffect(id: "inFlight", in: pipelineNS)
                .transition(.opacity)
        } else {
            EmptyColumnDash()
        }
    }

    private var activeColumn: PipelineStage? {
        guard let stage = appState.currentStage else { return nil }
        switch stage {
        case .caught: return .caught
        case .analyzing: return .reading
        case .renaming, .clipboard: return .setting
        case .success, .error: return nil
        }
    }

    private var inFlightCard: RecentFile? {
        guard let original = appState.currentFile else { return nil }
        return RecentFile(
            originalName: original,
            newName: "",
            path: "",
            timestamp: Date(),
            duration: 0,
            result: .success
        )
    }

    private enum PipelineStage { case caught, reading, setting }

    func focusSearch() {
        searchFocused = true
    }
}
```

- [ ] **Step 2: Delete HistoryView.swift**

```bash
rm VibeShot/Views/HistoryView.swift
```

- [ ] **Step 3: Compile-check (expect ContentView to fail since it still references HistoryView)**

Run: `swift build`
Expected: build FAILS with reference to HistoryView. This is expected — Task 6.1 fixes ContentView.

- [ ] **Step 4: Commit (note: build will be broken until Task 6.1; that's OK because they will land together in a series of commits)**

```bash
git add VibeShot/Views/PipelineView.swift VibeShot/Views/HistoryView.swift
git commit -m "feat(pipeline): add PipelineView; remove HistoryView (ContentView update pending)"
```

---

## Phase 6 — App wiring

### Task 6.1: Update ContentView for new tabs and window size

**Files:**
- Modify: `VibeShot/Views/ContentView.swift`

- [ ] **Step 1: Replace ContentView.swift contents**

```swift
import SwiftUI

struct ContentView: View {
    @Environment(AppState.self) private var appState
    @State private var selectedTab: AppTab = .dashboard

    enum AppTab: Hashable { case dashboard, pipeline }

    var body: some View {
        TabView(selection: $selectedTab) {
            Tab("Dashboard", systemImage: "newspaper", value: AppTab.dashboard) {
                DashboardView()
            }
            Tab("Pipeline", systemImage: "square.stack.3d.up", value: AppTab.pipeline) {
                PipelineView()
            }
        }
        .frame(minWidth: 760, minHeight: 540)
    }
}
```

- [ ] **Step 2: Build**

Run: `swift build`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/Views/ContentView.swift
git commit -m "feat(shell): rename History tab to Pipeline; bump window minimum to 760x540"
```

---

### Task 6.2: Update VibeShotApp default size to match

**Files:**
- Modify: `VibeShot/VibeShotApp.swift`

- [ ] **Step 1: Update default window size**

In `VibeShot/VibeShotApp.swift`, change the `.defaultSize` line to:

```swift
        .defaultSize(width: 880, height: 620)
```

(The full file should now have the new default; everything else stays.)

- [ ] **Step 2: Build**

Run: `swift build`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/VibeShotApp.swift
git commit -m "feat(shell): bump default window size to 880x620"
```

---

### Task 6.3: Add keyboard shortcut commands

**Files:**
- Modify: `VibeShot/VibeShotApp.swift`

- [ ] **Step 1: Add tab-switching and Filed-search shortcuts**

Replace the `.commands { ... }` block in `VibeShotApp.swift` with:

```swift
        .commands {
            CommandGroup(after: .appSettings) {
                Button("Check API Health") {
                    Task { await appState.checkAPIHealth() }
                }
                .keyboardShortcut("t", modifiers: [.command, .shift])
            }
            CommandGroup(replacing: .toolbar) {
                Button("Toggle Start/Stop") {
                    if appState.isWatching { appState.stop() } else { appState.start() }
                }
                .keyboardShortcut(.space, modifiers: [])
            }
        }
```

Note: `⌘1` / `⌘2` for tab switching are not added in this step because `TabView` with `selection` does not honor those out of the box without extra plumbing. Document this limitation in Task 7.2 verification.

- [ ] **Step 2: Build**

Run: `swift build`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add VibeShot/VibeShotApp.swift
git commit -m "feat(shell): add space-to-toggle Start/Stop shortcut"
```

---

## Phase 7 — Accessibility, polish, verification

### Task 7.1: Add Reduce Motion conformance check

The HeroCard already honors `accessibilityReduceMotion` (typing disabled). Confirm StageProgress and FiledColumn animations are also conditional.

**Files:**
- Modify: `VibeShot/Views/Editorial/StageProgress.swift`
- Modify: `VibeShot/Views/Editorial/FiledColumn.swift`

- [ ] **Step 1: Update StageProgress.swift to skip animation under Reduce Motion**

Inside the `StageProgress` struct, add the environment property at the top:

```swift
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
```

Then change the rectangle's animation modifier from:

```swift
                    .animation(.easeInOut(duration: 0.24), value: stage)
```

to:

```swift
                    .animation(reduceMotion ? nil : .easeInOut(duration: 0.24), value: stage)
```

- [ ] **Step 2: Update FiledColumn.swift filter-chip and search-focus animations**

Add the environment property at the top of `FiledColumn`:

```swift
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
```

Replace the chip underline animation:

```swift
                    .animation(.easeInOut(duration: 0.16), value: isActive)
```

with:

```swift
                    .animation(reduceMotion ? nil : .easeInOut(duration: 0.16), value: isActive)
```

And the search-focus rule animation:

```swift
                .animation(.easeInOut(duration: 0.14), value: searchFocused)
```

with:

```swift
                .animation(reduceMotion ? nil : .easeInOut(duration: 0.14), value: searchFocused)
```

- [ ] **Step 3: Build**

Run: `swift build`
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add VibeShot/Views/Editorial/StageProgress.swift VibeShot/Views/Editorial/FiledColumn.swift
git commit -m "feat(a11y): honor Reduce Motion in stage and chip animations"
```

---

### Task 7.2: Run the full test suite and build the app

**Files:**
- None modified.

- [ ] **Step 1: Run the test suite**

Run: `swift test`
Expected: all tests pass (SmokeTests, SessionMetaTests, TokenizerTests). 10 tests total expected.

- [ ] **Step 2: Build the .app bundle**

Run: `./build-app.sh`
Expected: prints `Built: .build/VibeShot.app`.

- [ ] **Step 3: Launch the app**

Run: `open .build/VibeShot.app`
Expected: window opens at 880×620 with the editorial masthead at the top. Tabs read "Dashboard" and "Pipeline."

- [ ] **Step 4: Manual verification checklist**

Verify each item below by interacting with the running app. Tick each box only after observing the behavior.

- [ ] Masthead reads `VIBESHOT · DAILY EDITION · {date} · NO. {n}` on both tabs.
- [ ] Dashboard idle headline reads *"Quietly Watching."* in italic serif.
- [ ] Dashboard hero card shows shutter glyph + reassuring copy when idle.
- [ ] Configure an API key in Settings, set watch dir, click "Start Watching" — headline changes to "Watching for screenshots."
- [ ] Take a screenshot in the watch dir — hero card shows thumbnail, slug appears letter-by-letter, stage progress moves through Analyzing → Renaming.
- [ ] After processing, "Today's Vocabulary" appears in the right column (requires ≥1 successful slug).
- [ ] Sparklines (Fig. 1/2/3) render once ≥2 latency data points exist.
- [ ] Switch to Pipeline tab — masthead persists, four columns visible.
- [ ] Take another screenshot — card appears briefly in Caught/Reading/Setting columns with active-column hairline turning coral.
- [ ] Card lands in Filed column. Double-click opens the file. Right-click → Reveal in Finder works.
- [ ] Search field filters Filed cards. Filter chips (ALL · ERRORS · TODAY) toggle correctly.
- [ ] Press Space (no text field focused) — Start/Stop toggles.
- [ ] Resize window to minimum — layout still readable at 760×540, no clipping.

- [ ] **Step 5: Commit no-op if all checks pass; otherwise fix and commit fixes**

If any checklist item failed, file an issue or fix it in a follow-up commit with subject like `fix(dashboard): <issue>`.

If everything passes, no commit needed for this step.

---

## Self-Review (planner side)

Performed by the planner before handing off.

**Spec coverage check:**

| Spec section | Plan tasks |
|---|---|
| §1 Goal | All tasks |
| §2 Non-goals | Implicit — no service tasks |
| §3 IA (Dashboard + Pipeline) | T4.3, T5.4, T6.1 |
| §4.1 Masthead | T2.2 |
| §4.2 Window sizing | T6.1, T6.2 |
| §4.3 Tab labels/icons | T6.1 |
| §4.4 Tab crossfade | Implicit via `TabView`; not explicitly forced — `TabView` provides default crossfade. **Acceptable**. |
| §5.1 Headline strip | T4.3 |
| §5.2.1 Hero card | T4.2, T4.3 |
| §5.2.2 Right column blocks | T4.3 (incl. sparklines, vocab, numbers, directories) |
| §5.3 Colophon | T4.3 |
| §6.1 Columns | T5.4 |
| §6.2 Column headers + active hairline | T5.2, T5.4 |
| §6.3 Empty state | T5.2 (EmptyColumnDash) |
| §6.4 Card design | T5.1 |
| §6.5 Filed extras | T5.3 |
| §6.6 Reads-when-idle | T5.4 (uses appState directly) |
| §7 Motion | T4.1, T4.2, T5.3, T7.1 |
| §7.9 Reduce Motion | T4.2, T7.1 |
| §8 Design system | T1.1, T1.2, T1.3, T1.4, T2.x |
| §9 Keyboard | T6.3 (partial — Cmd-1/Cmd-2 deferred, documented as known limitation) |
| §10 State | T3.3 |
| §11 File plan | All Phase 1–6 tasks |
| §12 Build sequence | Phases mirror spec build sequence |
| §13 Risk: matchedGeometryEffect | T5.4 uses a single ID and column-level conditional rendering (no glide across columns; cards fade in/out per column). **Deviation from spec § 7.1 noted: cards snap-switch columns rather than glide.** |
| §13 Risk: vocabulary false positives | Stopword list in T3.1 covers common cases. |
| §14 Acceptance criteria | Mapped to T7.2 manual checklist. |

**Identified deviations from spec, justified:**

1. **Paper grain:** Spec called for a 256×256 PNG asset; the plan ships a procedural Canvas-based noise (T1.3). Functionally equivalent, simpler to ship, no binary asset.
2. **matchedGeometryEffect glide between columns:** Spec § 7.1 described `matchedGeometryEffect` gliding cards between columns. The plan uses a single in-flight card whose column placement is derived from `appState.currentStage`, with fade transitions per column. This avoids SwiftUI rendering quirks when a `matchedGeometryEffect` target crosses into a `ScrollView`. Visually the card still appears in the correct column at the correct moment, with the active-column hairline as a calmer indicator of pipeline position.
3. **Cmd-1 / Cmd-2 tab switching:** Not added in T6.3 because SwiftUI's macOS `TabView` doesn't expose `selection` to the command system cleanly without state-lifting plumbing. Documented as a known limitation in T7.2; can be added in a follow-up if desired.

**Placeholder scan:** None remain. All steps contain complete code.

**Type consistency:** Spot-checked function signatures (`SessionMeta(defaultsKey:)`, `Tokenizer.topNouns(from:limit:)`, `StageProgress.init(stage:includesClipboard:)`, `HeroCard.init(stage:currentFile:thumbnailPath:proposedSlug:elapsedSeconds:includesClipboard:)`, `PipelineCard.init(file:isInFlight:)`, `FiledColumn.init(files:searchText:filter:selection:searchFocused:)`). All consistent between definition and call sites.
