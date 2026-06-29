import Foundation

struct AppConfig: Codable, Equatable {
    var provider: String?
    var apiKey: String?
    var model: String?
    var baseURL: String?
    var watchDir: String?
    var outputDir: String?
    var copyToClipboard: Bool?
    var language: String?
    var filenameTemplate: String?
    var subfolderRule: String?
    var notifyOnComplete: Bool?
    var appearance: String?

    enum CodingKeys: String, CodingKey {
        case provider
        case apiKey = "api_key"
        case model
        case baseURL = "base_url"
        case watchDir = "watch_dir"
        case outputDir = "output_dir"
        case copyToClipboard = "copy_to_clipboard"
        case language
        case filenameTemplate = "filename_template"
        case subfolderRule = "subfolder_rule"
        case notifyOnComplete = "notify_on_complete"
        case appearance
    }
}
