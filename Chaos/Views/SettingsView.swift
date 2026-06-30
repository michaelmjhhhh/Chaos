import AppKit
import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @State private var testResult: String?
    @State private var isTesting = false
    @State private var configRevision = 0
    @State private var showAdvanced = false
    @State private var checkingUsage = false
    @State private var usageText: String?
    @State private var usageRemaining: Int?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.sLg) {
                masthead
                providerCard
                directoriesCard
                outputCard
                behaviorCard
                appearanceCard
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

                if appState.resolvedProvider == .chaosHosted {
                    freeTrialRow
                }

                advancedDisclosure

                testRow

                SettingsHint(privacyNote, icon: "lock.shield")
            }
        }
    }

    /// Lets a user on the bundled hosted tier check how much of their free trial is left.
    private var freeTrialRow: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Button {
                checkUsage()
            } label: {
                Label(checkingUsage ? "Checking…" : "Check free trial", systemImage: "gauge.with.needle")
                    .font(Theme.bodySm)
            }
            .buttonStyle(.bordered)
            .tint(Theme.coral)
            .disabled(checkingUsage)
            .help("See how many free names you have left on this Mac.")

            if let usageText {
                Text(usageText)
                    .font(Theme.bodySm)
                    .foregroundStyle(usageRemaining == 0 ? Theme.error : Theme.textBody)
            }
        }
    }

    private func checkUsage() {
        checkingUsage = true
        usageText = nil
        Task {
            let usage = await appState.fetchHostedUsage()
            checkingUsage = false
            if let usage {
                usageRemaining = usage.remaining
                usageText = usage.remaining > 0
                    ? "You've used \(usage.used) of \(usage.limit) free names — \(usage.remaining) left."
                    : "You've used all \(usage.limit) free names. Add your own key below to keep going."
            } else {
                usageRemaining = nil
                usageText = "Couldn't check right now. Check your internet and try again."
            }
        }
    }

    private var privacyNote: String {
        appState.resolvedProvider == .ollama
            ? "Private: with Ollama, screenshots are read on this Mac and never leave it."
            : "Each screenshot is sent to \(appState.resolvedProvider.displayName) only to generate its name."
    }

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

                        if appState.resolvedProvider.requiresBaseURL, (appState.config.baseURL ?? "").isEmpty {
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
                .foregroundStyle(Theme.canvas)
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
                let customActive = appState.resolvedCustomPrompt != nil

                VStack(alignment: .leading, spacing: Theme.sMicro) {
                    Picker("Name language", selection: languageBinding) {
                        ForEach(SlugLanguage.allCases) { language in
                            Text(language.displayName).tag(language)
                        }
                    }
                    .help("The language of the generated names — not the app's own language.")
                    .disabled(customActive)
                    .opacity(customActive ? 0.5 : 1)
                    SettingsHint(
                        customActive
                            ? "Overridden while a custom prompt is on — language is whatever your prompt specifies."
                            : "Sets the language of the generated names (the app stays in English)."
                    )
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

                customPromptSection

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

    /// Lets a user replace the built-in naming instruction with their own. The editor only
    /// appears once enabled, and is pre-filled (via the toggle binding) with the current
    /// language's default prompt so there's a starting point to edit from.
    private var customPromptSection: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            Toggle("Use a custom naming prompt", isOn: useCustomPromptBinding)
                .help("Replace the built-in instruction with your own, telling the AI exactly how to name screenshots.")

            if appState.config.useCustomPrompt == true {
                TextEditor(text: customPromptBinding)
                    .font(Theme.body)
                    .frame(minHeight: 120)
                    .padding(Theme.sSmall)
                    .scrollContentBackground(.hidden)
                    .background(Theme.surfaceMuted)
                    .clipShape(.rect(cornerRadius: Theme.r6))
                    .overlay(
                        RoundedRectangle(cornerRadius: Theme.r6)
                            .stroke(Theme.border, lineWidth: 1)
                    )

                HStack(alignment: .firstTextBaseline) {
                    SettingsHint("Your prompt fully replaces the default. Keep names short and slug-like — output is capped and cleaned automatically.")
                    Spacer()
                    Button("Reset to default") {
                        appState.config.customPrompt = VisionAPIClient.defaultSystemPrompt(
                            language: appState.resolvedLanguage
                        )
                    }
                    .buttonStyle(.link)
                    .font(Theme.caption)
                    .fixedSize()
                }
            }
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

    private var appearanceCard: some View {
        SettingsCard(
            title: "Appearance",
            subtitle: "Choose how Chaos looks, or let it follow your Mac."
        ) {
            VStack(alignment: .leading, spacing: Theme.sSmall) {
                AppearanceSelector(selection: appearanceBinding)
                    .help("System follows your Mac's Light/Dark setting. Light and Dark pin Chaos to that mode.")

                SettingsHint(
                    appState.resolvedAppearance == .system
                        ? "Following your Mac's appearance."
                        : "Chaos stays in \(appState.resolvedAppearance.label) mode regardless of your Mac."
                )
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
                .priority: NSAccessibilityPriorityLevel.medium.rawValue
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

    private var appearanceBinding: Binding<AppearancePreference> {
        Binding(
            get: { appState.resolvedAppearance },
            set: { appState.setAppearance($0) }
        )
    }

    private var useCustomPromptBinding: Binding<Bool> {
        Binding(
            get: { appState.config.useCustomPrompt ?? false },
            set: { enabled in
                appState.config.useCustomPrompt = enabled
                // Prefill the editor from the current language's default the first time it's
                // turned on, so users have a working example to edit rather than a blank box.
                if enabled,
                   (appState.config.customPrompt?.trimmingCharacters(in: .whitespacesAndNewlines) ?? "").isEmpty
                {
                    appState.config.customPrompt = VisionAPIClient.defaultSystemPrompt(
                        language: appState.resolvedLanguage
                    )
                }
            }
        )
    }

    private var customPromptBinding: Binding<String> {
        Binding(
            get: { appState.config.customPrompt ?? "" },
            set: { appState.config.customPrompt = $0.isEmpty ? nil : $0 }
        )
    }
}
