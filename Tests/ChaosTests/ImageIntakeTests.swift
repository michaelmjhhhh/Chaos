import XCTest
@testable import Chaos

final class ImageIntakeTests: XCTestCase {
    func testAcceptsSupportedImageExtensionsCaseInsensitively() {
        for filename in ["image.png", "image.JPG", "image.jpeg", "image.HEIC", "image.webp"] {
            XCTAssertTrue(ImageIntake.accepts(url: URL(fileURLWithPath: filename)), filename)
        }
    }

    func testRejectsUnsupportedExtensions() {
        XCTAssertFalse(ImageIntake.accepts(url: URL(fileURLWithPath: "notes.pdf")))
    }
}
