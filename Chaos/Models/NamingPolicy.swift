import Foundation

enum SubfolderRule: String, CaseIterable, Identifiable {
    case none
    case day
    case month

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .none: "No Subfolders"
        case .day: "By Day"
        case .month: "By Month"
        }
    }

    static func from(_ raw: String?) -> SubfolderRule {
        guard let raw else { return .none }
        return SubfolderRule(rawValue: raw) ?? .none
    }
}

struct NamingPolicy {
    static let defaultTemplate = "{slug}_{time}"

    let template: String?
    let subfolderRule: SubfolderRule
    let timeZone: TimeZone

    init(
        template: String? = nil,
        subfolderRule: SubfolderRule = .none,
        timeZone: TimeZone = .current
    ) {
        self.template = template
        self.subfolderRule = subfolderRule
        self.timeZone = timeZone
    }

    func renderedBaseName(slug: String, date: Date) -> String {
        let trimmed = template?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let source = trimmed.isEmpty ? Self.defaultTemplate : trimmed
        let rendered = source
            .replacingOccurrences(of: "{slug}", with: slug)
            .replacingOccurrences(of: "{date}", with: format(date, as: "yyyy-MM-dd"))
            .replacingOccurrences(of: "{time}", with: format(date, as: "HHmmss"))
        return SlugSanitizer.sanitize(rendered)
    }

    func outputDirectory(base: URL, date: Date) -> URL {
        switch subfolderRule {
        case .none:
            base
        case .day:
            base.appendingPathComponent(format(date, as: "yyyy-MM-dd"))
        case .month:
            base.appendingPathComponent(format(date, as: "yyyy-MM"))
        }
    }

    private func format(_ date: Date, as format: String) -> String {
        Self.formatter(format: format, timeZone: timeZone).string(from: date)
    }

    // DateFormatter is expensive to build and gets called a few times per filed image.
    // Cache one per (format, time zone) pair behind a lock.
    private static let cacheLock = NSLock()
    nonisolated(unsafe) private static var formatters: [String: DateFormatter] = [:]

    private static func formatter(format: String, timeZone: TimeZone) -> DateFormatter {
        let key = "\(format)|\(timeZone.identifier)"
        cacheLock.lock()
        defer { cacheLock.unlock() }
        if let cached = formatters[key] { return cached }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = format
        formatters[key] = formatter
        return formatter
    }
}
