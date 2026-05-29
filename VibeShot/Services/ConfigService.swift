import Foundation

struct ConfigService {
    static let configPath: URL = {
        let appSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!
        return appSupport
            .appendingPathComponent("vibe-shot")
            .appendingPathComponent("config.json")
    }()

    func load() -> AppConfig {
        guard let data = try? Data(contentsOf: Self.configPath) else {
            return AppConfig()
        }
        guard !data.isEmpty else { return AppConfig() }
        return (try? JSONDecoder().decode(AppConfig.self, from: data)) ?? AppConfig()
    }

    func save(_ config: AppConfig) throws {
        let dir = Self.configPath.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let data = try encoder.encode(config)
        try data.write(to: Self.configPath, options: .atomic)
    }
}
