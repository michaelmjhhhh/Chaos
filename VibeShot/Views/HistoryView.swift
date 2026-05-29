import SwiftUI
import AppKit

struct HistoryView: View {
    @Environment(AppState.self) private var appState
    @State private var errorsOnly = false
    @State private var selection: RecentFile.ID?

    private var filtered: [RecentFile] {
        errorsOnly ? appState.recentFiles.filter(\.isError) : appState.recentFiles
    }

    var body: some View {
        ZStack {
            Theme.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                HStack(alignment: .center) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("History")
                            .font(Theme.displayLg)
                            .foregroundStyle(Theme.ink)
                            .tracking(-0.5)

                        Text("\(appState.recentFiles.count) file\(appState.recentFiles.count == 1 ? "" : "s") processed")
                            .font(Theme.bodySm)
                            .foregroundStyle(Theme.textSoft)
                    }

                    Spacer()

                    HStack(spacing: 6) {
                        Text("Errors only")
                            .font(Theme.bodySm)
                            .foregroundStyle(Theme.textMuted)
                        Toggle("", isOn: $errorsOnly)
                            .toggleStyle(.switch)
                            .controlSize(.mini)
                            .labelsHidden()
                    }
                }
                .padding(.horizontal, 24)
                .padding(.vertical, 16)

                Divider().overlay(Theme.divider)

                if filtered.isEmpty {
                    emptyView
                } else {
                    tableView
                }
            }
        }
        .frame(minWidth: 520, minHeight: 420)
    }

    @ViewBuilder
    private var emptyView: some View {
        VStack(spacing: 16) {
            ZStack {
                Circle()
                    .fill(Theme.surfaceMuted)
                    .frame(width: 64, height: 64)
                Image(systemName: errorsOnly ? "checkmark" : "photo.on.rectangle")
                    .font(.system(size: 24, weight: .light))
                    .foregroundStyle(Theme.textSoft)
            }

            VStack(spacing: 4) {
                Text(errorsOnly ? "No errors" : "No files yet")
                    .font(Theme.displaySm)
                    .foregroundStyle(Theme.textMuted)
                    .tracking(-0.2)

                Text(errorsOnly ? "All screenshots processed successfully." : "Start watching to capture and rename screenshots.")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textSoft)
                    .multilineTextAlignment(.center)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(40)
    }

    @ViewBuilder
    private var tableView: some View {
        Table(filtered, selection: $selection) {
            TableColumn("Time") { f in
                Text(f.timestamp, format: .dateTime.hour().minute().second())
                    .font(Theme.codeSm)
                    .foregroundStyle(Theme.textMuted)
            }
            .width(min: 65, ideal: 72)

            TableColumn("Original") { f in
                Text(f.originalName)
                    .font(Theme.body)
                    .foregroundStyle(Theme.textBody)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            .width(min: 100, ideal: 160)

            TableColumn("Renamed") { f in
                Text(f.newName.isEmpty ? "—" : f.newName)
                    .font(Theme.body)
                    .foregroundStyle(f.newName.isEmpty ? Theme.textSoft : Theme.textBody)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            .width(min: 100, ideal: 180)

            TableColumn("Status") { f in
                HStack(spacing: 5) {
                    Circle()
                        .fill(f.isError ? Theme.error : Theme.success)
                        .frame(width: 6, height: 6)
                    Text(f.isError ? "error" : "ok")
                        .font(Theme.caption)
                        .foregroundStyle(f.isError ? Theme.error : Theme.success)
                }
            }
            .width(min: 50, ideal: 60)

            TableColumn("Duration") { f in
                Text(f.duration > 0 ? String(format: "%.1fs", f.duration) : "—")
                    .font(Theme.codeSm)
                    .foregroundStyle(Theme.textSoft)
            }
            .width(min: 50, ideal: 60)
        }
        .tableStyle(.inset)
        .contextMenu(forSelectionType: RecentFile.ID.self) { ids in
            if let id = ids.first,
               let file = appState.recentFiles.first(where: { $0.id == id }),
               !file.path.isEmpty {
                Button("Reveal in Finder") {
                    NSWorkspace.shared.activateFileViewerSelecting(
                        [URL(fileURLWithPath: file.path)]
                    )
                }
            }
        } primaryAction: { ids in
            if let id = ids.first,
               let file = appState.recentFiles.first(where: { $0.id == id }),
               !file.path.isEmpty {
                NSWorkspace.shared.activateFileViewerSelecting(
                    [URL(fileURLWithPath: file.path)]
                )
            }
        }
    }
}
