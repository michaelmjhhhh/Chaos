import Foundation

final class DirectoryWatcher: @unchecked Sendable {
    private let watchURL: URL
    private let queue = DispatchQueue(label: "com.vibeshot.watcher", qos: .utility)
    private var source: DispatchSourceFileSystemObject?
    private var fileDescriptor: Int32 = -1
    private var seenFiles = Set<String>()
    private var onNewFile: ((URL) -> Void)?

    init(directory: URL) {
        self.watchURL = directory
    }

    @discardableResult
    func start(onNewFile: @escaping (URL) -> Void) -> Bool {
        self.onNewFile = onNewFile
        fileDescriptor = open(watchURL.path, O_EVTONLY)
        guard fileDescriptor >= 0 else {
            self.onNewFile = nil
            return false
        }

        let source = DispatchSource.makeFileSystemObjectSource(
            fileDescriptor: fileDescriptor,
            eventMask: .write,
            queue: queue
        )

        source.setEventHandler { [weak self] in
            self?.scanForNewFiles()
        }

        source.setCancelHandler { [weak self] in
            guard let self, self.fileDescriptor >= 0 else { return }
            close(self.fileDescriptor)
            self.fileDescriptor = -1
        }

        self.source = source
        source.resume()

        // Initial scan
        queue.async { [weak self] in
            self?.scanForNewFiles()
        }
        return true
    }

    func stop() {
        source?.cancel()
        source = nil
    }

    private func scanForNewFiles() {
        let fm = FileManager.default
        guard let contents = try? fm.contentsOfDirectory(
            at: watchURL,
            includingPropertiesForKeys: [.contentModificationDateKey],
            options: [.skipsHiddenFiles]
        ) else { return }

        for url in contents {
            let name = url.lastPathComponent
            guard !seenFiles.contains(name) else { continue }
            guard ScreenshotGuard.isScreenshotCandidate(name) else { continue }
            guard url.pathExtension.lowercased() == "png" else { continue }

            seenFiles.insert(name)
            onNewFile?(url)
        }
    }

    deinit {
        stop()
    }
}
