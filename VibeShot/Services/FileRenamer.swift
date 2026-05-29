import Foundation

enum FileRenamer {
    static func moveScreenshot(from src: URL, toDirectory outputDir: URL, slug: String) throws -> URL {
        let fm = FileManager.default
        try fm.createDirectory(at: outputDir, withIntermediateDirectories: true)

        let formatter = DateFormatter()
        formatter.dateFormat = "HHmmss"
        let ts = formatter.string(from: Date())

        for i in 0..<100 {
            let name: String
            if i == 0 {
                name = "\(slug)_\(ts).png"
            } else {
                name = "\(slug)-\(i + 1)_\(ts).png"
            }
            let dst = outputDir.appendingPathComponent(name)

            if fm.fileExists(atPath: dst.path) { continue }

            do {
                try fm.moveItem(at: src, to: dst)
                return dst
            } catch let error as NSError where error.code == NSFileWriteFileExistsError {
                continue
            } catch let error as NSError where error.domain == NSCocoaErrorDomain && error.code == 512 {
                // Cross-volume: copy then delete
                try fm.copyItem(at: src, to: dst)
                try fm.removeItem(at: src)
                return dst
            }
        }

        throw VibeShotError.renameCollision(slug)
    }
}

enum VibeShotError: LocalizedError {
    case renameCollision(String)
    case apiError(String)
    case configError(String)
    case fileNotStable(String)

    var errorDescription: String? {
        switch self {
        case .renameCollision(let slug): "Failed to generate non-colliding name for \"\(slug)\""
        case .apiError(let msg): "API error: \(msg)"
        case .configError(let msg): "Config error: \(msg)"
        case .fileNotStable(let path): "File did not stabilize: \(path)"
        }
    }
}
