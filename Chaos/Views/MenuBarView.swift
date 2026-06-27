import SwiftUI
import AppKit

struct MenuBarView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.openSettings) private var openSettings

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text("Chaos")
                    .font(.system(size: 14, weight: .regular, design: .serif))
                    .tracking(-0.2)

                Spacer()

                if appState.isWatching {
                    pillBtn("Stop", Theme.error) { appState.stop() }
                } else {
                    pillBtn("Start", Theme.coral) { appState.start() }
                }
            }

            if case .error(let m) = appState.watcherStatus {
                Text(m)
                    .font(Theme.codeSm)
                    .foregroundStyle(Theme.error)
                    .padding(.top, 6)
            }

            sep

            if appState.recentFiles.isEmpty {
                Text("No recent files")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textSoft)
                    .padding(.vertical, 4)
            } else {
                ForEach(appState.recentFiles.prefix(5)) { f in
                    Button {
                        guard !f.path.isEmpty else { return }
                        NSWorkspace.shared.activateFileViewerSelecting(
                            [URL(fileURLWithPath: f.path)]
                        )
                    } label: {
                        HStack(spacing: 6) {
                            Circle()
                                .fill(f.isError ? Theme.error : Theme.success)
                                .frame(width: 5, height: 5)
                            Text(f.newName.isEmpty ? f.originalName : f.newName)
                                .font(Theme.bodySm)
                                .foregroundStyle(Theme.textBody)
                                .lineLimit(1)
                                .truncationMode(.middle)
                            Spacer()
                        }
                        .contentShape(Rectangle())
                        .padding(.vertical, 3)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("\(f.isError ? "Failed" : "Filed"): \(f.newName.isEmpty ? f.originalName : f.newName)")
                }
            }

            sep

            menuLink("folder", "Open Output") {
                NSWorkspace.shared.open(URL(fileURLWithPath: appState.resolvedOutputDir))
            }
            menuLink("gear", "Settings…") {
                openSettings()
            }
            menuLink("questionmark.circle", "Help & Getting Started") {
                NSApp.activate(ignoringOtherApps: true)
                appState.showHelp = true
            }

            sep

            Text("\(appState.resolvedProvider.displayName) · \(appState.resolvedModel)")
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textSoft)
                .padding(.vertical, 3)

            sep

            Button {
                appState.stop()
                NSApp.terminate(nil)
            } label: {
                Text("Quit Chaos")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textMuted)
            }
            .buttonStyle(.plain)
            .keyboardShortcut("q")
        }
        .padding(14)
        .frame(width: 220)
    }

    @ViewBuilder
    private func pillBtn(_ label: String, _ bg: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(label)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(.white)
                .padding(.horizontal, 10)
                .padding(.vertical, 3)
                .background(bg)
                .clipShape(.rect(cornerRadius: Theme.r4))
        }
        .buttonStyle(.plain)
    }

    @ViewBuilder
    private func menuLink(_ icon: String, _ label: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 6) {
                Image(systemName: icon)
                    .font(.system(size: 10))
                    .foregroundStyle(Theme.textSoft)
                    .frame(width: 14)
                Text(label)
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textBody)
            }
        }
        .buttonStyle(.plain)
        .padding(.vertical, 3)
    }

    private var sep: some View {
        Divider().opacity(0.3).padding(.vertical, 5)
    }
}

struct MenuBarIcon: View {
    let status: WatcherStatus

    var body: some View {
        icon.accessibilityLabel("Chaos — \(statusLabel)")
    }

    @ViewBuilder
    private var icon: some View {
        switch status {
        case .stopped:
            Image(systemName: "camera.viewfinder")
        case .starting:
            Image(systemName: "arrow.triangle.2.circlepath")
        case .running:
            Image(systemName: "camera.viewfinder")
                .symbolRenderingMode(.palette)
        case .error:
            Image(systemName: "exclamationmark.triangle")
        }
    }

    private var statusLabel: String {
        switch status {
        case .stopped: "not watching"
        case .starting: "starting"
        case .running: "watching"
        case .error: "needs attention"
        }
    }
}
