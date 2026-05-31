import SwiftUI

struct SettingsCard<Content: View>: View {
    let title: String
    let subtitle: String?
    let content: Content

    init(
        title: String,
        subtitle: String? = nil,
        @ViewBuilder content: () -> Content
    ) {
        self.title = title
        self.subtitle = subtitle
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            SettingsCardHeader(title: title, subtitle: subtitle)

            Divider()

            content
        }
        .padding(Theme.sMed)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surfaceCard)
        .clipShape(.rect(cornerRadius: Theme.r10))
        .overlay {
            RoundedRectangle(cornerRadius: Theme.r10)
                .stroke(Theme.border, lineWidth: 0.5)
        }
        .shadow(color: Theme.shadowCard, radius: 4, y: 2)
    }
}

struct SettingsCardHeader: View {
    let title: String
    let subtitle: String?

    init(title: String, subtitle: String? = nil) {
        self.title = title
        self.subtitle = subtitle
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            Text(title)
                .font(Theme.displayMd)
                .foregroundStyle(Theme.warmInk)

            if let subtitle {
                Text(subtitle)
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textMuted)
            }
        }
    }
}

struct SettingsBadge: View {
    let text: String
    let systemImage: String
    let tint: Color

    var body: some View {
        HStack(spacing: Theme.sMicro) {
            Image(systemName: systemImage)
                .foregroundStyle(tint)

            Text(text)
                .foregroundStyle(Theme.textBody)
        }
            .font(Theme.caption)
            .padding(.horizontal, Theme.sSmall)
            .padding(.vertical, Theme.sMicro)
            .background(tint.opacity(0.1))
            .clipShape(.capsule)
    }
}

struct SettingsConnectionResult: View {
    let status: String
    let failureHint: String

    var body: some View {
        HStack(alignment: .top, spacing: Theme.sSmall) {
            if status == "Checking..." {
                ProgressView()
                    .controlSize(.small)
            } else {
                Image(systemName: iconName)
                    .foregroundStyle(messageTint)
            }

            VStack(alignment: .leading, spacing: Theme.sMicro) {
                Text(message)
                    .font(Theme.bodySm)
                    .foregroundStyle(messageForeground)

                if status == "FAIL" {
                    Text(failureHint)
                        .font(Theme.bodySm)
                        .foregroundStyle(Theme.textMuted)
                        .textSelection(.enabled)
                }
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(accessibilityDescription)
    }

    private var message: String {
        switch status {
        case "OK": "Connection successful."
        case "FAIL": "Connection failed."
        case "Checking...": "Checking connection..."
        default: status
        }
    }

    private var messageTint: Color {
        switch status {
        case "OK": Theme.success
        case "FAIL": Theme.error
        default: Theme.textMuted
        }
    }

    private var messageForeground: Color {
        status == "Checking..." ? Theme.textMuted : Theme.textBody
    }

    private var iconName: String {
        switch status {
        case "OK": "checkmark.circle.fill"
        case "FAIL": "xmark.circle.fill"
        default: "info.circle.fill"
        }
    }

    private var accessibilityDescription: String {
        status == "FAIL" ? "\(message) \(failureHint)" : message
    }
}
