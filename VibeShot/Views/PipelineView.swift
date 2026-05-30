import SwiftUI
import AppKit

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
}
