import Foundation

struct ConfigService {
    static let defaultConfigPath: URL = {
        let appSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!
        return appSupport
            .appendingPathComponent("chaos")
            .appendingPathComponent("config.json")
    }()

    static let legacyConfigPath: URL = {
        let appSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!
        return appSupport
            .appendingPathComponent("vibe-shot")
            .appendingPathComponent("config.json")
    }()

    let configPath: URL
    let legacyConfigPath: URL

    init(
        configPath: URL = Self.defaultConfigPath,
        legacyConfigPath: URL = Self.legacyConfigPath
    ) {
        self.configPath = configPath
        self.legacyConfigPath = legacyConfigPath
    }

    func load() -> AppConfig {
        migrateLegacyConfigIfNeeded()

        guard let data = try? Data(contentsOf: configPath) else {
            return AppConfig()
        }
        guard !data.isEmpty else { return AppConfig() }
        return (try? JSONDecoder().decode(AppConfig.self, from: data)) ?? AppConfig()
    }

    func save(_ config: AppConfig) throws {
        let dir = configPath.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let data = try encoder.encode(config)
        try data.write(to: configPath, options: .atomic)
    }

    private func migrateLegacyConfigIfNeeded() {
        let fm = FileManager.default
        guard !fm.fileExists(atPath: configPath.path),
              fm.fileExists(atPath: legacyConfigPath.path) else {
            return
        }

        do {
            try fm.createDirectory(
                at: configPath.deletingLastPathComponent(),
                withIntermediateDirectories: true
            )
            try fm.copyItem(at: legacyConfigPath, to: configPath)
        } catch {
            return
        }
    }
}
