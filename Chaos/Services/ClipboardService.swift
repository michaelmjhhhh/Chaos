import AppKit

enum ClipboardService {
    static func copyImage(at url: URL) throws {
        guard let image = NSImage(contentsOf: url) else {
            throw ChaosError.apiError("Failed to load image for clipboard")
        }
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        guard pasteboard.writeObjects([image]) else {
            throw ChaosError.apiError("Failed to write image to pasteboard")
        }
    }
}
