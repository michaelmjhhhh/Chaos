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
        topNounCounts(from: slugs, limit: limit).map(\.token)
    }

    /// Same ranking as `topNouns`, but keeps each token's frequency — used by the Insights
    /// "Top categories" bars, which need the counts to size the bars.
    static func topNounCounts(from slugs: [String], limit: Int) -> [(token: String, count: Int)] {
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
            .map { (token: $0.key, count: $0.value) }
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
