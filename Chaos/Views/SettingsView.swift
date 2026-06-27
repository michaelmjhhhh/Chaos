import AppKit
import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @State private var testResult: String?
    @State private var isTesting = false
    @State private var configRevision = 0
    @State private var showAdvanced = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.sLg) {
                masthead
                providerCard
                directoriesCard
                outputCard
                behaviorCard
                configurationCard
            }
            .frame(maxWidth: 680)
            .padding(Theme.sLg)
            .frame(maxWidth: .infinity)
        }
        .background(Theme.canvas)
        .frame(minWidth: 560, minHeight: 680)
        .onAppear {
            // Surface the required Base URL field straight away for custom endpoints.
            if appState.resolvedProvider.requiresBaseURL { showAdvanced = true }
        }
        .onChange(of: appState.config) {
            appState.saveConfig()
            appState.invalidateAPIHealthCheck()
            configRevision += 1
            testResult = nil
            if appState.resolvedProvider.requiresBaseURL { showAdvanced = true }
        }
    }

    private var masthead: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text("Settings")
                .font(Theme.displayXL)
                .foregroundStyle(Theme.warmInk)

            Text("Connect a naming model and tune your filing workflow.")
                .font(Theme.body)
                .foregroundStyle(Theme.textMuted)
        }
    }

    private var providerCard: some View {
        SettingsCard(
            title: "Naming Service",
            subtitle: "The AI that looks at your screenshots and writes their names."
        ) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                Picker("Provider", selection: providerBinding) {
                    ForEach(Provider.allCases) { provider in
                        Text(provider.displayName).tag(provider)
                    }
                }
                .labelsHidden()
                .help("Choose who does the naming. ‘Chaos’ needs no setup; the others use your own account.")

                HStack(alignment: .center, spacing: Theme.sSmall) {
                    Text(appState.resolvedProvider.plainDescription)
                        .font(Theme.bodySm)
                        .foregroundStyle(Theme.textMuted)
                        .fixedSize(horizontal: false, vertical: true)

                    Spacer()

                    SettingsBadge(
                        text: appState.resolvedProvider.connectionKind,
                        systemImage: badgeIcon,
                        tint: badgeTint
                    )
                }

                if appState.resolvedProvider.requiresAPIKey {
                    apiKeyField
                }

                advancedDisclosure

                testRow

                SettingsHint(privacyNote, icon: "lock.shield")
            }
        }
    }

    private var privacyNote: String {
        appState.resolvedProvider == .ollama
            ? "Private: with Ollama, screenshots are read on this Mac and never leave it."
            : "Each screenshot is sent to \(appState.resolvedProvider.displayName) only to generate its name."
    }

    @ViewBuilder
    private var apiKeyField: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            Text("API Key")
                .font(Theme.caption)
                .foregroundStyle(Theme.textMuted)

            SecureField("Paste your API key", text: apiKeyBinding)
                .font(Theme.code)
                .textFieldStyle(.roundedBorder)
                .help("The secret key from your provider account. It's stored on this Mac only.")

            if let url = appState.resolvedProvider.signupURL {
                Link(destination: url) {
                    Label("Don't have a key? Get one — opens \(url.host ?? "the provider")", systemImage: "arrow.up.right.square")
                        .font(Theme.bodySm)
                }
                .tint(Theme.coral)
            }

            SettingsHint("One saved key is reused across remote providers. Update it when you switch services.")
        }
    }

    /// Model and Base URL are power-user knobs — hidden by default so the common path
    /// (pick a service, paste a key, test) stays uncluttered for non-technical users.
    @ViewBuilder
    private var advancedDisclosure: some View {
        DisclosureGroup(isExpanded: $showAdvanced) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                VStack(alignment: .leading, spacing: Theme.sMicro) {
                    Text("Model")
                        .font(Theme.caption)
                        .foregroundStyle(Theme.textMuted)

                    TextField(
                        "Model",
                        text: modelBinding,
                        prompt: Text(appState.resolvedProvider.defaultModel).font(Theme.code)
                    )
                    .font(Theme.code)
                    .textFieldStyle(.roundedBorder)
                    .help("Which AI model to use. Leave blank to use the recommended default.")

                    SettingsHint("Leave blank to use the default: \(appState.resolvedProvider.defaultModel)")
                }

                if appState.resolvedProvider.allowsCustomBaseURL {
                    VStack(alignment: .leading, spacing: Theme.sMicro) {
                        Text("Base URL")
                            .font(Theme.caption)
                            .foregroundStyle(Theme.textMuted)

                        TextField("Base URL", text: baseURLBinding, prompt: Text("https://...").font(Theme.code))
                            .font(Theme.code)
                            .textFieldStyle(.roundedBorder)
                            .help("The web address of your OpenAI-compatible service.")

                        if appState.resolvedProvider.requiresBaseURL && (appState.config.baseURL ?? "").isEmpty {
                            HStack(spacing: Theme.sMicro) {
                                Image(systemName: "exclamationmark.triangle.fill")
                                    .foregroundStyle(Theme.warning)

                                Text("Required for OpenAI-Compatible")
                                    .foregroundStyle(Theme.textBody)
                            }
                                .font(Theme.bodySm)
                        }
                    }
                } else {
                    LabeledContent("Endpoint") {
                        Text(appState.resolvedBaseURL.isEmpty ? "—" : appState.resolvedBaseURL)
                            .font(Theme.codeSm)
                            .foregroundStyle(Theme.textMuted)
                            .textSelection(.enabled)
                    }
                }
            }
            .padding(.top, Theme.sSmall)
        } label: {
            Text("Advanced options")
                .font(Theme.caption)
                .foregroundStyle(Theme.textMuted)
        }
    }

    @ViewBuilder
    private var testRow: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Button {
                testAPI()
            } label: {
                Label(
                    isTesting ? "Testing Connection..." : "Test Connection",
                    systemImage: isTesting ? "clock" : "bolt.horizontal.circle"
                )
                .font(Theme.button)
                .foregroundStyle(.white)
                .padding(.horizontal, 14)
                .padding(.vertical, 7)
                .background(isTesting ? Theme.ink.opacity(0.6) : Theme.ink)
                .clipShape(.rect(cornerRadius: Theme.r6))
            }
            .buttonStyle(.plain)
            .disabled(isTesting)
            .help("Send a quick request to confirm naming will work.")

            if isTesting {
                SettingsConnectionResult(
                    status: "Checking...",
                    failureHint: appState.resolvedProvider.connectionFailureHint
                )
            } else if let testResult {
                SettingsConnectionResult(
                    status: testResult,
                    failureHint: appState.resolvedProvider.connectionFailureHint
                )
            }
        }
    }

    private var badgeIcon: String {
        switch appState.resolvedProvider {
        case .ollama: "desktopcomputer"
        case .chaosHosted: "sparkles"
        default: "network"
        }
    }

    private var badgeTint: Color {
        switch appState.resolvedProvider {
        case .ollama: Theme.teal
        case .chaosHosted: Theme.success
        default: Theme.coral
        }
    }

    private var directoriesCard: some View {
        SettingsCard(
            title: "Directories",
            subtitle: "Select where screenshots arrive and where renamed files are filed."
        ) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                dirPicker("Watch", watchDirBinding)
                dirPicker("Output", outputDirBinding)
            }
        }
    }

    private var outputCard: some View {
        SettingsCard(
            title: "Naming & Filing",
            subtitle: "Shape the filenames Chaos creates and how it groups them."
        ) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                VStack(alignment: .leading, spacing: Theme.sMicro) {
                    Picker("Name language", selection: languageBinding) {
                        ForEach(SlugLanguage.allCases) { language in
                            Text(language.displayName).tag(language)
                        }
                    }
                    .help("The language of the generated names — not the app's own language.")
                    SettingsHint("Sets the language of the generated names (the app stays in English).")
                }

                VStack(alignment: .leading, spacing: Theme.sMicro) {
                    Text("Filename Template")
                        .font(Theme.caption)
                        .foregroundStyle(Theme.textMuted)

                    TextField(
                        "Filename Template",
                        text: filenameTemplateBinding,
                        prompt: Text(NamingPolicy.defaultTemplate).font(Theme.code)
                    )
                    .font(Theme.code)
                    .textFieldStyle(.roundedBorder)
                    .help("Build the filename from pieces: {slug} the AI name, {date} the day, {time} the time.")

                    SettingsHint("Pieces you can use:  {slug} = AI name · {date} = 2026-06-27 · {time} = 143200")
                }

                Picker("Group into subfolders", selection: subfolderRuleBinding) {
                    ForEach(SubfolderRule.allCases) { rule in
                        Text(rule.displayName).tag(rule)
                    }
                }
                .help("Optionally tuck filed images into dated subfolders.")

                filenamePreview
            }
        }
    }

    /// A live sample of what a finished file will be called, so the template tokens
    /// stop being abstract. Updates as the template / subfolder / language change.
    @ViewBuilder
    private var filenamePreview: some View {
        let policy = appState.resolvedNamingPolicy
        let sample = sampleDate
        let base = policy.renderedBaseName(slug: "meeting-notes", date: sample) + ".png"
        let dir = policy.outputDirectory(base: URL(fileURLWithPath: "/__root__"), date: sample)
        let prefix = dir.lastPathComponent == "__root__" ? "" : dir.lastPathComponent + "/"

        VStack(alignment: .leading, spacing: Theme.sMicro) {
            Text("PREVIEW")
                .font(Theme.captionSm)
                .tracking(1.2)
                .foregroundStyle(Theme.textSoft)
            HStack(spacing: Theme.sSmall) {
                Image(systemName: "doc.text.image")
                    .foregroundStyle(Theme.coral)
                Text(prefix + base)
                    .font(Theme.code)
                    .foregroundStyle(Theme.warmInk)
                    .textSelection(.enabled)
            }
            .padding(Theme.sSmall)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.surfaceMuted)
            .clipShape(.rect(cornerRadius: Theme.r6))
        }
    }

    /// A fixed, readable example timestamp for the preview (so seconds don't tick).
    private var sampleDate: Date {
        var c = DateComponents()
        c.year = 2026; c.month = 6; c.day = 27; c.hour = 14; c.minute = 32; c.second = 0
        return Calendar(identifier: .gregorian).date(from: c) ?? Date()
    }

    private var behaviorCard: some View {
        SettingsCard(
            title: "Behavior",
            subtitle: "Control what Chaos does after processing and when it launches."
        ) {
            VStack(alignment: .leading, spacing: Theme.sSmall) {
                Toggle("Copy image to clipboard", isOn: clipboardBinding)
                    .help("After filing, put the renamed image on the clipboard so you can paste it.")
                Toggle("Start watching on launch", isOn: autoStartBinding)
                    .help("Begin watching automatically when Chaos opens.")
                Toggle("Show a notification when done", isOn: notifyBinding)
                    .help("Get a macOS notification each time a screenshot is filed or fails.")
            }
        }
    }

    private var configurationCard: some View {
        SettingsCard(
            title: "Configuration",
            subtitle: "Inspect the configuration file used by this installation."
        ) {
            LabeledContent("Config") {
                HStack(spacing: Theme.sSmall) {
                    Text(abbrev(ConfigService.defaultConfigPath.path))
                        .font(Theme.codeSm)
                        .foregroundStyle(Theme.textMuted)
                        .textSelection(.enabled)
                        .lineLimit(1)
                        .truncationMode(.middle)

                    Button("Reveal") {
                        NSWorkspace.shared.activateFileViewerSelecting([ConfigService.defaultConfigPath])
                    }
                    .controlSize(.small)
                }
            }
        }
    }

    @ViewBuilder
    private func dirPicker(_ label: String, _ path: Binding<String>) -> some View {
        LabeledContent {
            HStack(spacing: Theme.sSmall) {
                Text(path.wrappedValue.isEmpty ? "Default" : abbrev(path.wrappedValue))
                    .font(Theme.codeSm)
                    .foregroundStyle(Theme.textMuted)
                    .lineLimit(1)
                    .truncationMode(.middle)
                    .frame(maxWidth: .infinity, alignment: .trailing)

                Button("Choose…") {
                    let panel = NSOpenPanel()
                    panel.canChooseFiles = false
                    panel.canChooseDirectories = true
                    panel.allowsMultipleSelection = false
                    if !path.wrappedValue.isEmpty {
                        panel.directoryURL = URL(fileURLWithPath: path.wrappedValue)
                    }
                    if panel.runModal() == .OK, let url = panel.url {
                        path.wrappedValue = url.path
                    }
                }
                .controlSize(.small)
            }
        } label: {
            Text(label)
        }
    }

    private func abbrev(_ path: String) -> String {
        path.replacingOccurrences(of: NSHomeDirectory(), with: "~")
    }

    private func testAPI() {
        isTesting = true
        testResult = nil
        let revision = configRevision
        Task {
            let result = await appState.checkAPIHealth()
            isTesting = false
            guard configRevision == revision, let result else { return }
            testResult = result
            announceConnectionResult(result)
        }
    }

    private func announceConnectionResult(_ status: String) {
        let announcement = status == "OK"
            ? "Connection successful."
            : "Connection failed. \(appState.resolvedProvider.connectionFailureHint)"
        NSAccessibility.post(
            element: NSApp as Any,
            notification: .announcementRequested,
            userInfo: [
                .announcement: announcement,
                .priority: NSAccessibilityPriorityLevel.medium.rawValue,
            ]
        )
    }

    // MARK: - Bindings

    private var providerBinding: Binding<Provider> {
        Binding(get: { appState.resolvedProvider }, set: { appState.selectProvider($0) })
    }

    private var apiKeyBinding: Binding<String> {
        Binding(get: { appState.config.apiKey ?? "" }, set: { appState.config.apiKey = $0.isEmpty ? nil : $0 })
    }

    private var modelBinding: Binding<String> {
        Binding(get: { appState.config.model ?? "" }, set: { appState.config.model = $0.isEmpty ? nil : $0 })
    }

    private var baseURLBinding: Binding<String> {
        Binding(get: { appState.config.baseURL ?? "" }, set: { appState.config.baseURL = $0.isEmpty ? nil : $0 })
    }

    private var languageBinding: Binding<SlugLanguage> {
        Binding(get: { appState.resolvedLanguage }, set: { appState.config.language = $0.rawValue })
    }

    private var watchDirBinding: Binding<String> {
        Binding(get: { appState.config.watchDir ?? "" }, set: { appState.config.watchDir = $0.isEmpty ? nil : $0 })
    }

    private var outputDirBinding: Binding<String> {
        Binding(get: { appState.config.outputDir ?? "" }, set: { appState.config.outputDir = $0.isEmpty ? nil : $0 })
    }

    private var clipboardBinding: Binding<Bool> {
        Binding(get: { appState.config.copyToClipboard ?? false }, set: { appState.config.copyToClipboard = $0 })
    }

    private var notifyBinding: Binding<Bool> {
        Binding(
            get: { appState.config.notifyOnComplete ?? false },
            set: { enabled in
                appState.config.notifyOnComplete = enabled
                if enabled { NotificationService.requestAuthorization() }
            }
        )
    }

    private var autoStartBinding: Binding<Bool> {
        Binding(get: { appState.autoStart }, set: { appState.autoStart = $0 })
    }

    private var filenameTemplateBinding: Binding<String> {
        Binding(
            get: { appState.config.filenameTemplate ?? "" },
            set: { appState.config.filenameTemplate = $0.isEmpty ? nil : $0 }
        )
    }

    private var subfolderRuleBinding: Binding<SubfolderRule> {
        Binding(
            get: { SubfolderRule.from(appState.config.subfolderRule) },
            set: { appState.config.subfolderRule = $0.rawValue }
        )
    }
}
