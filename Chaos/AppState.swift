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
    /// While a manual/batch run is in flight, the 1-based index and total; nil otherwise.
    var batchProgress: (index: Int, total: Int)?
    var recentFiles: [RecentFile] = []
    var apiStatus: String = "N/A"

    var session = SessionMeta()
    var hourlyThroughput: [Int] = Array(repeating: 0, count: 24)
    var successWindow: [Double] = []

    /// Bumped after any durable history change (record, revert, rename) completes. The Insights
    /// page watches this to know when to recompute its snapshot.
    private(set) var historyRevision = 0

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
    /// Opened lazily in `loadConfig()` rather than `init` so unit tests that construct
    /// `AppState` directly don't touch the on-disk database.
    @ObservationIgnored private var historyDatabase: HistoryDatabase?
    @ObservationIgnored private var apiHealthCheckID = UUID()
    @ObservationIgnored private var batchTask: Task<Void, Never>?

    /// Read-only analytics over the full history, backed by the same database queue used for
    /// writes. Nil until the database is opened.
    var insightsRepository: InsightsRepository? {
        historyDatabase.map { InsightsRepository(dbQueue: $0.dbQueue) }
    }

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

    /// The user's custom system prompt when the feature is enabled and the text is non-blank;
    /// otherwise nil, meaning the built-in default prompt is used. This nil is the single signal
    /// the pipeline keys off of.
    var resolvedCustomPrompt: String? {
        guard config.useCustomPrompt == true,
              let prompt = config.customPrompt?.trimmingCharacters(in: .whitespacesAndNewlines),
              !prompt.isEmpty
        else {
            return nil
        }
        return prompt
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
        applyAppearance()
        openHistoryDatabaseIfNeeded()
        loadRecentFiles()
    }

    private func openHistoryDatabaseIfNeeded() {
        guard historyDatabase == nil else { return }
        do {
            historyDatabase = try HistoryDatabase()
        } catch {
            apiStatus = "HISTORY FAIL"
        }
    }

    /// Load the most-recent working set for the Dashboard and Pipeline. Insights reads the full
    /// table separately via `insightsRepository`.
    private func loadRecentFiles() {
        guard let db = historyDatabase else { return }
        Task {
            recentFiles = await (try? db.recentFiles(limit: 500)) ?? []
        }
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
        processBatch(accepted)
    }

    /// Failed history entries that can be re-run because their original image is still
    /// on disk, de-duplicated by source path (the same source can appear after retries).
    var retryableFailures: [RecentFile] {
        var seen = Set<String>()
        return recentFiles.filter { file in
            guard file.isError,
                  !file.sourcePath.isEmpty,
                  FileManager.default.fileExists(atPath: file.sourcePath),
                  !seen.contains(file.sourcePath)
            else { return false }
            seen.insert(file.sourcePath)
            return true
        }
    }

    func retryAllFailures() {
        let urls = retryableFailures.map { URL(fileURLWithPath: $0.sourcePath) }
        guard !urls.isEmpty else { return }
        processBatch(urls)
    }

    /// Run a set of images through the pipeline one at a time, publishing N-of-M
    /// progress for the UI. Shared by drag-drop, the Organize picker, and Retry all.
    /// Ignores re-entry while a batch is already running so two interleaved runs can't
    /// scramble `batchProgress`; the `defer` guarantees the indicator always resets.
    private func processBatch(_ urls: [URL]) {
        guard batchTask == nil else { return }
        batchTask = Task {
            defer {
                batchProgress = nil
                batchTask = nil
            }
            let total = urls.count
            for (i, url) in urls.enumerated() {
                batchProgress = (index: i + 1, total: total)
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
                namingPolicy: resolvedNamingPolicy,
                customSystemPrompt: resolvedCustomPrompt
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
            announce("Filed as \(result.destinationURL.lastPathComponent)")

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
            announce("Couldn't file \(originalName)")

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
            persist { try await $0.delete(id: file.id) }
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
                persist { try await $0.update(id: old.id, newName: dst.lastPathComponent, path: dst.path) }
            }
        } catch {
            lastError = FriendlyError(message: "Couldn't rename that file.", action: .none)
        }
    }

    private func record(_ entry: RecentFile) {
        // Keep the in-memory working set responsive immediately; the database write happens
        // off the main thread via GRDB's queue.
        recentFiles.insert(entry, at: 0)
        if recentFiles.count > 500 {
            recentFiles = Array(recentFiles.prefix(500))
        }
        persist { try await $0.insert(entry) }
    }

    /// Run a durable history mutation against the database off the hot path, then signal the
    /// Insights page to refresh. A write failure surfaces the same `"HISTORY FAIL"` status the
    /// old JSON path used.
    private func persist(_ change: @escaping (HistoryDatabase) async throws -> Void) {
        guard let db = historyDatabase else {
            apiStatus = "HISTORY FAIL"
            return
        }
        Task {
            do {
                try await change(db)
                historyRevision += 1
            } catch {
                apiStatus = "HISTORY FAIL"
            }
        }
    }

    private func setCurrentStage(_ stage: ProcessingStage) {
        currentStage = stage
    }

    /// Speak a brief status update to VoiceOver users. Low priority so a busy batch
    /// doesn't interrupt whatever they're reading.
    private func announce(_ message: String) {
        NSAccessibility.post(
            element: NSApp as Any,
            notification: .announcementRequested,
            userInfo: [
                .announcement: message,
                .priority: NSAccessibilityPriorityLevel.low.rawValue
            ]
        )
    }
}
