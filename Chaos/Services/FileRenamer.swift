import Foundation

enum FileRenamer {
    static func moveScreenshot(from src: URL, toDirectory outputDir: URL, baseName: String) throws -> URL {
        let fm = FileManager.default
        try fm.createDirectory(at: outputDir, withIntermediateDirectories: true)
        let ext = src.pathExtension.isEmpty ? "png" : src.pathExtension.lowercased()

        for i in 0 ..< 100 {
            let name = if i == 0 {
                "\(baseName).\(ext)"
            } else {
                "\(baseName)-\(i + 1).\(ext)"
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

        throw ChaosError.renameCollision(baseName)
    }

    /// Move a previously filed image back to its original location and name — the "undo"
    /// behind Revert. Falls back to a collision-free name if the original spot is taken.
    @discardableResult
    static func revert(from current: URL, toOriginalPath originalPath: String) throws -> URL {
        let fm = FileManager.default
        let target = URL(fileURLWithPath: originalPath)
        try fm.createDirectory(at: target.deletingLastPathComponent(), withIntermediateDirectories: true)

        if !fm.fileExists(atPath: target.path) {
            try moveItemCrossVolume(from: current, to: target)
            return target
        }

        // Original location is occupied; restore alongside it without clobbering.
        let dir = target.deletingLastPathComponent()
        let base = target.deletingPathExtension().lastPathComponent
        let ext = target.pathExtension.isEmpty ? "png" : target.pathExtension
        return try moveToFreeName(from: current, inDirectory: dir, baseName: "\(base)-restored", ext: ext)
    }

    /// Rename an already-filed image in place (same folder, new base name) — used by the
    /// inline "Rename" correction. Returns the new location.
    @discardableResult
    static func rename(at current: URL, toBaseName baseName: String) throws -> URL {
        let dir = current.deletingLastPathComponent()
        let ext = current.pathExtension.isEmpty ? "png" : current.pathExtension
        return try moveToFreeName(from: current, inDirectory: dir, baseName: baseName, ext: ext)
    }

    private static func moveToFreeName(from src: URL, inDirectory dir: URL, baseName: String, ext: String) throws -> URL {
        let fm = FileManager.default
        try fm.createDirectory(at: dir, withIntermediateDirectories: true)
        for i in 0 ..< 100 {
            let name = i == 0 ? "\(baseName).\(ext)" : "\(baseName)-\(i + 1).\(ext)"
            let dst = dir.appendingPathComponent(name)
            if dst.path == src.path { return src }
            if fm.fileExists(atPath: dst.path) { continue }
            try moveItemCrossVolume(from: src, to: dst)
            return dst
        }
        throw ChaosError.renameCollision(baseName)
    }

    private static func moveItemCrossVolume(from src: URL, to dst: URL) throws {
        let fm = FileManager.default
        do {
            try fm.moveItem(at: src, to: dst)
        } catch let error as NSError where error.domain == NSCocoaErrorDomain && error.code == 512 {
            try fm.copyItem(at: src, to: dst)
            try fm.removeItem(at: src)
        }
    }
}

enum ChaosError: LocalizedError {
    case renameCollision(String)
    case apiError(String)
    case httpStatus(Int)
    case configError(String)
    case fileNotStable(String)

    var errorDescription: String? {
        switch self {
        case .renameCollision(let slug): "Failed to generate non-colliding name for \"\(slug)\""
        case .apiError(let msg): "API error: \(msg)"
        case .httpStatus(let code): "API error: HTTP \(code)"
        case .configError(let msg): "Config error: \(msg)"
        case .fileNotStable(let path): "File did not stabilize: \(path)"
        }
    }
}
