import AppKit
import SwiftUI

/// First-run welcome shown once (gated by `AppState.hasCompletedOnboarding`). It walks
/// a brand-new, non-technical user from "what is this" to a running watcher in a few
/// taps. When the bundled hosted provider is live the AI step disappears entirely —
/// true zero-config. Until then it offers a short, guided "connect your own provider"
/// step so the product still works end-to-end.
struct WelcomeView: View {
    @Environment(AppState.self) private var appState
    let onFinish: () -> Void

    @State private var stepIndex = 0
    @State private var isTesting = false
    @State private var testResult: String?

    private enum Step: Equatable {
        case intro
        case connect
        case files
        case ready
    }

    private var steps: [Step] {
        // Skip the "connect" step once naming is built in.
        HostedProvider.isConfigured
            ? [.intro, .files, .ready]
            : [.intro, .connect, .files, .ready]
    }

    private var step: Step {
        steps[min(stepIndex, steps.count - 1)]
    }

    var body: some View {
        VStack(spacing: 0) {
            content
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                .padding(Theme.sBreak)

            footer
        }
        .frame(width: 560, height: 460)
        .background(Theme.canvas)
    }

    // MARK: - Steps

    @ViewBuilder
    private var content: some View {
        switch step {
        case .intro: introStep
        case .connect: connectStep
        case .files: filesStep
        case .ready: readyStep
        }
    }

