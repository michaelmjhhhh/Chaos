import Foundation

enum ScreenshotGuard {
    private static let screenshotPrefixes = ["Screenshot", "屏幕快照", "截屏"]
    private static let pngMagic: [UInt8] = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]
    private static let minSize = 20 * 1024
    private static let maxSize = 25 * 1024 * 1024

    static func isScreenshotCandidate(_ name: String) -> Bool {
        screenshotPrefixes.contains { name.hasPrefix($0) }
    }

    static func isEligible(url: URL, watcherStartedAt: Date) -> Bool {
        let name = url.lastPathComponent
        guard isScreenshotCandidate(name) else { return false }
        guard url.pathExtension.lowercased() == "png" else { return false }

        let fm = FileManager.default
        guard let attrs = try? fm.attributesOfItem(atPath: url.path),
              let type = attrs[.type] as? FileAttributeType,
              type == .typeRegular
        else {
            return false
        }

        guard let modDate = attrs[.modificationDate] as? Date,
              modDate >= watcherStartedAt.addingTimeInterval(-2)
        else {
            return false
        }

        guard let size = attrs[.size] as? Int,
              size >= minSize, size <= maxSize
        else {
            return false
        }

        guard let handle = try? FileHandle(forReadingFrom: url) else { return false }
        defer { try? handle.close() }
        guard let headerData = try? handle.read(upToCount: pngMagic.count),
              headerData.count == pngMagic.count
        else {
            return false
        }
        return headerData.elementsEqual(pngMagic)
    }
}
