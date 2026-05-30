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

        let data = try Data(contentsOf: screenshotURL)
        let base64 = data.base64EncodedString()

        let rawSlug = try await apiClient.generateSlug(
            imageBase64: base64,
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

    private func waitUntilStable(url: URL, attempts: Int = 10, interval: TimeInterval = 0.2) async throws {
        var previousSize: Int64 = -1
        let fm = FileManager.default

        for _ in 0..<attempts {
            guard let attrs = try? fm.attributesOfItem(atPath: url.path),
                  let size = attrs[.size] as? Int64 else {
                try await Task.sleep(for: .milliseconds(Int(interval * 1000)))
                continue
            }

            if size > 0 && size == previousSize {
                return
            }
            previousSize = size
            try await Task.sleep(for: .milliseconds(Int(interval * 1000)))
        }

        throw ChaosError.fileNotStable(url.lastPathComponent)
    }
}
