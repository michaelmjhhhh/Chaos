import Foundation
import ImageIO
import AppKit
import UniformTypeIdentifiers

/// Prepares a screenshot for the vision API.
///
/// Screenshots from Retina displays are frequently 5–15 MB PNGs. Sending the full image
/// dominates the per-file latency twice over: a large base64 upload, and slower model
/// inference on a huge image. A name only needs to read what's on screen, not pixel
/// detail — so we downscale the long edge to a sane bound and re-encode as JPEG before
/// base64. A 10 MB PNG typically becomes a ~100–300 KB JPEG.
///
/// Uses ImageIO's thumbnail path, which downsamples without fully decoding the original
/// into memory.
enum VisionImage {
    struct Prepared {
        let base64: String
        let mimeType: String
    }

    /// Downscale + compress for the API. Falls back to the original bytes if anything
    /// about the fast path fails, so a screenshot is never dropped for a perf reason.
    static func prepare(
        url: URL,
        maxPixelSize: Int = 1280,
        quality: CGFloat = 0.7
    ) -> Prepared {
        if let jpeg = downsampledJPEG(url: url, maxPixelSize: maxPixelSize, quality: quality) {
            return Prepared(base64: jpeg.base64EncodedString(), mimeType: "image/jpeg")
        }
        // Fallback: send the original file untouched.
        if let data = try? Data(contentsOf: url) {
            return Prepared(base64: data.base64EncodedString(), mimeType: mimeType(for: url))
        }
        return Prepared(base64: "", mimeType: "image/png")
    }

    private static func downsampledJPEG(url: URL, maxPixelSize: Int, quality: CGFloat) -> Data? {
        guard let source = CGImageSourceCreateWithURL(url as CFURL, nil) else { return nil }
        let options: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: maxPixelSize
        ]
        guard let thumbnail = CGImageSourceCreateThumbnailAtIndex(source, 0, options as CFDictionary) else {
            return nil
        }
        let rep = NSBitmapImageRep(cgImage: thumbnail)
        return rep.representation(using: .jpeg, properties: [.compressionFactor: quality])
    }

    private static func mimeType(for url: URL) -> String {
        guard let type = UTType(filenameExtension: url.pathExtension) else { return "image/png" }
        if type.conforms(to: .jpeg) { return "image/jpeg" }
        if type.conforms(to: .webP) { return "image/webp" }
        return "image/png"
    }
}
