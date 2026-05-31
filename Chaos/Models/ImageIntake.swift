import Foundation

enum ImageIntake {
    private static let supportedExtensions: Set<String> = [
        "png", "jpg", "jpeg", "heic", "webp",
    ]

    static func accepts(url: URL) -> Bool {
        supportedExtensions.contains(url.pathExtension.lowercased())
    }

    static func acceptedURLs(from urls: [URL]) -> [URL] {
        urls.filter(accepts)
    }
}
