import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @State private var testResult: String?
    @State private var isTesting = false

    var body: some View {
        TabView {
            Tab("General", systemImage: "gear") { generalTab }
            Tab("Advanced", systemImage: "slider.horizontal.3") { advancedTab }
        }
        .scenePadding()
        .frame(minWidth: 500, minHeight: 420)
    }

    @ViewBuilder
    private var generalTab: some View {
        Form {
            Section("Provider") {
                Picker("Provider", selection: providerBinding) {
                    ForEach(Provider.allCases) { p in
                        Text(p.displayName).tag(p)
                    }
                }

                SecureField("API Key", text: apiKeyBinding)
                    .font(Theme.code)
                    .textFieldStyle(.roundedBorder)

                TextField("Model", text: modelBinding,
                          prompt: Text(appState.resolvedProvider.defaultModel).font(Theme.code))
                    .font(Theme.code)
                    .textFieldStyle(.roundedBorder)

                HStack(spacing: 10) {
                    Button {
                        testAPI()
                    } label: {
                        Text("Test Connection")
                            .font(Theme.button)
                            .foregroundStyle(.white)
                            .padding(.horizontal, 14)
                            .padding(.vertical, 5)
                            .background(isTesting ? Theme.coral.opacity(0.6) : Theme.coral)
                            .clipShape(.rect(cornerRadius: Theme.r6))
                    }
                    .buttonStyle(.plain)
                    .disabled(isTesting)

                    if isTesting {
                        ProgressView().controlSize(.small)
                    }

                    if let r = testResult {
                        HStack(spacing: 4) {
                            Circle()
                                .fill(r == "OK" ? Theme.success : Theme.error)
                                .frame(width: 6, height: 6)
                            Text(r)
                                .font(Theme.caption)
                                .foregroundStyle(r == "OK" ? Theme.success : Theme.error)
                        }
                    }
                }
            }

            Section("Language") {
                Picker("Slug Language", selection: languageBinding) {
                    ForEach(SlugLanguage.allCases) { l in
                        Text(l.displayName).tag(l)
                    }
                }
            }

            Section("Directories") {
                dirPicker("Watch Directory", watchDirBinding)
                dirPicker("Output Directory", outputDirBinding)
            }

            Section("Behavior") {
                Toggle("Copy image to clipboard", isOn: clipboardBinding)
                Toggle("Start watching on launch", isOn: autoStartBinding)
            }
        }
        .formStyle(.grouped)
        .onChange(of: appState.config) {
            appState.saveConfig()
        }
    }

    @ViewBuilder
    private var advancedTab: some View {
        Form {
            Section("Custom Endpoint") {
                TextField("Base URL", text: baseURLBinding,
                          prompt: Text(appState.resolvedProvider.defaultBaseURL ?? "required").font(Theme.code))
                    .font(Theme.code)
                    .textFieldStyle(.roundedBorder)

                if appState.resolvedProvider.requiresBaseURL {
                    Label("Required for OpenAI-Compatible", systemImage: "exclamationmark.triangle")
                        .font(Theme.bodySm)
                        .foregroundStyle(Theme.warning)
                }
            }

            Section("Config File") {
                LabeledContent("Path") {
                    Text(abbrev(ConfigService.configPath.path))
                        .font(Theme.codeSm)
                        .foregroundStyle(Theme.textMuted)
                        .textSelection(.enabled)
                }

                Button("Reveal in Finder") {
                    NSWorkspace.shared.activateFileViewerSelecting([ConfigService.configPath])
                }
            }
        }
        .formStyle(.grouped)
    }

    @ViewBuilder
    private func dirPicker(_ label: String, _ path: Binding<String>) -> some View {
        LabeledContent {
            HStack(spacing: 8) {
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

    private func abbrev(_ p: String) -> String {
        p.replacingOccurrences(of: NSHomeDirectory(), with: "~")
    }

    private func testAPI() {
        isTesting = true; testResult = nil
        Task {
            await appState.checkAPIHealth()
            isTesting = false; testResult = appState.apiStatus
        }
    }

    // MARK: - Bindings

    private var providerBinding: Binding<Provider> {
        Binding(get: { appState.resolvedProvider }, set: { appState.config.provider = $0.rawValue })
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
}
