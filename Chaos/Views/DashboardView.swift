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
        .dropDestination(for: URL.self) { urls, _ in
            appState.processDroppedURLs(urls)
            return urls.contains(where: ImageIntake.accepts)
        }
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
        VStack(alignment: .leading, spacing: Theme.sSec) {
            todaysReadingBlock
            numbersBlock
            if !appState.vocabularyToday.isEmpty {
                vocabularyBlock
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
                caption: "Fig. 1 — Latency",
                lastValueText: appState.latencyHistory.last.map { String(format: "%.1fs", $0) }
            )
            Sparkline(
                values: appState.hourlyThroughput.map(Double.init),
                caption: "Fig. 2 — Throughput, hourly",
                lastValueText: nonZeroThroughputLabel
            )
            Sparkline(
                values: appState.successRateHistory,
                caption: "Fig. 3 — Success rate",
                lastValueText: appState.successRateHistory.last.map { String(format: "%.0f%%", $0 * 100) }
            )
        }
    }

    private var nonZeroThroughputLabel: String? {
        let total = appState.hourlyThroughput.reduce(0, +)
        guard total > 0 else { return nil }
        return "\(total) today"
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
