import AppKit
import SwiftUI

/// A single question/answer shown in the Help sheet.
struct HelpTopic: Identifiable {
    let id = UUID()
    let question: String
    let answer: String
}

/// In-app help for people who don't live in Terminal. Answers the questions that
/// otherwise send a new user away: where files go, what an API key is, why a name looks
/// off, and what's sent where. Reached from the Help menu, the menu-bar dropdown, and
/// contextual “?” buttons.
struct HelpView: View {
    let onClose: () -> Void

    @State private var query = ""

    private let topics: [HelpTopic] = [
        HelpTopic(
            question: "How do I start naming screenshots?",
            answer: "Press Start Watching on the dashboard, then take a screenshot (⌘⇧4). Chaos notices it, names it, and files it into your output folder — usually in a couple of seconds."
        ),
        HelpTopic(
            question: "Where do my renamed files go?",
            answer: "Into your output folder (Settings → Naming & Filing). By default that's a ‘chaos-output’ folder on your Desktop. Your originals aren't deleted — they're moved there with a clearer name."
        ),
        HelpTopic(
            question: "What is an API key and do I need one?",
            answer: "An API key is a password that lets Chaos use an AI service to read your screenshots. If you pick the built-in ‘Chaos’ service you don't need one. If you use your own provider (OpenAI, DeepSeek, etc.), open Settings → Naming Service and use the ‘Get a key’ link to create one, then paste it in."
        ),
        HelpTopic(
            question: "Which naming service should I choose?",
            answer: "Start with ‘Chaos (recommended)’ if it's available — no setup. ‘Ollama’ runs privately on your Mac for free but needs a one-time install. The others (OpenAI, DeepSeek, OpenRouter, SiliconRouter) use your own paid account and key."
        ),
        HelpTopic(
            question: "The name doesn't match the picture. Why?",
            answer: "The AI occasionally misreads an image. Open the Pipeline tab, right-click the item, and choose Retry to try again. You can also switch to a more capable model in Settings → Advanced options."
        ),
        HelpTopic(
            question: "Is my data private?",
            answer: "Each screenshot is sent to the naming service you chose, only to generate its name. With ‘Ollama’ nothing leaves your Mac. With the other services the image goes to that provider — review their privacy policy if that matters to you."
        ),
        HelpTopic(
            question: "It says it can't connect. What now?",
            answer: "Check your internet first. Then open Settings → Naming Service and press Test Connection. If you use your own provider, make sure the API key is pasted correctly and your account has credit."
        ),
        HelpTopic(
            question: "Can I change the filename style?",
            answer: "Yes — Settings → Naming & Filing. The template builds names from pieces: {slug} is the AI name, {date} is the day, {time} is the time. The live Preview shows exactly what files will be called."
        )
    ]

    private var filtered: [HelpTopic] {
        let q = query.trimmingCharacters(in: .whitespaces).lowercased()
        guard !q.isEmpty else { return topics }
        return topics.filter {
            $0.question.lowercased().contains(q) || $0.answer.lowercased().contains(q)
        }
    }

    var body: some View {
        VStack(spacing: 0) {
            header

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: Theme.sSmall) {
                    if filtered.isEmpty {
                        Text("No results for “\(query)”.")
                            .font(Theme.body)
                            .foregroundStyle(Theme.textMuted)
                            .padding(.vertical, Theme.sMed)
                    }
                    ForEach(filtered) { topic in
                        TopicRow(topic: topic)
                    }
                }
                .padding(Theme.sLg)
            }

            Divider()
            footer
        }
        .frame(width: 560, height: 560)
        .background(Theme.canvas)
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            HStack {
                Text("Help & Getting Started")
                    .font(Theme.displayLg)
                    .foregroundStyle(Theme.warmInk)
                Spacer()
                Button("Done") { onClose() }
                    .keyboardShortcut(.defaultAction)
            }
            HStack(spacing: Theme.sSmall) {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.textSoft)
                TextField("Search help…", text: $query)
                    .textFieldStyle(.plain)
                    .font(Theme.body)
            }
            .padding(Theme.sSmall)
            .background(Theme.surfaceMuted)
            .clipShape(.rect(cornerRadius: Theme.r6))
        }
        .padding(Theme.sLg)
    }

    private var footer: some View {
        HStack(spacing: Theme.sLg) {
            if let site = URL(string: "https://github.com/michaelmjhhhh/chaos") {
                Link(destination: site) {
                    Label("Visit website", systemImage: "globe").font(Theme.bodySm)
                }
                .tint(Theme.coral)
            }
            Spacer()
        }
        .padding(.horizontal, Theme.sLg)
        .padding(.vertical, Theme.sMed)
    }
}

private struct TopicRow: View {
    let topic: HelpTopic
    @State private var expanded = false

    var body: some View {
        DisclosureGroup(isExpanded: $expanded) {
            Text(topic.answer)
                .font(Theme.body)
                .foregroundStyle(Theme.textBody)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, Theme.sMicro)
                .padding(.trailing, Theme.sMed)
                .frame(maxWidth: .infinity, alignment: .leading)
        } label: {
            Text(topic.question)
                .font(Theme.titleSm)
                .foregroundStyle(Theme.warmInk)
        }
        .padding(Theme.sMed)
        .background(Theme.surfaceCard)
        .clipShape(.rect(cornerRadius: Theme.r8))
        .overlay {
            RoundedRectangle(cornerRadius: Theme.r8).stroke(Theme.border, lineWidth: 0.5)
        }
    }
}
