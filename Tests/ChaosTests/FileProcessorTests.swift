import XCTest
@testable import Chaos

final class FileProcessorTests: XCTestCase {
    func testHealthCheckRejectsEmptyBaseURL() async {
        let processor = FileProcessor()
        let healthy = await processor.checkAPIHealth(
            baseURL: "",
            apiKey: "test-key",
            model: "test-model"
        )
        XCTAssertFalse(healthy)
    }

    func testGenerateSlugRejectsInvalidProviderBaseURLs() async {
        let client = VisionAPIClient()

        for baseURL in ["", "ftp://example.test", "https:///missing-host"] {
            do {
                _ = try await client.generateSlug(
                    imageBase64: "image",
                    baseURL: baseURL,
                    apiKey: "test-key",
                    model: "test-model",
                    language: .en
                )
                XCTFail("Expected invalid provider base URL error for \(baseURL)")
            } catch {
                XCTAssertEqual(
                    error.localizedDescription,
                    "API error: Invalid provider base URL"
                )
            }
        }
    }

    func testReportsRenamingAfterAnalysis() async throws {
        let temporaryDirectory = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString)
        let screenshotURL = temporaryDirectory.appendingPathComponent("Screenshot.png")
        let outputDirectory = temporaryDirectory.appendingPathComponent("output")
        try FileManager.default.createDirectory(
            at: temporaryDirectory,
            withIntermediateDirectories: true
        )
        try Data("image".utf8).write(to: screenshotURL)
        defer { try? FileManager.default.removeItem(at: temporaryDirectory) }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubURLProtocol.self]
        let apiClient = VisionAPIClient(session: URLSession(configuration: configuration))
        let processor = FileProcessor(apiClient: apiClient)
        let recorder = StageRecorder()

        _ = try await processor.process(
            screenshotURL: screenshotURL,
            outputDir: outputDirectory,
            baseURL: "https://example.test/v1",
            apiKey: "test-key",
            model: "test-model",
            language: .en,
            copyToClipboard: false
        ) { stage in
            await recorder.append(stage)
        }

        let stages = await recorder.stages
        XCTAssertEqual(stages, [.renaming])
    }
}

private actor StageRecorder {
    private(set) var stages: [ProcessingStage] = []

    func append(_ stage: ProcessingStage) {
        stages.append(stage)
    }
}

private final class StubURLProtocol: URLProtocol {
    override class func canInit(with request: URLRequest) -> Bool {
        true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        request
    }

    override func startLoading() {
        let data = Data(#"{"choices":[{"message":{"content":"terminal-git-log"}}]}"#.utf8)
        let response = HTTPURLResponse(
            url: request.url!,
            statusCode: 200,
            httpVersion: nil,
            headerFields: ["Content-Type": "application/json"]
        )!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: data)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}
