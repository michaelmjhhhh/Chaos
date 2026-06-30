import XCTest
@testable import Chaos

final class ConfigServiceTests: XCTestCase {
    func testLoadMigratesLegacyConfigWhenChaosConfigIsMissing() throws {
        let paths = try TemporaryConfigPaths()
        defer { paths.remove() }
        let legacyConfig = AppConfig(provider: "openai", apiKey: "legacy-key")
        try paths.write(legacyConfig, to: paths.legacyURL)

        let loaded = ConfigService(
            configPath: paths.chaosURL,
            legacyConfigPath: paths.legacyURL
        ).load()

        XCTAssertEqual(loaded, legacyConfig)
        XCTAssertEqual(try paths.read(from: paths.chaosURL), legacyConfig)
        XCTAssertEqual(try paths.read(from: paths.legacyURL), legacyConfig)
    }

    func testLoadPrefersExistingChaosConfig() throws {
        let paths = try TemporaryConfigPaths()
        defer { paths.remove() }
        let chaosConfig = AppConfig(provider: "openai", apiKey: "chaos-key")
        let legacyConfig = AppConfig(provider: "deepseek", apiKey: "legacy-key")
        try paths.write(chaosConfig, to: paths.chaosURL)
        try paths.write(legacyConfig, to: paths.legacyURL)

        let loaded = ConfigService(
            configPath: paths.chaosURL,
            legacyConfigPath: paths.legacyURL
        ).load()

        XCTAssertEqual(loaded, chaosConfig)
        XCTAssertEqual(try paths.read(from: paths.legacyURL), legacyConfig)
    }

    func testRoundTripsCustomPromptFields() throws {
        let paths = try TemporaryConfigPaths()
        defer { paths.remove() }
        let config = AppConfig(
            provider: "openai",
            useCustomPrompt: true,
            customPrompt: "Name files by the visible window title."
        )
        try paths.write(config, to: paths.chaosURL)

        let loaded = ConfigService(
            configPath: paths.chaosURL,
            legacyConfigPath: paths.legacyURL
        ).load()

        XCTAssertEqual(loaded.useCustomPrompt, true)
        XCTAssertEqual(loaded.customPrompt, "Name files by the visible window title.")
    }

    func testCustomPromptEncodesWithSnakeCaseKeys() throws {
        let config = AppConfig(useCustomPrompt: true, customPrompt: "x")
        let json = try String(decoding: JSONEncoder().encode(config), as: UTF8.self)
        XCTAssertTrue(json.contains("use_custom_prompt"))
        XCTAssertTrue(json.contains("custom_prompt"))
    }
}

private struct TemporaryConfigPaths {
    let directory: URL
    let chaosURL: URL
    let legacyURL: URL

    init() throws {
        directory = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString)
        chaosURL = directory.appendingPathComponent("chaos/config.json")
        legacyURL = directory.appendingPathComponent("vibe-shot/config.json")
        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true
        )
    }

    func write(_ config: AppConfig, to url: URL) throws {
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        try JSONEncoder().encode(config).write(to: url)
    }

    func read(from url: URL) throws -> AppConfig {
        try JSONDecoder().decode(AppConfig.self, from: Data(contentsOf: url))
    }

    func remove() {
        try? FileManager.default.removeItem(at: directory)
    }
}
