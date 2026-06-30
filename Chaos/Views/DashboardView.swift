import SwiftUI
import AppKit
import UniformTypeIdentifiers

struct DashboardView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.openSettings) private var openSettings
    @State private var ellipsisCount = 0

    var body: some View {
        ZStack {
            PaperBackground()

            VStack(spacing: 0) {
                Masthead(status: appState.watcherStatus, date: Date())

                ScrollView {
                    VStack(alignment: .leading, spacing: Theme.sSec) {
                        headlineStrip
                        if let err = appState.lastError {
                            errorBanner(err)
                        }
                        bodyColumns
                    }
                    .padding(.horizontal, Theme.sSec)
                    .padding(.vertical, Theme.sLg)
                }

                colophon
            }
        }
        .frame(minWidth: 520, minHeight: 540)
        .onAppear {
            Timer.scheduledTimer(withTimeInterval: 0.6, repeats: true) { _ in
                ellipsisCount = (ellipsisCount + 1) % 4
            }
        }
    }

    private var headlineStrip: some View {
        HStack(alignment: .center) {
            headlineText
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
        case 1: "."
        case 2: ".."
        case 3: "..."
        default: ""
        }
    }

    @ViewBuilder
    private var controlButton: some View {
        if appState.isWatching {
            Button { appState.stop() } label: {
                HStack(spacing: 6) {
                    Image(systemName: "stop.fill").font(.system(size: 8))
                    Text("Stop").font(Theme.button)
                }
                .foregroundStyle(Theme.onBrand)
                .padding(.horizontal, 16)
                .padding(.vertical, 7)
                .background(Theme.error)
                .clipShape(.rect(cornerRadius: Theme.r6))
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Stop watching for screenshots")
        } else {
            Button { appState.start() } label: {
                HStack(spacing: 6) {
                    Image(systemName: "play.fill").font(.system(size: 9))
                    Text("Start Watching").font(Theme.button)
                }
                .foregroundStyle(Theme.onBrand)
                .padding(.horizontal, 18)
                .padding(.vertical, 7)
                .background(Theme.coral)
                .clipShape(.rect(cornerRadius: Theme.r6))
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Start watching for screenshots")
        }
    }

    /// A plain-language, actionable banner for the most recent failure. Gives a
    /// non-technical user the next step (retry / open settings) instead of a stack trace.
    private func errorBanner(_ error: FriendlyError) -> some View {
        HStack(alignment: .top, spacing: Theme.sSmall) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(Theme.error)

            Text(error.message)
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textBody)
                .fixedSize(horizontal: false, vertical: true)

            Spacer(minLength: Theme.sSmall)

            if let label = error.action.label {
                Button(label) { handleRecovery(error.action) }
                    .controlSize(.small)
            }

            Button {
                appState.lastError = nil
            } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 10))
                    .foregroundStyle(Theme.textSoft)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Dismiss")
        }
        .padding(Theme.sMed)
        .background(Theme.error.opacity(0.08))
        .clipShape(.rect(cornerRadius: Theme.r8))
        .overlay {
            RoundedRectangle(cornerRadius: Theme.r8).stroke(Theme.error.opacity(0.25), lineWidth: 0.5)
        }
    }

    private func handleRecovery(_ action: RecoveryAction) {
        switch action {
        case .openSettings:
            openSettings()
        case .retry, .checkInternet:
            if let failed = appState.recentFiles.first(where: { $0.isError }) {
                appState.retry(failed)
            }
            appState.lastError = nil
        case .none:
            appState.lastError = nil
        }
    }

    /// Two columns when there's room; falls back to a single stacked column on narrow
    /// widths (e.g. a tiled window on a 13" MacBook). Giving the wide hero a real
    /// minWidth lets ViewThatFits pick the wide layout only when it actually fits.
    private var bodyColumns: some View {
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .top, spacing: 28) {
                heroColumn
                    .frame(minWidth: 360, maxWidth: .infinity)
                    .layoutPriority(2)
                editorialColumn
                    .frame(width: 280)
            }

            VStack(alignment: .leading, spacing: Theme.sLg) {
                heroColumn
                editorialColumn
            }
        }
    }

    private var heroColumn: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            HeroCard(
                stage: appState.currentStage,
                currentFile: appState.currentFile,
                thumbnailPath: latestThumbnailPath,
                proposedSlug: proposedSlug,
                elapsedSeconds: heroElapsed,
                includesClipboard: appState.resolvedCopyToClipboard,
                isWatching: appState.isWatching,
                hasFiledBefore: !appState.recentFiles.isEmpty
            )
            .dropDestination(for: URL.self) { urls, _ in
                appState.processManualURLs(urls)
                return !ImageIntake.acceptedURLs(from: urls).isEmpty
            }
            .accessibilityHint("Drop images here to rename them")

            Button(action: organizeExistingScreenshots) {
                Label("Organize Existing Screenshots", systemImage: "photo.on.rectangle.angled")
                    .font(Theme.button)
            }
            .buttonStyle(.bordered)
            .tint(Theme.coral)
        }
    }

    private func organizeExistingScreenshots() {
        let panel = NSOpenPanel()
        panel.title = "Organize Existing Screenshots"
        panel.message = "Choose images to rename and move into your Chaos output folder."
        panel.prompt = "Organize"
        panel.allowsMultipleSelection = true
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [.png, .jpeg, .heic, .webP]

        guard panel.runModal() == .OK else { return }
        appState.processManualURLs(panel.urls)
    }

    private var latestThumbnailPath: String? {
        appState.recentFiles.first { !$0.isError && !$0.path.isEmpty }?.path
    }

    private var proposedSlug: String? {
        switch appState.currentStage {
        case .success(let name): name
        default: appState.currentFile
        }
    }

    private var heroElapsed: TimeInterval {
        guard let last = appState.recentFiles.first else { return 0 }
        return last.duration
    }

    private var editorialColumn: some View {
        VStack(alignment: .leading, spacing: Theme.sLg) {
            todayBlock
            EditorialRule()
            foldersBlock
        }
    }

    private var todayBlock: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text("TODAY").smallCaps().foregroundStyle(Theme.textMuted)
            HStack(spacing: Theme.sLg) {
                MetricFigure(value: "\(appState.successes)", label: "Filed")
                MetricFigure(
                    value: "\(appState.errors)",
                    label: "Errors",
                    accent: appState.errors > 0 ? Theme.error : nil
                )
            }
            if appState.avgLatency > 0 {
                Text("Avg \(fmtDur(appState.avgLatency)) per image")
                    .font(Theme.codeSm)
                    .foregroundStyle(Theme.textSoft)
            }
        }
    }

    private var foldersBlock: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            Text("FOLDERS").smallCaps().foregroundStyle(Theme.textMuted)
            FolderRoute(
                watchPath: appState.resolvedWatchDir,
                outputPath: appState.resolvedOutputDir,
                isWatching: appState.isWatching
            )
        }
    }

    private var colophon: some View {
        VStack(spacing: 0) {
            EditorialRule()
            HStack(spacing: Theme.sSmall) {
                Circle().fill(apiColor).frame(width: 5, height: 5)
                Text("\(appState.resolvedProvider.displayName) · \(appState.resolvedModel)")
                    .font(Theme.captionSm)
                    .tracking(0.4)
                    .foregroundStyle(Theme.textSoft)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Spacer()
                Button("Settings") { openSettings() }
                    .buttonStyle(.plain)
                    .font(Theme.captionSm)
                    .foregroundStyle(Theme.coral)
            }
            .padding(.horizontal, Theme.sSec)
            .padding(.vertical, Theme.sSmall)
        }
    }

    private var apiColor: Color {
        switch appState.apiStatus {
        case "OK": Theme.success
        case "FAIL": Theme.error
        default: Theme.textSoft
        }
    }

    private func fmtDur(_ s: TimeInterval) -> String {
        if s <= 0 { return "—" }
        if s < 1 { return String(format: "%.0fms", s * 1000) }
        return String(format: "%.1fs", s)
    }
}
