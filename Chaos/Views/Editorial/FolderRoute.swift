import SwiftUI

/// The FOLDERS block, drawn as a single editorial "route": screenshots are caught
/// at the watched folder (a live coral node) and travel down a hairline rule to land
/// in the output folder (a hollow terminus). Replaces two disconnected icons + raw
/// monospace paths with one device that encodes the actual pipeline, and a path
/// treatment that leads with the folder *name* a person recognizes.
struct FolderRoute: View {
    let watchPath: String
    let outputPath: String
    let isWatching: Bool
    /// Home directory, abbreviated to `~` for display. Injected for testability.
    var home: String = NSHomeDirectory()

    var body: some View {
        Grid(alignment: .leadingFirstTextBaseline,
             horizontalSpacing: 14,
             verticalSpacing: Theme.sMed) {
            GridRow {
                node(.origin)
                stop(role: "WATCH", path: watchPath)
            }
            GridRow {
                node(.terminus)
                stop(role: "OUTPUT", path: outputPath)
            }
        }
        .backgroundPreferenceValue(NodeAnchorKey.self) { anchors in
            GeometryReader { proxy in
                if anchors.count == 2 {
                    Path { path in
                        path.move(to: proxy[anchors[0]])
                        path.addLine(to: proxy[anchors[1]])
                    }
                    .stroke(Theme.rule, style: StrokeStyle(lineWidth: 1, dash: [1, 3]))
                }
            }
        }
    }

    // MARK: - Route nodes

    private enum Node { case origin, terminus }

    @ViewBuilder
    private func node(_ kind: Node) -> some View {
        Group {
            switch kind {
            case .origin:
                // The live source: coral while watching, otherwise resting.
                Circle().fill(isWatching ? Theme.coral : Theme.textSoft)
            case .terminus:
                // The destination: an open ring waiting to be filled.
                Circle().strokeBorder(Theme.textMuted, lineWidth: 1.5)
            }
        }
        .frame(width: 9, height: 9)
        .anchorPreference(key: NodeAnchorKey.self, value: .center) { [$0] }
        .accessibilityHidden(true)
    }

    // MARK: - Path stop

    private func stop(role: String, path: String) -> some View {
        let (parent, leaf) = Self.split(path, home: home)
        return VStack(alignment: .leading, spacing: 3) {
            Text(role).smallCaps().foregroundStyle(Theme.textSoft)
            HStack(alignment: .firstTextBaseline, spacing: 0) {
                if !parent.isEmpty {
                    Text(parent)
                        .font(.system(size: 12, weight: .regular, design: .serif))
                        .foregroundStyle(Theme.textMuted)
                        .lineLimit(1)
                        .truncationMode(.head)
                        .layoutPriority(0)
                }
                Text(leaf)
                    .font(Theme.displaySm)
                    .foregroundStyle(Theme.warmInk)
                    .tracking(-0.2)
                    .lineLimit(1)
                    .layoutPriority(1)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(role) folder: \(parent)\(leaf)")
    }

    // MARK: - Path splitting

    /// Abbreviates the home directory to `~` and splits a path into its parent prefix
    /// (kept, but muted) and its leaf folder name (the recognizable part). The parent
    /// keeps its trailing slash so the two pieces read as one path when concatenated.
    static func split(_ rawPath: String, home: String) -> (parent: String, leaf: String) {
        var path = rawPath
        if !home.isEmpty, path == home || path.hasPrefix(home + "/") {
            path = "~" + path.dropFirst(home.count)
        }
        if path.count > 1, path.hasSuffix("/") { path.removeLast() }

        guard let slash = path.lastIndex(of: "/") else { return ("", path) }
        let leaf = String(path[path.index(after: slash)...])
        let parent = String(path[...slash])
        return leaf.isEmpty ? ("", parent) : (parent, leaf)
    }
}

/// Collects the route node centers (origin first, terminus second) so the connecting
/// rule can be drawn between them regardless of how each row's text wraps.
private struct NodeAnchorKey: PreferenceKey {
    static let defaultValue: [Anchor<CGPoint>] = []
    static func reduce(value: inout [Anchor<CGPoint>], nextValue: () -> [Anchor<CGPoint>]) {
        value.append(contentsOf: nextValue())
    }
}

#Preview {
    VStack(alignment: .leading, spacing: Theme.sMed) {
        Text("FOLDERS").smallCaps().foregroundStyle(Theme.textMuted)
        FolderRoute(
            watchPath: "/Users/jane/Desktop",
            outputPath: "/Users/jane/Downloads/test",
            isWatching: true,
            home: "/Users/jane"
        )
    }
    .padding(40)
    .frame(width: 280)
    .background(Theme.canvas)
}
