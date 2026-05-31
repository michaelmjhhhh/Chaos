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

    func testAcceptedURLsFiltersUnsupportedFilesAndPreservesSelectionOrder() {
        let urls = [
            URL(fileURLWithPath: "first.PNG"),
            URL(fileURLWithPath: "notes.pdf"),
            URL(fileURLWithPath: "second.heic"),
            URL(fileURLWithPath: "third.webp"),
        ]

        XCTAssertEqual(
            ImageIntake.acceptedURLs(from: urls).map(\.lastPathComponent),
            ["first.PNG", "second.heic", "third.webp"]
        )
    }
}
