import AppKit
import Foundation
import SwiftUI

@Observable @MainActor
final class AppState {
    var watcherStatus: WatcherStatus = .stopped
    var watcherStartedAt: Date?
    var config: AppConfig = .init()
    var currentFile: String?
    var currentStage: ProcessingStage?
    var recentFiles: [RecentFile] = []
    var apiStatus: String = "N/A"

    var session = SessionMeta()
    var hourlyThroughput: [Int] = Array(repeating: 0, count: 24)
    var successWindow: [Double] = []

    var totalProcessed: Int = 0
    var successes: Int = 0
    var errors: Int = 0
    private(set) var latencies: [TimeInterval] = []

    /// Plain-language description of the most recent failure, with a suggested recovery
    /// action. Surfaced on the dashboard so non-technical users know what to do next.
    var lastError: FriendlyError?

    /// Drives the in-app Help/FAQ sheet. Set from the Help menu, the menu-bar dropdown,
    /// or contextual “?” affordances.
    var showHelp = false

    @ObservationIgnored @AppStorage("autoStart") var autoStart = false
    @ObservationIgnored @AppStorage("hasCompletedOnboarding") var hasCompletedOnboarding = false

    @ObservationIgnored private var watcher: DirectoryWatcher?
    @ObservationIgnored private let processor = FileProcessor()
    @ObservationIgnored private let configService = ConfigService()
    @ObservationIgnored private let historyStore = HistoryStore()
    @ObservationIgnored private var apiHealthCheckID = UUID()

    var isWatching: Bool {
        if case .running = watcherStatus { return true }
        if case .starting = watcherStatus { return true }
        return false
    }

    var resolvedProvider: Provider {
        Provider.from(config.provider)
    }

    var resolvedModel: String {
        let m = config.model?.trimmingCharacters(in: .whitespaces) ?? ""
        return m.isEmpty ? resolvedProvider.defaultModel : m
    }

    var resolvedBaseURL: String {
        guard resolvedProvider.allowsCustomBaseURL else {
            return resolvedProvider.defaultBaseURL ?? ""
        }
        let b = config.baseURL?.trimmingCharacters(in: .whitespaces) ?? ""
        return b.isEmpty ? (resolvedProvider.defaultBaseURL ?? "") : b
    }

    var resolvedAPIKey: String {
        // The bundled hosted provider authenticates with a managed app token plus this
        // device's hash, so the proxy can meter the free trial per device. The user never
        // sees either. Everything else uses the user's own key (or none for Ollama).
        if resolvedProvider == .chaosHosted {
            return "\(HostedProvider.bundledCredential):\(DeviceIdentity.hash)"
        }
        return resolvedProvider.requiresAPIKey ? (config.apiKey ?? "") : ""
    }

    var startupValidationError: String? {
        guard resolvedProvider.requiresAPIKey,
              resolvedAPIKey.trimmingCharacters(in: .whitespaces).isEmpty
        else {
            return nil
        }
        return "API key not configured"
    }

    var resolvedWatchDir: String {
        let w = config.watchDir?.trimmingCharacters(in: .whitespaces) ?? ""
        return w.isEmpty ? NSHomeDirectory() + "/Desktop" : w
    }

    var resolvedOutputDir: String {
        let o = config.outputDir?.trimmingCharacters(in: .whitespaces) ?? ""
        return o.isEmpty ? NSHomeDirectory() + "/Desktop/chaos-output" : o
    }

    var resolvedLanguage: SlugLanguage {
        SlugLanguage.from(config.language)
    }

    var resolvedCopyToClipboard: Bool {
        config.copyToClipboard ?? false
    }

    var resolvedNotifyOnComplete: Bool {
        config.notifyOnComplete ?? false
    }

    var resolvedAppearance: AppearancePreference {
        AppearancePreference.from(config.appearance)
    }

    var resolvedNamingPolicy: NamingPolicy {
        NamingPolicy(
            template: config.filenameTemplate,
            subfolderRule: SubfolderRule.from(config.subfolderRule)
        )
    }

    var successRate: Double {
        guard totalProcessed > 0 else { return 0 }
        return Double(successes) / Double(totalProcessed)
    }

    var avgLatency: TimeInterval {
        guard !latencies.isEmpty else { return 0 }
        return latencies.reduce(0, +) / Double(latencies.count)
    }

    var p95Latency: TimeInterval {
        guard !latencies.isEmpty else { return 0 }
        let sorted = latencies.sorted()
        let idx = Int(Double(sorted.count - 1) * 0.95 + 0.5)
        return sorted[min(idx, sorted.count - 1)]
    }

    var vocabularyToday: [String] {
        let calendar = Calendar.current
        let today = calendar.startOfDay(for: Date())
        let slugs = recentFiles
            .filter { !$0.isError }
            .filter { calendar.startOfDay(for: $0.timestamp) == today }
            .map(\.newName)
        return Tokenizer.topNouns(from: slugs, limit: 5)
    }

