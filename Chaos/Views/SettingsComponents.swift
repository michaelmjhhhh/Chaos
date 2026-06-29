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

/// A small, muted line of explanatory text under a control — plain-language help so a
/// non-technical user understands what a field does and why it matters.
struct SettingsHint: View {
    let text: String
    var icon: String = "info.circle"

    init(_ text: String, icon: String = "info.circle") {
        self.text = text
        self.icon = icon
    }

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: Theme.sMicro) {
            Image(systemName: icon)
                .font(.system(size: 10))
                .foregroundStyle(Theme.textSoft)
            Text(text)
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textMuted)
                .fixedSize(horizontal: false, vertical: true)
        }
    }
}

/// A compact, on-brand segmented control for picking the app appearance. Each
/// segment pairs an SF Symbol with its label; the selected one gets the coral
/// fill. Adapts to light/dark automatically via the Theme tokens.
struct AppearanceSelector: View {
    @Binding var selection: AppearancePreference
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        HStack(spacing: 0) {
            ForEach(AppearancePreference.allCases) { option in
                segment(option)
            }
        }
        .padding(3)
        .background(Theme.surfaceMuted)
        .clipShape(.rect(cornerRadius: Theme.r8))
        .overlay {
            RoundedRectangle(cornerRadius: Theme.r8)
                .stroke(Theme.border, lineWidth: 0.5)
        }
        .animation(reduceMotion ? nil : .easeInOut(duration: 0.15), value: selection)
    }

    @ViewBuilder
    private func segment(_ option: AppearancePreference) -> some View {
        let isSelected = selection == option
        Button {
            selection = option
        } label: {
            HStack(spacing: Theme.sMicro) {
                Image(systemName: option.icon)
                    .font(.system(size: 12))
                Text(option.label)
                    .font(Theme.caption)
            }
            .foregroundStyle(isSelected ? Theme.onBrand : Theme.textMuted)
            .frame(maxWidth: .infinity)
            .padding(.vertical, 6)
            .background {
                if isSelected {
                    RoundedRectangle(cornerRadius: Theme.r6)
                        .fill(Theme.coral)
                        .shadow(color: Theme.shadowCard, radius: 2, y: 1)
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(option.label)
        .accessibilityAddTraits(isSelected ? [.isSelected] : [])
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
