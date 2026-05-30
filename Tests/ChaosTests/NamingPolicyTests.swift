import XCTest
@testable import Chaos

final class NamingPolicyTests: XCTestCase {
    private let date = Date(timeIntervalSince1970: 1_769_947_445)
    private let timeZone = TimeZone(secondsFromGMT: 0)!

    func testDefaultTemplateRendersSlugAndTime() {
        let policy = NamingPolicy(template: nil, subfolderRule: .none, timeZone: timeZone)

        XCTAssertEqual(policy.renderedBaseName(slug: "terminal-log", date: date), "terminal-log-120405")
    }

    func testTemplateRendersDateAndTimeTokens() {
        let policy = NamingPolicy(
            template: "{date}_{slug}_{time}",
            subfolderRule: .none,
            timeZone: timeZone
        )

        XCTAssertEqual(
            policy.renderedBaseName(slug: "terminal-log", date: date),
            "2026-02-01-terminal-log-120405"
        )
    }

    func testEmptyTemplateFallsBackToDefault() {
        let policy = NamingPolicy(template: "  ", subfolderRule: .none, timeZone: timeZone)

        XCTAssertEqual(policy.renderedBaseName(slug: "terminal-log", date: date), "terminal-log-120405")
    }

    func testDayAndMonthRulesResolveDatedOutputDirectories() {
        let base = URL(fileURLWithPath: "/output")

        XCTAssertEqual(
            NamingPolicy(template: nil, subfolderRule: .day, timeZone: timeZone)
                .outputDirectory(base: base, date: date).path,
            "/output/2026-02-01"
        )
        XCTAssertEqual(
            NamingPolicy(template: nil, subfolderRule: .month, timeZone: timeZone)
                .outputDirectory(base: base, date: date).path,
            "/output/2026-02"
        )
    }

    func testFileRenamerPreservesExtensionAndAddsCollisionSuffix() throws {
        let directory = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString)
        let output = directory.appendingPathComponent("output")
        let first = directory.appendingPathComponent("first.JPG")
        let second = directory.appendingPathComponent("second.JPG")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try Data("first".utf8).write(to: first)
        try Data("second".utf8).write(to: second)
        defer { try? FileManager.default.removeItem(at: directory) }

        let firstDestination = try FileRenamer.moveScreenshot(
            from: first,
            toDirectory: output,
            baseName: "terminal-log-123045"
        )
        let secondDestination = try FileRenamer.moveScreenshot(
            from: second,
            toDirectory: output,
            baseName: "terminal-log-123045"
        )

        XCTAssertEqual(firstDestination.lastPathComponent, "terminal-log-123045.jpg")
        XCTAssertEqual(secondDestination.lastPathComponent, "terminal-log-123045-2.jpg")
    }
}
