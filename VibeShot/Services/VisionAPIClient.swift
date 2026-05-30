import Foundation

actor VisionAPIClient {
    private let session: URLSession

    init(session: URLSession = .shared) {
        self.session = session
    }

    func generateSlug(
        imageBase64: String,
        baseURL: String,
        apiKey: String,
        model: String,
        language: SlugLanguage
    ) async throws -> String {
        let (systemPrompt, userPrompt) = slugPrompts(language: language)

        let payload: [String: Any] = [
            "model": model,
            "messages": [
                ["role": "system", "content": systemPrompt],
                [
                    "role": "user",
                    "content": [
                        ["type": "text", "text": userPrompt],
                        [
                            "type": "image_url",
                            "image_url": ["url": "data:image/png;base64,\(imageBase64)"],
                        ],
                    ] as [[String: Any]],
                ] as [String: Any],
            ] as [[String: Any]],
            "stream": false,
        ]

        let url = URL(string: "\(baseURL.trimmingSuffix("/"))/chat/completions")!
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)

        let (data, response) = try await session.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse,
              (200..<300).contains(httpResponse.statusCode) else {
            let statusCode = (response as? HTTPURLResponse)?.statusCode ?? 0
            throw VibeShotError.apiError("HTTP \(statusCode)")
        }

        return try parseSlugResponse(data)
    }

    func checkHealth(
        baseURL: String,
        apiKey: String,
        model: String
    ) async throws -> Bool {
        let payload: [String: Any] = [
            "model": model,
            "messages": [["role": "user", "content": "Reply with only: ok"]],
            "stream": false,
        ]

        let url = URL(string: "\(baseURL.trimmingSuffix("/"))/chat/completions")!
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)
        request.timeoutInterval = 15

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse,
              (200..<300).contains(httpResponse.statusCode) else {
            return false
        }
        return !data.isEmpty
    }

    private func slugPrompts(language: SlugLanguage) -> (system: String, user: String) {
        switch language {
        case .zh:
            return (
                "You are a file-naming assistant. Analyze the image and provide a concise, slug-style filename in Simplified Chinese (Chinese characters, numbers, hyphens only). No explanation, no prefix.",
                "Return only the filename slug in Simplified Chinese."
            )
        case .en:
            return (
                "You are a file-naming assistant. Analyze the image and provide a concise, slug-style English filename (lowercase, numbers, hyphens only). No explanation, no prefix.",
                "Return only the filename slug."
            )
        }
    }

    private func parseSlugResponse(_ data: Data) throws -> String {
        guard let json = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw VibeShotError.apiError("Invalid JSON response")
        }

        // OpenAI-style: choices[0].message.content
        if let choices = json["choices"] as? [[String: Any]],
           let first = choices.first,
           let message = first["message"] as? [String: Any] {
            if let content = message["content"] as? String {
                let trimmed = content.trimmingCharacters(in: .whitespacesAndNewlines)
                if !trimmed.isEmpty { return trimmed }
            }
            // Array-style content
            if let parts = message["content"] as? [[String: Any]] {
                for part in parts {
                    if part["type"] as? String == "text",
                       let text = part["text"] as? String {
                        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
                        if !trimmed.isEmpty { return trimmed }
                    }
                }
            }
        }

        // Gemini-style: candidates[0].content.parts[0].text
        if let candidates = json["candidates"] as? [[String: Any]],
           let first = candidates.first,
           let content = first["content"] as? [String: Any],
           let parts = content["parts"] as? [[String: Any]],
           let firstPart = parts.first,
           let text = firstPart["text"] as? String {
            let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
            if !trimmed.isEmpty { return trimmed }
        }

        // Check for provider error
        if let errorObj = json["error"] as? [String: Any],
           let msg = errorObj["message"] as? String {
            throw VibeShotError.apiError(msg)
        }
        if let errorStr = json["error"] as? String {
            throw VibeShotError.apiError(errorStr)
        }

        throw VibeShotError.apiError("Empty response from provider")
    }
}

private extension String {
    func trimmingSuffix(_ suffix: String) -> String {
        if hasSuffix(suffix) {
            return String(dropLast(suffix.count))
        }
        return self
    }
}
