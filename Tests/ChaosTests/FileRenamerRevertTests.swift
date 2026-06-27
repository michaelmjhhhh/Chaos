import XCTest
@testable import Chaos

final class FileRenamerRevertTests: XCTestCase {
    private var tmp: URL!

    override func setUpWithError() throws {
        tmp = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: tmp)
    }

    private func makeFile(_ name: String, in dir: URL) throws -> URL {
        let url = dir.appendingPathComponent(name)
        try Data("x".utf8).write(to: url)
        return url
    }

    func testRevertRestoresOriginalLocationAndName() throws {
        let originalDir = tmp.appendingPathComponent("desktop", isDirectory: true)
        let outputDir = tmp.appendingPathComponent("output", isDirectory: true)
        try FileManager.default.createDirectory(at: originalDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: outputDir, withIntermediateDirectories: true)

        let filed = try makeFile("nice-name.png", in: outputDir)
        let originalPath = originalDir.appendingPathComponent("Screenshot.png").path

        let restored = try FileRenamer.revert(from: filed, toOriginalPath: originalPath)

        XCTAssertEqual(restored.path, originalPath)
        XCTAssertTrue(FileManager.default.fileExists(atPath: originalPath))
        XCTAssertFalse(FileManager.default.fileExists(atPath: filed.path))
    }

    func testRevertAvoidsClobberingWhenOriginalSpotTaken() throws {
        let dir = tmp.appendingPathComponent("d", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        _ = try makeFile("Screenshot.png", in: dir) // occupy original spot
        let filed = try makeFile("nice.png", in: tmp)

        let restored = try FileRenamer.revert(
            from: filed,
            toOriginalPath: dir.appendingPathComponent("Screenshot.png").path
        )

        XCTAssertTrue(restored.lastPathComponent.contains("restored"))
        XCTAssertTrue(FileManager.default.fileExists(atPath: restored.path))
    }

    func testRenameMovesFileToNewBaseNameSameExtension() throws {
        let filed = try makeFile("old-name.png", in: tmp)

        let renamed = try FileRenamer.rename(at: filed, toBaseName: "brand-new-name")

        XCTAssertEqual(renamed.lastPathComponent, "brand-new-name.png")
        XCTAssertTrue(FileManager.default.fileExists(atPath: renamed.path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: filed.path))
    }

    func testRenameToSameNameIsANoOp() throws {
        let filed = try makeFile("same.png", in: tmp)
        let renamed = try FileRenamer.rename(at: filed, toBaseName: "same")
        XCTAssertEqual(renamed.path, filed.path)
        XCTAssertTrue(FileManager.default.fileExists(atPath: renamed.path))
    }
}
