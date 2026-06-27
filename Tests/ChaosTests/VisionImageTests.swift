import XCTest
import AppKit
import ImageIO
@testable import Chaos

final class VisionImageTests: XCTestCase {
    private var tmp: URL!

    override func setUpWithError() throws {
        tmp = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: tmp)
    }

    private func writePNG(width: Int, height: Int) throws -> (url: URL, bytes: Int) {
        let size = NSSize(width: width, height: height)
        let image = NSImage(size: size)
        image.lockFocus()
        NSColor.systemBlue.setFill()
        NSRect(origin: .zero, size: size).fill()
        image.unlockFocus()

        guard let tiff = image.tiffRepresentation,
              let rep = NSBitmapImageRep(data: tiff),
              let png = rep.representation(using: .png, properties: [:]) else {
            throw XCTSkip("Could not synthesize a PNG on this host")
        }
        let url = tmp.appendingPathComponent("shot.png")
        try png.write(to: url)
        return (url, png.count)
    }

    func testPrepareDownscalesLargeScreenshotToJPEG() throws {
        let (url, originalBytes) = try writePNG(width: 2400, height: 1600)

        let prepared = VisionImage.prepare(url: url, maxPixelSize: 512, quality: 0.6)

        XCTAssertEqual(prepared.mimeType, "image/jpeg")
        let decoded = try XCTUnwrap(Data(base64Encoded: prepared.base64))
        XCTAssertFalse(decoded.isEmpty)
        XCTAssertLessThan(decoded.count, originalBytes, "downscaled JPEG should be smaller than the source PNG")

        let source = try XCTUnwrap(CGImageSourceCreateWithData(decoded as CFData, nil))
        let cg = try XCTUnwrap(CGImageSourceCreateImageAtIndex(source, 0, nil))
        XCTAssertLessThanOrEqual(max(cg.width, cg.height), 512, "longest edge should be bounded")
    }

    func testPrepareFallsBackToOriginalBytesForNonImage() throws {
        let url = tmp.appendingPathComponent("broken.png")
        let raw = Data("not actually an image".utf8)
        try raw.write(to: url)

        let prepared = VisionImage.prepare(url: url)

        XCTAssertEqual(prepared.mimeType, "image/png")
        let decoded = try XCTUnwrap(Data(base64Encoded: prepared.base64))
        XCTAssertEqual(decoded, raw)
    }
}
