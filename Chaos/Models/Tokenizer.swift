import Foundation

enum Tokenizer {
    private static let stopwords: Set<String> = [
        "the", "and", "for", "with", "from", "this", "that", "into",
        "onto", "your", "you", "are", "but", "not", "all", "any",
        "was", "were", "has", "have", "had", "out", "its", "his",
        "her", "their", "them", "they", "she", "him", "who", "how",
        "why", "what", "when", "where", "which", "ourselves", "png"
    ]

    static func topNouns(from slugs: [String], limit: Int) -> [String] {
        guard limit > 0 else { return [] }

        var counts: [String: Int] = [:]
        for slug in slugs {
            for token in tokens(in: slug) {
                counts[token, default: 0] += 1
            }
        }

        return counts
            .sorted { lhs, rhs in
                if lhs.value != rhs.value {
                    return lhs.value > rhs.value
                }
                return lhs.key < rhs.key
            }
            .prefix(limit)
            .map(\.key)
    }

    private static func tokens(in slug: String) -> [String] {
        slug
            .lowercased()
            .components(separatedBy: CharacterSet.alphanumerics.inverted)
            .filter { token in
                guard token.count >= 3 else { return false }
                guard !stopwords.contains(token) else { return false }
                guard token.contains(where: \.isLetter) else { return false }
                return true
            }
    }
}
