import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @State private var testResult: String?
    @State private var isTesting = false

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
        .onChange(of: appState.config) {
            appState.saveConfig()
        }
    }

    private var masthead: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text("Settings")
                .font(Theme.displayXL)
                .foregroundStyle(Theme.warmInk)

            Text("Connect a naming model and tune your filing workflow.")
                .font(Theme.body)
                .foregroundStyle(Theme.textSoft)
        }
    }

    private var providerCard: some View {
        SettingsCard(
            title: "AI Provider",
            subtitle: "Choose the model service used to name incoming images."
        ) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                Picker("Provider", selection: providerBinding) {
                    ForEach(Provider.allCases) { provider in
                        Text(provider.displayName).tag(provider)
                    }
                }

                HStack(alignment: .center, spacing: Theme.sSmall) {
                    Text(appState.resolvedProvider.summary)
                        .font(Theme.bodySm)
                        .foregroundStyle(Theme.textSoft)

                    Spacer()

                    SettingsBadge(
                        text: appState.resolvedProvider.connectionKind,
                        systemImage: appState.resolvedProvider == .ollama ? "desktopcomputer" : "network",
                        tint: appState.resolvedProvider == .ollama ? Theme.teal : Theme.coral
                    )
                }

                if appState.resolvedProvider.requiresAPIKey {
                    VStack(alignment: .leading, spacing: Theme.sMicro) {
                        SecureField("API Key", text: apiKeyBinding)
                            .font(Theme.code)
                            .textFieldStyle(.roundedBorder)

                        Text("Stored for the selected remote provider and used for its requests.")
                            .font(Theme.bodySm)
                            .foregroundStyle(Theme.textSoft)
                    }
                }

                VStack(alignment: .leading, spacing: Theme.sMicro) {
                    TextField(
                        "Model",
                        text: modelBinding,
                        prompt: Text(appState.resolvedProvider.defaultModel).font(Theme.code)
                    )
                    .font(Theme.code)
                    .textFieldStyle(.roundedBorder)

                    Text("Default model: \(appState.resolvedProvider.defaultModel)")
                        .font(Theme.bodySm)
                        .foregroundStyle(Theme.textSoft)
                }

                if appState.resolvedProvider.allowsCustomBaseURL {
                    VStack(alignment: .leading, spacing: Theme.sMicro) {
                        TextField("Base URL", text: baseURLBinding, prompt: Text("https://...").font(Theme.code))
                            .font(Theme.code)
                            .textFieldStyle(.roundedBorder)

                        if appState.resolvedProvider.requiresBaseURL && (appState.config.baseURL ?? "").isEmpty {
                            Label("Required for OpenAI-Compatible", systemImage: "exclamationmark.triangle.fill")
                                .font(Theme.bodySm)
                                .foregroundStyle(Theme.warning)
                        }
                    }
                } else {
                    LabeledContent("Endpoint") {
                        Text(appState.resolvedBaseURL)
                            .font(Theme.codeSm)
                            .foregroundStyle(Theme.textMuted)
                            .textSelection(.enabled)
                    }
                }

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
                        .background(isTesting ? Theme.coral.opacity(0.6) : Theme.coral)
                        .clipShape(.rect(cornerRadius: Theme.r6))
                    }
                    .buttonStyle(.plain)
                    .disabled(isTesting)

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
            title: "Output",
            subtitle: "Shape generated filenames and optional subfolder organization."
        ) {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                Picker("Language", selection: languageBinding) {
                    ForEach(SlugLanguage.allCases) { language in
                        Text(language.displayName).tag(language)
                    }
                }

                VStack(alignment: .leading, spacing: Theme.sMicro) {
                    TextField(
                        "Filename Template",
                        text: filenameTemplateBinding,
                        prompt: Text(NamingPolicy.defaultTemplate).font(Theme.code)
                    )
                    .font(Theme.code)
                    .textFieldStyle(.roundedBorder)

                    Text("Tokens: {slug}  {date}  {time}")
                        .font(Theme.codeSm)
                        .foregroundStyle(Theme.textSoft)
                }

                Picker("Subfolders", selection: subfolderRuleBinding) {
                    ForEach(SubfolderRule.allCases) { rule in
                        Text(rule.displayName).tag(rule)
                    }
                }
            }
        }
    }

    private var behaviorCard: some View {
        SettingsCard(
            title: "Behavior",
            subtitle: "Control what Chaos does after processing and when it launches."
        ) {
            VStack(alignment: .leading, spacing: Theme.sSmall) {
                Toggle("Copy image to clipboard", isOn: clipboardBinding)
                Toggle("Start watching on launch", isOn: autoStartBinding)
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

                Button("Choose...") {
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
        Task {
            await appState.checkAPIHealth()
            isTesting = false
            testResult = appState.apiStatus
        }
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
