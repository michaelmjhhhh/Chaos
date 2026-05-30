import SwiftUI

struct DashboardView: View {
    @Environment(AppState.self) private var appState
    @State private var uptimeTick = false

    var body: some View {
        ZStack {
            Theme.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                headerBar
                Divider().overlay(Theme.divider)

                ScrollView {
                    VStack(alignment: .leading, spacing: 24) {
                        statusRow
                        metricsGrid
                        directoriesRow
                        recentActivityPreview
                    }
                    .padding(24)
                }

                Divider().overlay(Theme.divider)
                footerBar
            }
        }
        .frame(minWidth: 520, minHeight: 420)
        .onAppear {
            Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { _ in
                uptimeTick.toggle()
            }
        }
    }

    // MARK: - Header

    @ViewBuilder
    private var headerBar: some View {
        HStack(alignment: .center) {
            VStack(alignment: .leading, spacing: 2) {
                Text("Dashboard")
                    .font(Theme.displayLg)
                    .foregroundStyle(Theme.ink)
                    .tracking(-0.5)

                let _ = uptimeTick
                if appState.isWatching, let t = appState.watcherStartedAt {
                    Text("Running \(fmtUptime(t))")
                        .font(Theme.bodySm)
                        .foregroundStyle(Theme.textSoft)
                }
            }

            Spacer()

            if appState.isWatching {
                Button {
                    appState.stop()
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "stop.fill")
                            .font(.system(size: 8))
                        Text("Stop")
                            .font(Theme.button)
                    }
                    .foregroundStyle(.white)
                    .padding(.horizontal, 16)
                    .padding(.vertical, 7)
                    .background(Theme.error)
                    .clipShape(.rect(cornerRadius: Theme.r6))
                }
                .buttonStyle(.plain)
            } else {
                Button {
                    appState.start()
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "play.fill")
                            .font(.system(size: 9))
                        Text("Start Watching")
                            .font(Theme.button)
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
        .padding(.horizontal, 24)
        .padding(.vertical, 16)
    }

    // MARK: - Status

    @ViewBuilder
    private var statusRow: some View {
        HStack(spacing: 12) {
            statusBadge
                .frame(width: 38, height: 38)

            VStack(alignment: .leading, spacing: 1) {
                Text(statusText)
                    .font(Theme.titleMd)
                    .foregroundStyle(statusColor)

                if let f = appState.currentFile {
                    Text(f)
                        .font(Theme.codeSm)
                        .foregroundStyle(Theme.textMuted)
                        .lineLimit(1)
                        .truncationMode(.middle)
                } else {
                    Text(statusSubtext)
                        .font(Theme.bodySm)
                        .foregroundStyle(Theme.textSoft)
                }
            }

            Spacer()
        }
        .card()
    }

    @ViewBuilder
    private var statusBadge: some View {
        let processing: Bool = {
            switch appState.currentStage {
            case .analyzing, .renaming, .clipboard: true
            default: false
            }
        }()

        switch appState.watcherStatus {
        case .running where processing:
            ProgressView()
                .controlSize(.small)
                .frame(width: 38, height: 38)
                .background(Theme.surfaceMuted)
                .clipShape(Circle())
        case .running:
            Image(systemName: "eye.fill")
                .font(.system(size: 14))
                .foregroundStyle(Theme.success)
                .frame(width: 38, height: 38)
                .background(Theme.success.opacity(0.1))
                .clipShape(Circle())
        case .error:
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.system(size: 13))
                .foregroundStyle(Theme.error)
                .frame(width: 38, height: 38)
                .background(Theme.error.opacity(0.1))
                .clipShape(Circle())
        case .starting:
            ProgressView()
                .controlSize(.small)
                .frame(width: 38, height: 38)
        case .stopped:
            Image(systemName: "moon.fill")
                .font(.system(size: 13))
                .foregroundStyle(Theme.textSoft)
                .frame(width: 38, height: 38)
                .background(Theme.surfaceMuted)
                .clipShape(Circle())
        }
    }

    private var statusText: String {
        switch appState.watcherStatus {
        case .stopped: "Idle"
        case .starting: "Starting…"
        case .error(let m): m
        case .running:
            switch appState.currentStage {
            case .caught: "Caught a screenshot…"
            case .analyzing: "Analyzing screenshot…"
            case .renaming: "Renaming file…"
            case .clipboard: "Copying to clipboard…"
            case .success(let n): "Done — \(n)"
            case .error(let m): m
            case .none: "Watching for screenshots"
            }
        }
    }

    private var statusSubtext: String {
        switch appState.watcherStatus {
        case .stopped: "Press Start to begin watching"
        case .running: "Monitoring \(abbrev(appState.resolvedWatchDir))"
        default: ""
        }
    }

    private var statusColor: Color {
        switch appState.watcherStatus {
        case .stopped: Theme.textMuted
        case .starting: Theme.ink
        case .error: Theme.error
        case .running:
            switch appState.currentStage {
            case .error: Theme.error
            case .success: Theme.success
            case .caught, .analyzing, .renaming, .clipboard: Theme.ink
            case .none: Theme.ink
            }
        }
    }

    // MARK: - Metrics

    @ViewBuilder
    private var metricsGrid: some View {
        HStack(spacing: 10) {
            metricCard("Processed", "\(appState.totalProcessed)")
            metricCard("Success", "\(appState.successes)", accent: Theme.success)
            metricCard("Errors", "\(appState.errors)", accent: appState.errors > 0 ? Theme.error : nil)
        }

        HStack(spacing: 10) {
            metricCard("Success Rate", String(format: "%.0f%%", appState.successRate * 100))
            metricCard("Avg Latency", fmtDur(appState.avgLatency))
            metricCard("P95", fmtDur(appState.p95Latency))
        }
    }

    @ViewBuilder
    private func metricCard(_ label: String, _ value: String, accent: Color? = nil) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label)
                .font(Theme.captionSm)
                .foregroundStyle(Theme.textSoft)
                .textCase(.uppercase)
                .tracking(0.6)

            Text(value)
                .font(.system(size: 22, weight: .regular, design: .serif))
                .foregroundStyle(accent ?? Theme.ink)
                .monospacedDigit()
                .tracking(-0.3)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .card()
    }

    // MARK: - Directories

    @ViewBuilder
    private var directoriesRow: some View {
        HStack(spacing: 10) {
            dirCard("Watch", "eye", appState.resolvedWatchDir)
            dirCard("Output", "arrow.down.to.line", appState.resolvedOutputDir)
        }
    }

    @ViewBuilder
    private func dirCard(_ label: String, _ icon: String, _ path: String) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 5) {
                Image(systemName: icon)
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(Theme.textSoft)
                Text(label)
                    .font(Theme.captionSm)
                    .foregroundStyle(Theme.textSoft)
                    .textCase(.uppercase)
                    .tracking(0.6)
            }

            Text(abbrev(path))
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textMuted)
                .lineLimit(1)
                .truncationMode(.middle)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .card(padding: 14)
    }

    // MARK: - Recent Activity Preview

    @ViewBuilder
    private var recentActivityPreview: some View {
        if !appState.recentFiles.isEmpty {
            VStack(alignment: .leading, spacing: 10) {
                HStack {
                    Text("Recent Activity")
                        .sectionHead()
                    Spacer()
                }

                VStack(spacing: 0) {
                    ForEach(appState.recentFiles.prefix(3)) { f in
                        HStack(spacing: 8) {
                            Circle()
                                .fill(f.isError ? Theme.error : Theme.success)
                                .frame(width: 6, height: 6)

                            Text(f.newName.isEmpty ? f.originalName : f.newName)
                                .font(Theme.body)
                                .foregroundStyle(Theme.textBody)
                                .lineLimit(1)
                                .truncationMode(.middle)

                            Spacer()

                            if f.duration > 0 {
                                Text(String(format: "%.1fs", f.duration))
                                    .font(Theme.codeSm)
                                    .foregroundStyle(Theme.textSoft)
                            }

                            Text(f.timestamp, format: .dateTime.hour().minute().second())
                                .font(Theme.codeSm)
                                .foregroundStyle(Theme.textSoft)
                        }
                        .padding(.vertical, 8)
                        .padding(.horizontal, 14)

                        if f.id != appState.recentFiles.prefix(3).last?.id {
                            Divider().overlay(Theme.borderLight)
                        }
                    }
                }
                .background(Theme.surfaceCard)
                .clipShape(.rect(cornerRadius: Theme.r10))
                .overlay(
                    RoundedRectangle(cornerRadius: Theme.r10)
                        .stroke(Theme.border, lineWidth: 0.5)
                )
                .shadow(color: Theme.shadowCard, radius: 4, y: 2)
            }
        }
    }

    // MARK: - Footer

    @ViewBuilder
    private var footerBar: some View {
        HStack(spacing: 0) {
            Label(appState.resolvedProvider.displayName, systemImage: "server.rack")
                .font(Theme.captionSm)
                .foregroundStyle(Theme.textSoft)

            Text("  ·  ").foregroundStyle(Theme.borderLight)

            Text(appState.resolvedModel)
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textSoft)

            Text("  ·  ").foregroundStyle(Theme.borderLight)

            HStack(spacing: 4) {
                Circle()
                    .fill(apiColor)
                    .frame(width: 5, height: 5)
                Text("API \(appState.apiStatus)")
                    .font(Theme.captionSm)
                    .foregroundStyle(Theme.textSoft)
            }

            Spacer()
        }
        .padding(.horizontal, 24)
        .padding(.vertical, 8)
    }

    private var apiColor: Color {
        switch appState.apiStatus {
        case "OK": Theme.success
        case "FAIL": Theme.error
        default: Theme.textSoft
        }
    }

    // MARK: - Fmt

    private func fmtDur(_ s: TimeInterval) -> String {
        if s <= 0 { return "—" }
        if s < 1 { return String(format: "%.0fms", s * 1000) }
        return String(format: "%.1fs", s)
    }

    private func fmtUptime(_ d: Date) -> String {
        let i = Int(Date().timeIntervalSince(d))
        let h = i / 3600, m = (i % 3600) / 60, s = i % 60
        if h > 0 { return "\(h)h \(m)m" }
        return "\(m)m \(s)s"
    }

    private func abbrev(_ p: String) -> String {
        p.replacingOccurrences(of: NSHomeDirectory(), with: "~")
    }
}
