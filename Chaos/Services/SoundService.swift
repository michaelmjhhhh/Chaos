import AppKit

enum SoundService {
    static func playGlass() {
        NSSound(named: "Glass")?.play()
    }
}
