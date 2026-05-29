import Foundation
import SwiftUI

@Observable @MainActor
final class AppState {
    var watcherStatus: WatcherStatus = .stopped
    var watcherStartedAt: Date?
    var config: AppConfig = AppConfig()
    var currentFile: String?
    var currentStage: ProcessingStage?
    var recentFiles: [RecentFile] = []
    var apiStatus: String = "N/A"

    var totalProcessed: Int = 0
    var successes: Int = 0
    var errors: Int = 0
    private(set) var latencies: [TimeInterval] = []

    @ObservationIgnored @AppStorage("autoStart") var autoStart = false

    @ObservationIgnored private var watcher: DirectoryWatcher?
    @ObservationIgnored private let processor = FileProcessor()
    @ObservationIgnored private let configService = ConfigService()

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
        let b = config.baseURL?.trimmingCharacters(in: .whitespaces) ?? ""
        return b.isEmpty ? (resolvedProvider.defaultBaseURL ?? "") : b
    }

    var resolvedWatchDir: String {
        let w = config.watchDir?.trimmingCharacters(in: .whitespaces) ?? ""
        return w.isEmpty ? NSHomeDirectory() + "/Desktop" : w
    }

    var resolvedOutputDir: String {
        let o = config.outputDir?.trimmingCharacters(in: .whitespaces) ?? ""
        return o.isEmpty ? NSHomeDirectory() + "/Desktop/vibe-shot-output" : o
    }

    var resolvedLanguage: SlugLanguage {
        SlugLanguage.from(config.language)
    }

    var resolvedCopyToClipboard: Bool {
        config.copyToClipboard ?? false
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

    func loadConfig() {
        config = configService.load()
    }

    func saveConfig() {
        try? configService.save(config)
    }

    func start() {
        guard !isWatching else { return }
        guard let apiKey = config.apiKey, !apiKey.trimmingCharacters(in: .whitespaces).isEmpty else {
            watcherStatus = .error("API key not configured")
            return
        }

        watcherStatus = .starting
        let watchURL = URL(fileURLWithPath: resolvedWatchDir)

        let dirWatcher = DirectoryWatcher(directory: watchURL)
        watcher = dirWatcher
        watcherStartedAt = Date()

        let startedAt = watcherStartedAt!
        dirWatcher.start { [weak self] url in
            guard let self else { return }
            Task { @MainActor in
                await self.handleNewFile(url: url, watcherStartedAt: startedAt)
            }
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

    func checkAPIHealth() async {
        apiStatus = "Checking..."
        let healthy = await processor.checkAPIHealth(
            baseURL: resolvedBaseURL,
            apiKey: config.apiKey ?? "",
            model: resolvedModel
        )
        apiStatus = healthy ? "OK" : "FAIL"
    }

    private func handleNewFile(url: URL, watcherStartedAt: Date) async {
        guard ScreenshotGuard.isEligible(url: url, watcherStartedAt: watcherStartedAt) else {
            return
        }

        let originalName = url.lastPathComponent
        currentFile = originalName
        currentStage = .analyzing
        totalProcessed += 1

        do {
            currentStage = .renaming

            let result = try await processor.process(
                screenshotURL: url,
                outputDir: URL(fileURLWithPath: resolvedOutputDir),
                baseURL: resolvedBaseURL,
                apiKey: config.apiKey ?? "",
                model: resolvedModel,
                language: resolvedLanguage,
                copyToClipboard: resolvedCopyToClipboard
            )

            successes += 1
            latencies.append(result.duration)
            if latencies.count > 100 { latencies.removeFirst() }

            let entry = RecentFile(
                originalName: result.originalName,
                newName: result.destinationURL.lastPathComponent,
                path: result.destinationURL.path,
                timestamp: Date(),
                duration: result.duration,
                result: .success
            )
            recentFiles.insert(entry, at: 0)
            if recentFiles.count > 50 { recentFiles = Array(recentFiles.prefix(50)) }

            currentStage = .success(result.destinationURL.lastPathComponent)
            currentFile = result.destinationURL.lastPathComponent
        } catch {
            errors += 1
            currentStage = .error(error.localizedDescription)

            let entry = RecentFile(
                originalName: originalName,
                newName: "",
                path: "",
                timestamp: Date(),
                duration: 0,
                result: .error(error.localizedDescription)
            )
            recentFiles.insert(entry, at: 0)
            if recentFiles.count > 50 { recentFiles = Array(recentFiles.prefix(50)) }
        }
    }
}
