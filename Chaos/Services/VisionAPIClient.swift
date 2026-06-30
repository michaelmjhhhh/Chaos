import Foundation

/// Free-trial usage for the bundled hosted provider, as reported by `GET /api/usage`.
struct HostedUsage: Decodable, Sendable, Equatable {
    let used: Int
    let limit: Int
    let remaining: Int
}

actor VisionAPIClient {
    private let session: URLSession

    init(session: URLSession = .shared) {
        self.session = session
    }

    /// Ask the hosted proxy how much of this device's free trial is left. The base URL is
    /// the hosted provider's (".../api"); usage lives next to chat at ".../api/usage".
    func fetchUsage(baseURL: String, apiKey: String) async throws -> HostedUsage {
        let trimmed = baseURL.hasSuffix("/") ? String(baseURL.dropLast()) : baseURL
        guard let url = URL(string: trimmed + "/usage"),
              let scheme = url.scheme?.lowercased(),
              ["http", "https"].contains(scheme)
        else {
            throw ChaosError.apiError("Invalid usage URL")
        }
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = 15

        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse, (200 ..< 300).contains(http.statusCode) else {
            throw ChaosError.httpStatus((response as? HTTPURLResponse)?.statusCode ?? 0)
        }
        return try JSONDecoder().decode(HostedUsage.self, from: data)
    }

    func generateSlug(
        imageBase64: String,
        mimeType: String = "image/jpeg",
        baseURL: String,
        apiKey: String,
        model: String,
        language: SlugLanguage,
        customSystemPrompt: String? = nil
    ) async throws -> String {
        let payload: [String: Any] = [
            "model": model,
            "messages": Self.chatMessages(
                imageBase64: imageBase64,
                mimeType: mimeType,
                language: language,
                customSystemPrompt: customSystemPrompt
            ),
            // A slug is a handful of words; capping output keeps inference fast.
            "max_tokens": 32,
            "stream": false
        ]

        let url = try endpointURL(baseURL: baseURL)
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)
        request.timeoutInterval = 30

        let (data, response) = try await session.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse,
              (200 ..< 300).contains(httpResponse.statusCode)
        else {
            let statusCode = (response as? HTTPURLResponse)?.statusCode ?? 0
            throw ChaosError.httpStatus(statusCode)
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
            "stream": false
        ]

        let url = try endpointURL(baseURL: baseURL)
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)
        request.timeoutInterval = 15

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse,
              (200 ..< 300).contains(httpResponse.statusCode)
        else {
            return false
        }
        return !data.isEmpty
    }

    /// The OpenAI-compatible `messages` array for a slug request: a system turn (the custom
    /// prompt when active, else the language default) and a user turn pairing a short
    /// "return only the slug" nudge with the image. Exposed so the wiring can be tested.
    static func chatMessages(
        imageBase64: String,
        mimeType: String,
        language: SlugLanguage,
        customSystemPrompt: String?
    ) -> [[String: Any]] {
        [
            ["role": "system", "content": resolveSystemPrompt(language: language, customSystemPrompt: customSystemPrompt)],
            [
                "role": "user",
                "content": [
                    ["type": "text", "text": userPrompt(language: language, customSystemPrompt: customSystemPrompt)],
                    [
                        "type": "image_url",
                        "image_url": ["url": "data:\(mimeType);base64,\(imageBase64)"]
                    ]
                ] as [[String: Any]]
            ] as [String: Any]
        ]
    }

    /// The system prompt to send for a request: the user's custom prompt when one is set and
    /// non-blank, otherwise the built-in default for the chosen language. A custom prompt fully
    /// replaces the default (including its language instruction); `SlugSanitizer` remains the
    /// safety net for whatever the model returns.
    static func resolveSystemPrompt(language: SlugLanguage, customSystemPrompt: String?) -> String {
        isCustomActive(customSystemPrompt) ? customSystemPrompt! : defaultSystemPrompt(language: language)
    }

    /// The built-in system prompt for a language. Exposed so the settings editor can prefill and
    /// reset the custom-prompt field from the same source of truth used at request time.
    static func defaultSystemPrompt(language: SlugLanguage) -> String {
        switch language {
        case .zh:
            "You are a file-naming assistant. Analyze the image and provide a concise, slug-style filename in Simplified Chinese (Chinese characters, numbers, hyphens only). No explanation, no prefix."
        case .en:
            "You are a file-naming assistant. Analyze the image and provide a concise, slug-style English filename (lowercase, numbers, hyphens only). No explanation, no prefix."
        }
    }

    /// True when a non-blank custom system prompt has been supplied.
    private static func isCustomActive(_ customSystemPrompt: String?) -> Bool {
        guard let custom = customSystemPrompt else { return false }
        return !custom.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    /// The user-turn nudge that accompanies the image. A custom prompt overrides the language, so
    /// it gets the language-neutral nudge; otherwise the default reinforces the chosen language.
    private static func userPrompt(language: SlugLanguage, customSystemPrompt: String?) -> String {
        isCustomActive(customSystemPrompt) ? "Return only the filename slug." : defaultUserPrompt(language: language)
    }

    /// The default user-turn nudge for a language, used when no custom prompt is active.
    private static func defaultUserPrompt(language: SlugLanguage) -> String {
        switch language {
        case .zh: "Return only the filename slug in Simplified Chinese."
        case .en: "Return only the filename slug."
        }
    }

    private func endpointURL(baseURL: String) throws -> URL {
        guard let url = URL(string: baseURL),
              let scheme = url.scheme?.lowercased(),
              ["http", "https"].contains(scheme),
              let host = url.host,
              !host.isEmpty
        else {
            throw ChaosError.apiError("Invalid provider base URL")
        }

        return url
            .appendingPathComponent("chat")
            .appendingPathComponent("completions")
    }

    private func parseSlugResponse(_ data: Data) throws -> String {
        guard let json = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw ChaosError.apiError("Invalid JSON response")
        }

        // OpenAI-style: choices[0].message.content
        if let choices = json["choices"] as? [[String: Any]],
           let first = choices.first,
           let message = first["message"] as? [String: Any]
        {
            if let content = message["content"] as? String {
                let trimmed = content.trimmingCharacters(in: .whitespacesAndNewlines)
                if !trimmed.isEmpty { return trimmed }
            }
            // Array-style content
            if let parts = message["content"] as? [[String: Any]] {
                for part in parts {
                    if part["type"] as? String == "text",
                       let text = part["text"] as? String
                    {
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
           let text = firstPart["text"] as? String
        {
            let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
            if !trimmed.isEmpty { return trimmed }
        }

        // Check for provider error
        if let errorObj = json["error"] as? [String: Any],
           let msg = errorObj["message"] as? String
        {
            throw ChaosError.apiError(msg)
        }
        if let errorStr = json["error"] as? String {
            throw ChaosError.apiError(errorStr)
        }

        throw ChaosError.apiError("Empty response from provider")
    }
}
