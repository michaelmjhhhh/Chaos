import SwiftUI
import AppKit

struct PipelineView: View {
    @Environment(AppState.self) private var appState

    @State private var searchText: String = ""
    @State private var filter: FiledColumn.Filter = .all
    @State private var selection: RecentFile.ID?
    @FocusState private var searchFocused: Bool

    @State private var renameTarget: RecentFile?
    @State private var renameText: String = ""

    var body: some View {
        ZStack(alignment: .top) {
            PaperBackground()

            VStack(spacing: 0) {
                Masthead(status: appState.watcherStatus, date: Date())

                VStack(alignment: .leading, spacing: Theme.sMed) {
                    if let live = liveLine {
                        liveStrip(live)
                    }

                    FiledColumn(
                        files: appState.recentFiles,
                        searchText: $searchText,
                        filter: $filter,
                        selection: $selection,
                        searchFocused: $searchFocused,
                        onRetry: appState.retry,
                        onRevert: appState.revert,
                        onRename: beginRename
                    )
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                }
                .padding(.horizontal, Theme.sLg)
                .padding(.top, Theme.sMed)
                .padding(.bottom, Theme.sLg)
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            }
        }
        .frame(minWidth: 760, minHeight: 540)
        .sheet(item: $renameTarget) { file in
            renameSheet(for: file)
        }
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

    // MARK: - Live strip

    /// A single calm line describing what's happening right now, shown only while a file
    /// is in flight. Replaces the three mostly-empty kanban columns.
    private var liveLine: (stage: String, file: String)? {
        guard let stage = appState.currentStage, let file = appState.currentFile else { return nil }
        switch stage {
        case .caught: return ("Caught", file)
        case .analyzing: return ("Reading", file)
        case .renaming, .clipboard: return ("Filing", file)
        case .success, .error: return nil
        }
    }

    @ViewBuilder
    private func liveStrip(_ live: (stage: String, file: String)) -> some View {
        HStack(spacing: Theme.sSmall) {
            ProgressView().controlSize(.small)
            Text(live.stage.uppercased())
                .font(Theme.captionSm)
                .tracking(1)
                .foregroundStyle(Theme.coral)
            Text(live.file)
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textMuted)
                .lineLimit(1)
                .truncationMode(.middle)
            Spacer()
        }
        .padding(.horizontal, Theme.sMed)
        .padding(.vertical, Theme.sSmall)
        .background(Theme.surfaceMuted)
        .clipShape(.rect(cornerRadius: Theme.r8))
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(live.stage) \(live.file)")
    }

    // MARK: - Rename

    private func beginRename(_ file: RecentFile) {
        let current = file.newName.isEmpty ? file.originalName : file.newName
        renameText = (current as NSString).deletingPathExtension
        renameTarget = file
    }

    @ViewBuilder
    private func renameSheet(for file: RecentFile) -> some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            Text("Rename file")
                .font(Theme.displayMd)
                .foregroundStyle(Theme.warmInk)

            Text("Give this screenshot a clearer name. The file extension stays the same.")
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textMuted)

            TextField("New name", text: $renameText)
                .textFieldStyle(.roundedBorder)
                .font(Theme.code)
                .onSubmit { commitRename(file) }

            HStack {
                Spacer()
                Button("Cancel") { renameTarget = nil }
                    .keyboardShortcut(.cancelAction)
                Button("Rename") { commitRename(file) }
                    .keyboardShortcut(.defaultAction)
                    .disabled(renameText.trimmingCharacters(in: .whitespaces).isEmpty)
            }
        }
        .padding(Theme.sLg)
        .frame(width: 380)
        .background(Theme.canvas)
    }

    private func commitRename(_ file: RecentFile) {
        appState.rename(file, to: renameText)
        renameTarget = nil
    }
}
