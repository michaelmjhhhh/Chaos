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
        .defaultSize(width: 880, height: 620)
        .windowResizability(.contentMinSize)
        .windowToolbarStyle(.unified)
        .commands {
            CommandGroup(after: .appSettings) {
                Button("Check API Health") {
                    Task { await appState.checkAPIHealth() }
                }
                .keyboardShortcut("t", modifiers: [.command, .shift])
            }
            CommandGroup(replacing: .toolbar) {
                Button("Toggle Start/Stop") {
                    if appState.isWatching { appState.stop() } else { appState.start() }
                }
                .keyboardShortcut(.space, modifiers: [])
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