    var latencyHistory: [Double] {
        Array(latencies.suffix(24))
    }

    var successRateHistory: [Double] {
        Array(successWindow.suffix(20))
    }

    func loadConfig() {
        config = configService.load()
        recentFiles = historyStore.load()
        applyAppearance()
    }

    func saveConfig() {
        try? configService.save(config)
    }

    /// Update the app's light/dark appearance preference and apply it immediately.
    /// `.system` is stored as nil to keep the config file clean (the default).
    func setAppearance(_ preference: AppearancePreference) {
        config.appearance = preference == .system ? nil : preference.rawValue
        applyAppearance()
    }

    /// Force the app's appearance to the saved preference, or follow the system
    /// when `.system`. The adaptive Theme colors resolve against this.
    func applyAppearance() {
        switch resolvedAppearance {
        case .system: NSApplication.shared.appearance = nil
        case .light: NSApplication.shared.appearance = NSAppearance(named: .aqua)
        case .dark: NSApplication.shared.appearance = NSAppearance(named: .darkAqua)
        }
    }

    func selectProvider(_ provider: Provider) {
        guard provider != resolvedProvider else { return }
        config.provider = provider.rawValue
        config.model = nil
        config.baseURL = nil
    }

    func start() {
        guard !isWatching else { return }
        if let startupValidationError {
            watcherStatus = .error(startupValidationError)
            return
        }

        watcherStatus = .starting
        let watchURL = URL(fileURLWithPath: resolvedWatchDir)

        let dirWatcher = DirectoryWatcher(directory: watchURL)
        watcher = dirWatcher
        watcherStartedAt = Date()

        let startedAt = watcherStartedAt!
        guard dirWatcher.start(onNewFile: { [weak self] url in
            guard let self else { return }
            Task { @MainActor in
                await self.handleNewFile(url: url, watcherStartedAt: startedAt)
            }
        }) else {
            watcher = nil
            watcherStartedAt = nil
            watcherStatus = .error("Could not watch directory: \(resolvedWatchDir)")
            return
        }

        watcherStatus = .running

        Task {
            await checkAPIHealth()
        }
    }

    func stop() {
        watcher?.stop()
        watcher = nil
        watcherStatus = .stopped
        currentFile = nil
        currentStage = nil
    }

    @discardableResult
    func checkAPIHealth() async -> String? {
        let healthCheckID = UUID()
        apiHealthCheckID = healthCheckID
        apiStatus = "Checking..."
        let healthy = await processor.checkAPIHealth(
            baseURL: resolvedBaseURL,
            apiKey: resolvedAPIKey,
            model: resolvedModel
        )
        guard apiHealthCheckID == healthCheckID else { return nil }
        let result = healthy ? "OK" : "FAIL"
        apiStatus = result
        return result
    }

    func invalidateAPIHealthCheck() {
        apiHealthCheckID = UUID()
        apiStatus = "N/A"
    }

    /// Free-trial usage for the bundled hosted provider. Returns nil for other providers.
    func fetchHostedUsage() async -> HostedUsage? {
        guard resolvedProvider == .chaosHosted else { return nil }
        return try? await processor.fetchUsage(baseURL: resolvedBaseURL, apiKey: resolvedAPIKey)
    }

    private func handleNewFile(url: URL, watcherStartedAt: Date) async {
        currentFile = url.lastPathComponent
        currentStage = .caught

        guard ScreenshotGuard.isEligible(url: url, watcherStartedAt: watcherStartedAt) else {
            currentFile = nil
            currentStage = nil
            return
        }

        await processInput(url: url)
    }

    func retry(_ file: RecentFile) {
        let sourceURL = URL(fileURLWithPath: file.sourcePath)
        guard !file.sourcePath.isEmpty,
              FileManager.default.fileExists(atPath: sourceURL.path)
        else {
            record(RecentFile(
                originalName: file.originalName,
                newName: "",
                path: "",
                sourcePath: file.sourcePath,
                timestamp: Date(),
                duration: 0,
                result: .error("Source image is unavailable")
            ))
            return
        }

        Task {
            await processInput(url: sourceURL)
        }
    }

    func processManualURLs(_ urls: [URL]) {
        let accepted = ImageIntake.acceptedURLs(from: urls)
        guard !accepted.isEmpty else { return }

        Task {
            for url in accepted {
                await processInput(url: url)
            }
        }
    }

