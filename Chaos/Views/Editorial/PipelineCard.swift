import SwiftUI
import AppKit

struct PipelineCard: View {
    let file: RecentFile
    var isInFlight: Bool = false

    var body: some View {
        ZStack(alignment: .topTrailing) {
            HStack(alignment: .top, spacing: Theme.sSmall) {
                thumbnail
                content
            }
            .padding(Theme.sSmall + 2)
            .background(Theme.surfaceCard)
            .overlay(alignment: .top) {
                if !file.isError {
                    Rectangle()
                        .fill(Theme.rule)
                        .frame(height: 0.5)
                }
            }
            .overlay(alignment: .leading) {
                if file.isError {
                    Rectangle()
                        .fill(Theme.coral)
                        .frame(width: 1)
                }
            }

            if !file.isError && !isInFlight {
                Text("✓")
                    .font(.system(size: 10, design: .serif))
                    .foregroundStyle(Theme.success)
                    .padding(.top, 6)
                    .padding(.trailing, 8)
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(accessibilityLabel)
    }

    private var accessibilityLabel: String {
        if file.isError {
            return "Failed to name \(file.originalName). \(file.resultText)"
        }
        let name = file.newName.isEmpty ? file.originalName : file.newName
        return "Filed as \(name), from \(file.originalName)"
    }

    @ViewBuilder
    private var thumbnail: some View {
        ZStack {
            if !file.path.isEmpty,
               let image = NSImage(contentsOfFile: file.path) {
                Image(nsImage: image)
                    .resizable()
                    .scaledToFill()
            } else {
                Rectangle().fill(Theme.surfaceMuted)
            }
        }
        .frame(width: 40, height: 40)
        .clipShape(.rect(cornerRadius: Theme.r6))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.r6)
                .fill(Theme.paperTint)
        )
    }

    @ViewBuilder
    private var content: some View {
        VStack(alignment: .leading, spacing: 2) {
            if file.isError {
                Text(file.resultText)
                    .font(.system(size: 12, design: .serif).italic())
                    .foregroundStyle(Theme.ink)
                    .lineLimit(2)
                    .truncationMode(.tail)
            } else {
                Text(file.newName.isEmpty ? file.originalName : file.newName)
                    .font(.system(size: 12, design: .monospaced))
                    .foregroundStyle(Theme.ink)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }

            Text(file.originalName)
                .font(Theme.serifItalicSm)
                .foregroundStyle(Theme.textSoft)
                .lineLimit(1)
                .truncationMode(.middle)

            Text(file.timestamp, format: .dateTime.hour().minute().second())
                .font(.system(size: 10, design: .monospaced))
                .foregroundStyle(Theme.textSoft)
                .tracking(0.5)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 0) {
        PipelineCard(file: RecentFile(
            originalName: "Screenshot 2026-05-30 at 11.42.13.png",
            newName: "terminal-git-log_114213.png",
            path: "",
            timestamp: Date(),
            duration: 1.4,
            result: .success
        ))
        PipelineCard(file: RecentFile(
            originalName: "Screenshot 2026-05-30 at 11.55.02.png",
            newName: "",
            path: "",
            timestamp: Date(),
            duration: 0,
            result: .error("API timeout after 30s")
        ))
        PipelineCard(file: RecentFile(
            originalName: "Screenshot.png",
            newName: "in-flight-card.png",
            path: "",
            timestamp: Date(),
            duration: 0,
            result: .success
        ), isInFlight: true)
    }
    .padding(20)
    .background(Theme.canvas)
    .frame(width: 320)
}