    private var introStep: some View {
        stepScaffold(
            eyebrow: "Welcome",
            title: "Chaos names your screenshots.",
            blurb: "Take a screenshot and Chaos quietly looks at it, gives it a clear, searchable name, and files it away — so you never dig through “Screenshot 2026-…” again."
        ) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                bullet("camera.viewfinder", "It watches your Desktop for new screenshots.")
                bullet("sparkles", "AI reads each one and writes a useful name.")
                bullet("tray.full", "Renamed images land in one tidy folder.")
            }
        }
    }

    private var connectStep: some View {
        stepScaffold(
            eyebrow: "One quick step",
            title: "Turn on naming.",
            blurb: "Chaos uses an AI service to read your screenshots. Pick one below — it's free to start, and you can change it later in Settings."
        ) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                Picker("Service", selection: providerBinding) {
                    ForEach(onboardingProviders) { provider in
                        Text(provider.displayName).tag(provider)
                    }
                }
                .labelsHidden()

                Text(appState.resolvedProvider.plainDescription)
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textMuted)

                if appState.resolvedProvider.requiresAPIKey {
                    VStack(alignment: .leading, spacing: Theme.sSmall) {
                        SecureField("Paste your API key", text: apiKeyBinding)
                            .textFieldStyle(.roundedBorder)
                            .font(Theme.code)

                        if let url = appState.resolvedProvider.signupURL {
                            Link(destination: url) {
                                Label("Get a key — opens \(url.host ?? "the provider")", systemImage: "arrow.up.right.square")
                                    .font(Theme.bodySm)
                            }
                            .tint(Theme.coral)
                        }
                    }
                }

                connectTestRow
            }
        }
    }

    private var filesStep: some View {
        stepScaffold(
            eyebrow: "Where things go",
            title: "Your tidy folder.",
            blurb: "Renamed screenshots are filed here. The default works for most people — change it if you'd like."
        ) {
            VStack(alignment: .leading, spacing: Theme.sSmall) {
                folderRow(label: "Saved to", path: appState.resolvedOutputDir) {
                    chooseFolder { appState.config.outputDir = $0 }
                }
                folderRow(label: "Watching", path: appState.resolvedWatchDir) {
                    chooseFolder { appState.config.watchDir = $0 }
                }
                Text("Your original screenshots aren't deleted — they're moved here with a better name.")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textMuted)
                    .padding(.top, Theme.sMicro)
            }
        }
    }

    private var readyStep: some View {
        stepScaffold(
            eyebrow: "All set",
            title: "You're ready.",
            blurb: "Press Start, then take a screenshot (⌘⇧4). Watch it appear, freshly named, on your dashboard."
        ) {
            HStack(spacing: Theme.sSmall) {
                Image(systemName: "lock.shield")
                    .foregroundStyle(Theme.teal)
                Text("Privacy: each screenshot is sent to your chosen AI service only to generate its name.")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textMuted)
            }
        }
    }

    // MARK: - Footer / navigation

    private var footer: some View {
        VStack(spacing: 0) {
            Rectangle().fill(Theme.divider).frame(height: 0.5)

            HStack(spacing: Theme.sMed) {
                Button("Skip") { finish(start: false) }
                    .buttonStyle(.plain)
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textSoft)

                Spacer()

                stepDots

                Spacer()

                if stepIndex > 0 {
                    Button("Back") { withAnimation { stepIndex -= 1 } }
                        .buttonStyle(.plain)
                        .font(Theme.button)
                        .foregroundStyle(Theme.textBody)
                }

                primaryButton
            }
            .padding(.horizontal, Theme.sBreak)
            .padding(.vertical, Theme.sMed)
        }
    }

    private var primaryButton: some View {
        Button {
            if step == .ready {
                finish(start: true)
            } else {
                withAnimation { stepIndex += 1 }
            }
        } label: {
            Text(step == .ready ? "Start Watching" : "Continue")
                .font(Theme.button)
                .foregroundStyle(.white)
                .padding(.horizontal, 18)
                .padding(.vertical, 8)
                .background(continueEnabled ? Theme.coral : Theme.coral.opacity(0.4))
                .clipShape(.rect(cornerRadius: Theme.r6))
        }
        .buttonStyle(.plain)
        .disabled(!continueEnabled)
    }

    /// Block "Continue" past the connect step until a key is present for providers that
    /// need one — so a user can't sail past the only thing that makes naming work.
    private var continueEnabled: Bool {
        guard step == .connect, appState.resolvedProvider.requiresAPIKey else { return true }
        return !appState.resolvedAPIKey.trimmingCharacters(in: .whitespaces).isEmpty
    }

    private var stepDots: some View {
        HStack(spacing: 6) {
            ForEach(0 ..< steps.count, id: \.self) { i in
                Circle()
                    .fill(i == stepIndex ? Theme.coral : Theme.border)
                    .frame(width: 6, height: 6)
            }
        }
    }

    private var connectTestRow: some View {
        HStack(spacing: Theme.sSmall) {
            Button {
                runTest()
            } label: {
                Label(isTesting ? "Checking…" : "Test", systemImage: "bolt.horizontal.circle")
                    .font(Theme.bodySm)
            }
            .buttonStyle(.bordered)
            .tint(Theme.ink)
            .disabled(isTesting || !continueEnabled)

            if isTesting {
                ProgressView().controlSize(.small)
            } else if let testResult {
                Label(
                    testResult == "OK" ? "Looks good" : "Couldn't connect",
                    systemImage: testResult == "OK" ? "checkmark.circle.fill" : "xmark.circle.fill"
                )
                .font(Theme.bodySm)
                .foregroundStyle(testResult == "OK" ? Theme.success : Theme.error)
            }
        }
    }

    // MARK: - Building blocks

    private func stepScaffold(
        eyebrow: String,
        title: String,
        blurb: String,
        @ViewBuilder body: () -> some View
    ) -> some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            Text(eyebrow.uppercased())
                .font(Theme.captionSm)
                .tracking(1.4)
                .foregroundStyle(Theme.coral)

            Text(title)
                .font(Theme.displayXL)
                .foregroundStyle(Theme.warmInk)

            Text(blurb)
                .font(Theme.body)
                .foregroundStyle(Theme.textBody)
                .fixedSize(horizontal: false, vertical: true)

            Spacer().frame(height: Theme.sSmall)

            body()

            Spacer(minLength: 0)
        }
    }

    private func bullet(_ icon: String, _ text: String) -> some View {
        HStack(alignment: .center, spacing: Theme.sSmall) {
            Image(systemName: icon)
                .foregroundStyle(Theme.coral)
                .frame(width: 22)
            Text(text)
                .font(Theme.body)
                .foregroundStyle(Theme.textBody)
        }
    }

    private func folderRow(label: String, path: String, action: @escaping () -> Void) -> some View {
        HStack(spacing: Theme.sSmall) {
            Text(label.uppercased())
                .font(Theme.captionSm)
                .tracking(1.2)
                .foregroundStyle(Theme.textSoft)
                .frame(width: 72, alignment: .leading)
            Text(abbrev(path))
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textMuted)
                .lineLimit(1)
                .truncationMode(.middle)
                .frame(maxWidth: .infinity, alignment: .leading)
            Button("Change…", action: action)
                .controlSize(.small)
        }
    }

    // MARK: - Actions & bindings

    private var onboardingProviders: [Provider] {
        // Keep the first-run list short and approachable; advanced presets remain in
        // full Settings.
        Provider.allCases.filter { $0 != .openaiCompatible }
    }

    private var providerBinding: Binding<Provider> {
        Binding(get: { appState.resolvedProvider }, set: {
            appState.selectProvider($0)
            testResult = nil
        })
    }

    private var apiKeyBinding: Binding<String> {
        Binding(
            get: { appState.config.apiKey ?? "" },
            set: {
                appState.config.apiKey = $0.isEmpty ? nil : $0
                testResult = nil
            }
        )
    }

    private func chooseFolder(_ assign: @escaping (String) -> Void) {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            assign(url.path)
        }
    }

    private func runTest() {
        isTesting = true
        testResult = nil
        Task {
            let result = await appState.checkAPIHealth()
            isTesting = false
            testResult = result
        }
    }

    private func finish(start: Bool) {
        appState.saveConfig()
        appState.hasCompletedOnboarding = true
        if start, appState.startupValidationError == nil {
            appState.start()
        }
        onFinish()
    }

    private func abbrev(_ p: String) -> String {
        p.replacingOccurrences(of: NSHomeDirectory(), with: "~")
    }
}
