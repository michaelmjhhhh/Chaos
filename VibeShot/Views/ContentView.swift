import SwiftUI

struct ContentView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        TabView {
            Tab("Dashboard", systemImage: "gauge.with.dots.needle.bottom.50percent") {
                DashboardView()
            }
            Tab("History", systemImage: "clock") {
                HistoryView()
            }
        }
        .frame(minWidth: 640, minHeight: 440)
    }
}
