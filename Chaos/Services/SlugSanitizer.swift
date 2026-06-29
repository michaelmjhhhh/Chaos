import Foundation

enum SlugSanitizer {
    static func sanitize(_ raw: String) -> String {
        let lowered = raw.lowercased().trimmingCharacters(in: .whitespaces)
        var result = ""
        var lastHyphen = false

        for scalar in lowered.unicodeScalars {
            let isAlphaNum = (scalar.value >= 0x61 && scalar.value <= 0x7A) // a-z
                || (scalar.value >= 0x30 && scalar.value <= 0x39) // 0-9
            let isCJK = (scalar.value >= 0x4E00 && scalar.value <= 0x9FFF)
                || (scalar.value >= 0x3400 && scalar.value <= 0x4DBF)
                || (scalar.value >= 0x20000 && scalar.value <= 0x2A6DF)

            if isAlphaNum || isCJK {
                result.append(Character(scalar))
                lastHyphen = false
            } else if !lastHyphen {
                result.append("-")
                lastHyphen = true
            }
        }

        let trimmed = result.trimmingCharacters(in: CharacterSet(charactersIn: "-"))
        return trimmed.isEmpty ? "untitled-shot" : trimmed
    }
}
