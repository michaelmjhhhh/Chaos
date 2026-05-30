import SwiftUI

struct StageProgress: View {
    let stage: ProcessingStage?
    let includesClipboard: Bool

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private var labels: [String] {
        includesClipboard
            ? ["Analyzing", "Renaming", "Clipboard"]
            : ["Analyzing", "Renaming"]
    }

    private func state(for label: String) -> LabelState {
        guard let stage else { return .upcoming }
        let activeIdx: Int
        switch stage {
        case .caught:    activeIdx = -1
        case .analyzing: activeIdx = 0
        case .renaming:  activeIdx = 1
        case .clipboard: activeIdx = includesClipboard ? 2 : 1
        case .success:   activeIdx = labels.count
        case .error:     return .upcoming
        }
        guard let idx = labels.firstIndex(of: label) else { return .upcoming }
        if idx < activeIdx { return .completed }
        if idx == activeIdx { return .active }
        return .upcoming
    }

    var body: some View {
        HStack(spacing: Theme.sLg) {
            ForEach(labels, id: \.self) { label in
                VStack(alignment: .leading, spacing: 2) {
                    Text(label)
                        .smallCaps()
                        .foregroundStyle(color(for: state(for: label)))
                    Rectangle()
                        .fill(state(for: label) == .active ? Theme.coral : Color.clear)
                        .frame(height: 1)
                        .animation(reduceMotion ? nil : .easeInOut(duration: 0.24), value: stage)
                }
            }
            Spacer()
        }
    }

    private enum LabelState { case upcoming, active, completed }

    private func color(for s: LabelState) -> Color {
        switch s {
        case .active:    return Theme.coral
        case .completed: return Theme.ink
        case .upcoming:  return Theme.textSoft
        }
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 20) {
        StageProgress(stage: .analyzing, includesClipboard: true)
        StageProgress(stage: .renaming, includesClipboard: true)
        StageProgress(stage: .clipboard, includesClipboard: true)
        StageProgress(stage: nil, includesClipboard: true)
        StageProgress(stage: .renaming, includesClipboard: false)
    }
    .padding(40)
    .background(Theme.canvas)
    .frame(width: 380)
}
