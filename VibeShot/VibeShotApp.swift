import SwiftUI

@main
struct VibeShotApp: App {
    @State private var appState = AppState()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environment(appState)
                .onAppear {
                    appState.loadConfig()
                    if appState.autoStart {
                        appState.start()
                    }
                }
        }
        .defaultSize(width: 700, height: 500)
        .windowResizability(.contentMinSize)
        .windowToolbarStyle(.unified)
        .commands {
            CommandGroup(after: .appSettings) {
                Button("Check API Health") {
                    Task { await appState.checkAPIHealth() }
                }
                .keyboardShortcut("t", modifiers: [.command, .shift])
            }
        }

        MenuBarExtra {
            MenuBarView()
                .environment(appState)
        } label: {
            MenuBarIcon(status: appState.watcherStatus)
        }
        .menuBarExtraStyle(.window)

        Settings {
            SettingsView()
                .environment(appState)
        }
    }
}