    private func processInput(url: URL) async {
        let originalName = url.lastPathComponent
        currentFile = originalName
        currentStage = .analyzing
        totalProcessed += 1

        let hourBucket = Calendar.current.component(.hour, from: Date())
        hourlyThroughput[hourBucket] += 1

        do {
            let result = try await processor.process(
                screenshotURL: url,
                outputDir: URL(fileURLWithPath: resolvedOutputDir),
                baseURL: resolvedBaseURL,
                apiKey: resolvedAPIKey,
                model: resolvedModel,
                language: resolvedLanguage,
                copyToClipboard: resolvedCopyToClipboard,
                namingPolicy: resolvedNamingPolicy
            ) { [weak self] stage in
                await self?.setCurrentStage(stage)
            }

            successes += 1
            lastError = nil
            latencies.append(result.duration)
            if latencies.count > 100 { latencies.removeFirst() }

            successWindow.append(1.0)
            if successWindow.count > 100 { successWindow.removeFirst() }

            let entry = RecentFile(
                originalName: result.originalName,
                newName: result.destinationURL.lastPathComponent,
                path: result.destinationURL.path,
                sourcePath: url.path,
                timestamp: Date(),
                duration: result.duration,
                result: .success
            )
            record(entry)

            currentStage = .success(result.destinationURL.lastPathComponent)
            currentFile = result.destinationURL.lastPathComponent

            if resolvedNotifyOnComplete {
                NotificationService.notifySuccess(
                    originalName: result.originalName,
                    newName: result.destinationURL.lastPathComponent
                )
            }
        } catch {
            errors += 1
            let friendly = FriendlyError(error, provider: resolvedProvider)
            lastError = friendly
            currentStage = .error(friendly.message)

            successWindow.append(0.0)
            if successWindow.count > 100 { successWindow.removeFirst() }

            let entry = RecentFile(
                originalName: originalName,
                newName: "",
                path: "",
                sourcePath: url.path,
                timestamp: Date(),
                duration: 0,
                result: .error(friendly.message)
            )
            record(entry)

            if resolvedNotifyOnComplete {
                NotificationService.notifyError(originalName: originalName, message: friendly.message)
            }
        }
    }

    /// Undo a filing: move the renamed image back to where it came from and drop it from
    /// history. Gives non-technical users a safety net for wrong names.
    func revert(_ file: RecentFile) {
        guard !file.path.isEmpty, !file.sourcePath.isEmpty else { return }
        let current = URL(fileURLWithPath: file.path)
        guard FileManager.default.fileExists(atPath: current.path) else {
            lastError = FriendlyError(
                message: "That file isn't where Chaos left it, so it can't be undone.",
                action: .none
            )
            return
        }
        do {
            try FileRenamer.revert(from: current, toOriginalPath: file.sourcePath)
            recentFiles.removeAll { $0.id == file.id }
            saveHistory()
        } catch {
            lastError = FriendlyError(message: "Couldn't undo that filing.", action: .none)
        }
    }

    /// Correct an AI-generated name in place. Keeps the file in its current folder.
    func rename(_ file: RecentFile, to newBaseName: String) {
        let trimmed = newBaseName.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !file.path.isEmpty, !trimmed.isEmpty else { return }
        let sanitized = SlugSanitizer.sanitize(trimmed)
        guard !sanitized.isEmpty else { return }
        let current = URL(fileURLWithPath: file.path)
        guard FileManager.default.fileExists(atPath: current.path) else {
            lastError = FriendlyError(
                message: "That file isn't where Chaos left it, so it can't be renamed.",
                action: .none
            )
            return
        }
        do {
            let dst = try FileRenamer.rename(at: current, toBaseName: sanitized)
            if let idx = recentFiles.firstIndex(where: { $0.id == file.id }) {
                let old = recentFiles[idx]
                recentFiles[idx] = RecentFile(
                    id: old.id,
                    originalName: old.originalName,
                    newName: dst.lastPathComponent,
                    path: dst.path,
                    sourcePath: old.sourcePath,
                    timestamp: old.timestamp,
                    duration: old.duration,
                    result: old.result
                )
                saveHistory()
            }
        } catch {
            lastError = FriendlyError(message: "Couldn't rename that file.", action: .none)
        }
    }

    private func record(_ entry: RecentFile) {
        recentFiles.insert(entry, at: 0)
        if recentFiles.count > 500 {
            recentFiles = Array(recentFiles.prefix(500))
        }
        saveHistory()
    }

    /// Persist history off the hot path. Writing JSON after every filed screenshot
    /// shouldn't block the next one or the UI, so it runs on a background task against a
    /// snapshot of the current list.
    private func saveHistory() {
        let snapshot = recentFiles
        let store = historyStore
        Task.detached(priority: .utility) {
            do {
                try store.save(snapshot)
            } catch {
                await MainActor.run { [weak self] in self?.apiStatus = "HISTORY FAIL" }
            }
        }
    }

    private func setCurrentStage(_ stage: ProcessingStage) {
        currentStage = stage
    }
}
