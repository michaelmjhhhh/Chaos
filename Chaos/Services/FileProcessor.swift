import Foundation

actor FileProcessor {
    private let apiClient: VisionAPIClient

    init(apiClient: VisionAPIClient = VisionAPIClient()) {
        self.apiClient = apiClient
    }

    struct ProcessResult {
        let originalName: String
        let destinationURL: URL
        let duration: TimeInterval
    }

    func process(
        screenshotURL: URL,
        outputDir: URL,
        baseURL: String,
        apiKey: String,
        model: String,
        language: SlugLanguage,
        copyToClipboard: Bool,
        namingPolicy: NamingPolicy = NamingPolicy(),
        onStageChange: @Sendable (ProcessingStage) async -> Void = { _ in }
    ) async throws -> ProcessResult {
        let start = Date()
        let originalName = screenshotURL.lastPathComponent

        try await waitUntilStable(url: screenshotURL)

        let prepared = VisionImage.prepare(url: screenshotURL)

        let rawSlug = try await apiClient.generateSlug(
            imageBase64: prepared.base64,
            mimeType: prepared.mimeType,
            baseURL: baseURL,
            apiKey: apiKey,
            model: model,
            language: language
        )
        let slug = SlugSanitizer.sanitize(rawSlug)
        let processingDate = Date()
        let baseName = namingPolicy.renderedBaseName(slug: slug, date: processingDate)
        let destinationDirectory = namingPolicy.outputDirectory(base: outputDir, date: processingDate)

        await onStageChange(.renaming)
        let destination = try FileRenamer.moveScreenshot(
            from: screenshotURL,
            toDirectory: destinationDirectory,
            baseName: baseName
        )

        if copyToClipboard {
            await onStageChange(.clipboard)
            try ClipboardService.copyImage(at: destination)
        }

        SoundService.playGlass()

        return ProcessResult(
            originalName: originalName,
            destinationURL: destination,
            duration: Date().timeIntervalSince(start)
        )
    }

    func checkAPIHealth(
        baseURL: String,
        apiKey: String,
        model: String
    ) async -> Bool {
        (try? await apiClient.checkHealth(
            baseURL: baseURL,
            apiKey: apiKey,
            model: model
        )) ?? false
    }

    /// Wait until the screenshot has finished being written. Screenshots land almost
    /// instantly, so we poll on a short interval and return the moment the size holds
    /// steady — typically after ~0.1s rather than burning a fixed budget.
    private func waitUntilStable(url: URL, attempts: Int = 12, interval: TimeInterval = 0.1) async throws {
        var previousSize: Int64 = -1
        let fm = FileManager.default

        for _ in 0..<attempts {
            if let attrs = try? fm.attributesOfItem(atPath: url.path),
               let size = attrs[.size] as? Int64 {
                if size > 0 && size == previousSize {
                    return
                }
                previousSize = size
            }
            try await Task.sleep(for: .milliseconds(Int(interval * 1000)))
        }

        throw ChaosError.fileNotStable(url.lastPathComponent)
    }
}
