import SwiftUI
import AppKit

struct HeroCard: View {
    let stage: ProcessingStage?
    let currentFile: String?
    let thumbnailPath: String?
    let proposedSlug: String?
    let elapsedSeconds: TimeInterval
    let includesClipboard: Bool
    var isWatching: Bool = false
    var hasFiledBefore: Bool = false

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var typedCount: Int = 0
    @State private var blink: Bool = false

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            if isActive {
                activeView
            } else {
                idleView
            }
        }
        .padding(Theme.sLg)
        .frame(maxWidth: .infinity, alignment: .topLeading)
        .frame(minHeight: 240)
        .background(Theme.surfaceCard)
        .clipShape(.rect(cornerRadius: Theme.r10))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.r10)
                .stroke(Theme.rule, lineWidth: 0.5)
        )
        .onChange(of: proposedSlug ?? "") { _, newSlug in
            startTyping(for: newSlug)
        }
        .onAppear {
            startTyping(for: proposedSlug ?? "")
            Timer.scheduledTimer(withTimeInterval: 0.5, repeats: true) { _ in
                blink.toggle()
            }
        }
    }

    private var isActive: Bool {
        guard let stage else { return false }
        switch stage {
        case .caught, .analyzing, .renaming, .clipboard: return true
        case .success: return true
        case .error: return false
        }
    }

    @ViewBuilder
    private var idleView: some View {
        VStack(spacing: Theme.sSmall) {
            Spacer()
            EditorialIcon.Shutter(size: 56, color: Theme.textSoft.opacity(0.45))

            Text(idlePrimary)
                .font(Theme.serifItalicLg)
                .foregroundStyle(Theme.textMuted)
                .multilineTextAlignment(.center)

            Text(idleSecondary)
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textSoft)
                .multilineTextAlignment(.center)
            Spacer()
        }
        .frame(maxWidth: .infinity)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(idlePrimary) \(idleSecondary)")
    }

    private var idlePrimary: String {
        isWatching ? "Watching for screenshots." : "Ready when you are."
    }

    private var idleSecondary: String {
        isWatching
            ? "Take one with ⌘⇧4, or drop an image here."
            : "Press Start Watching, then take a screenshot."
    }

    @ViewBuilder
    private var activeView: some View {
        thumbnail
            .frame(maxWidth: .infinity, maxHeight: 160)
            .clipShape(.rect(cornerRadius: Theme.r10))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.r10)
                    .fill(Theme.paperTint)
            )

        VStack(alignment: .leading, spacing: Theme.sMicro) {
            HStack(alignment: .firstTextBaseline, spacing: Theme.sSmall) {
                Text(displayedSlug)
                    .font(.system(size: 14, design: .monospaced))
                    .foregroundStyle(Theme.ink)
                if showCursor {
                    Text("|")
                        .font(.system(size: 14, design: .monospaced))
                        .foregroundStyle(Theme.coral)
                        .opacity(blink ? 1 : 0)
                }
                Spacer()
            }
            if let original = currentFile, !original.isEmpty {
                Text(original)
                    .marginalia()
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
        }

        StageProgress(stage: stage, includesClipboard: includesClipboard)

        Text(elapsedText)
            .font(Theme.codeSm)
            .foregroundStyle(Theme.textSoft)
    }

    @ViewBuilder
    private var thumbnail: some View {
        if let path = thumbnailPath,
           !path.isEmpty,
           let image = NSImage(contentsOfFile: path) {
            Image(nsImage: image)
                .resizable()
                .scaledToFill()
        } else {
            Rectangle()
                .fill(Theme.surfaceMuted)
                .overlay(
                    EditorialIcon.Shutter(size: 60, color: Theme.textSoft.opacity(0.4))
                )
        }
    }

    private var displayedSlug: String {
        guard let slug = proposedSlug, !slug.isEmpty else { return "—" }
        if reduceMotion { return slug }
        return String(slug.prefix(typedCount))
    }

    private var showCursor: Bool {
        guard !reduceMotion else { return false }
        guard let slug = proposedSlug, !slug.isEmpty else { return false }
        return typedCount < slug.count
    }

    private var elapsedText: String {
        if elapsedSeconds <= 0 { return "—" }
        if elapsedSeconds < 1 { return String(format: "%.0fms", elapsedSeconds * 1000) }
        return String(format: "%.1fs elapsed", elapsedSeconds)
    }

    private func startTyping(for slug: String) {
        guard !reduceMotion else { typedCount = slug.count; return }
        typedCount = 0
        guard !slug.isEmpty else { return }

        let total = slug.count
        let interval: TimeInterval = 0.028
        Timer.scheduledTimer(withTimeInterval: interval, repeats: true) { timer in
            DispatchQueue.main.async {
                if typedCount < total {
                    typedCount += 1
                } else {
                    timer.invalidate()
                }
            }
        }
    }
}

#Preview("Idle") {
    HeroCard(stage: nil, currentFile: nil, thumbnailPath: nil,
             proposedSlug: nil, elapsedSeconds: 0, includesClipboard: true)
        .padding(40)
        .frame(width: 460, height: 360)
        .background(Theme.canvas)
}

#Preview("Active") {
    HeroCard(stage: .renaming,
             currentFile: "Screenshot 2026-05-30 at 11.42.13.png",
             thumbnailPath: nil,
             proposedSlug: "terminal-git-log_114213",
             elapsedSeconds: 1.4,
             includesClipboard: true)
        .padding(40)
        .frame(width: 460, height: 360)
        .background(Theme.canvas)
}
