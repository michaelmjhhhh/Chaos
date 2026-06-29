import SwiftUI
import AppKit

/// Formatters and calendar are expensive to allocate, so cache them once rather
/// than rebuilding them every time the history list re-renders.
private enum FiledFormatters {
    static let calendar = Calendar.current
    static let dayMonth: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "d MMM"
        return f
    }()
}

struct FiledColumn: View {
    let files: [RecentFile]
    @Binding var searchText: String
    @Binding var filter: Filter
    @Binding var selection: RecentFile.ID?
    @FocusState.Binding var searchFocused: Bool
    var onRetry: (RecentFile) -> Void = { _ in }
    var onRevert: (RecentFile) -> Void = { _ in }
    var onRename: (RecentFile) -> Void = { _ in }

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// Search is applied from a debounced, pre-lowercased copy of `searchText`
    /// so filtering doesn't run on every keystroke.
    @State private var debouncedQuery: String = ""
    @State private var debounceTask: Task<Void, Never>?

    enum Filter: String, CaseIterable {
        case all = "ALL"
        case errors = "ERRORS"
        case today = "TODAY"
    }

    private var visible: [RecentFile] {
        let cal = FiledFormatters.calendar
        let today = cal.startOfDay(for: Date())
        let q = debouncedQuery
        return files.filter { file in
            switch filter {
            case .all: break
            case .errors: if !file.isError { return false }
            case .today: if cal.startOfDay(for: file.timestamp) != today { return false }
            }
            if !q.isEmpty, !file.searchKey.contains(q) { return false }
            return true
        }
    }

    private var grouped: [(label: String, items: [RecentFile])] {
        let cal = FiledFormatters.calendar
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

        let df = FiledFormatters.dayMonth

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
            if !files.isEmpty {
                controlBar
                    .padding(.bottom, Theme.sMed)
            }
            content
        }
        .onChange(of: searchText) { _, newValue in
            debounceTask?.cancel()
            // Clearing the field should feel instant; only debounce typing.
            if newValue.isEmpty {
                debouncedQuery = ""
                return
            }
            let query = newValue.lowercased()
            debounceTask = Task { @MainActor in
                try? await Task.sleep(for: .milliseconds(150))
                if !Task.isCancelled { debouncedQuery = query }
            }
        }
    }

    private var controlBar: some View {
        HStack(spacing: Theme.sMed) {
            searchField
            Spacer()
            filterChips
        }
    }

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
                .accessibilityLabel("Search filings")
            Rectangle()
                .fill(searchFocused ? Theme.coral : Theme.rule)
                .frame(height: searchFocused ? 1 : 0.5)
                .animation(reduceMotion ? nil : .easeInOut(duration: 0.14), value: searchFocused)
        }
    }

    private var filterChips: some View {
        HStack(spacing: Theme.sMed) {
            ForEach(Filter.allCases, id: \.self) { f in
                chip(label: f.rawValue, isActive: filter == f) {
                    filter = f
                }
            }
        }
    }

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
                    .animation(reduceMotion ? nil : .easeInOut(duration: 0.16), value: isActive)
            }
        }
        .buttonStyle(.plain)
    }

    @ViewBuilder
    private var content: some View {
        if visible.isEmpty {
            Text(emptyMessage)
                .font(Theme.serifItalicLg)
                .foregroundStyle(Theme.textMuted)
                .padding(.top, Theme.sLg)
                .frame(maxWidth: .infinity, alignment: .center)
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

    private var emptyMessage: String {
        if files.isEmpty { return "No filings yet." }
        if filter == .errors { return "No errors filed." }
        if filter == .today { return "Nothing filed today." }
        return "No matches."
    }

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
                if file.isError {
                    Button("Retry") {
                        onRetry(file)
                    }
                }
                if !file.isError, !file.path.isEmpty {
                    Button("Rename…") {
                        onRename(file)
                    }
                    if !file.sourcePath.isEmpty {
                        Button("Undo (restore original name)") {
                            onRevert(file)
                        }
                    }
                }
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

#Preview("Empty") {
    struct PreviewWrapper: View {
        @State var search = ""
        @State var filter: FiledColumn.Filter = .all
        @State var selection: RecentFile.ID? = nil
        @FocusState var focus: Bool

        var body: some View {
            FiledColumn(
                files: [],
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

#Preview("With data") {
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
