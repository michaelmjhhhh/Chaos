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

    private let liveColumnWidth: CGFloat = 140

    var body: some View {
        ZStack(alignment: .top) {
            PaperBackground()

            VStack(spacing: 0) {
                Masthead(sessionNumber: appState.session.sessionNumber, date: Date())

                VStack(alignment: .leading, spacing: Theme.sMed) {
                    headerRow
                    boardBody
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                }
                .padding(.horizontal, Theme.sLg)
                .padding(.top, Theme.sLg)
                .padding(.bottom, Theme.sLg)
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
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
    private var headerRow: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            HStack(spacing: Theme.sMed) {
                headerLabel("CAUGHT", isActive: activeColumn == .caught)
                    .frame(width: liveColumnWidth, alignment: .leading)
                headerLabel("READING", isActive: activeColumn == .reading)
                    .frame(width: liveColumnWidth, alignment: .leading)
                headerLabel("SETTING", isActive: activeColumn == .setting)
                    .frame(width: liveColumnWidth, alignment: .leading)
                headerLabel("FILED", isActive: false)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            EditorialRule()
        }
    }

    @ViewBuilder
    private func headerLabel(_ title: String, isActive: Bool) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title).smallCaps().foregroundStyle(Theme.ink)
            Rectangle()
                .fill(isActive ? Theme.coral : Color.clear)
                .frame(height: 1)
                .animation(reduceMotion ? nil : .easeInOut(duration: 0.2), value: isActive)
        }
    }

    @ViewBuilder
    private var boardBody: some View {
        HStack(alignment: .top, spacing: Theme.sMed) {
            liveCardArea(active: activeColumn == .caught)
                .frame(width: liveColumnWidth, alignment: .top)
            liveCardArea(active: activeColumn == .reading)
                .frame(width: liveColumnWidth, alignment: .top)
            liveCardArea(active: activeColumn == .setting)
                .frame(width: liveColumnWidth, alignment: .top)

            FiledColumn(
                files: appState.recentFiles,
                searchText: $searchText,
                filter: $filter,
                selection: $selection,
                searchFocused: $searchFocused
            )
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
    }

    @ViewBuilder
    private func liveCardArea(active: Bool) -> some View {
        if active, let card = inFlightCard {
            PipelineCard(file: card, isInFlight: true)
                .matchedGeometryEffect(id: "inFlight", in: pipelineNS)
                .transition(.opacity)
        } else {
            Text("—")
                .font(.system(size: 16, design: .serif))
                .foregroundStyle(Theme.ink.opacity(0.3))
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.leading, 2)
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
