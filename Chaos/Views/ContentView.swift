import SwiftUI

struct ContentView: View {
    @Environment(AppState.self) private var appState
    @State private var selectedTab: AppTab = .dashboard

    enum AppTab: Hashable { case dashboard, pipeline }

    var body: some View {
        TabView(selection: $selectedTab) {
            Tab("Dashboard", systemImage: "newspaper", value: AppTab.dashboard) {
                DashboardView()
            }
            Tab("Pipeline", systemImage: "square.stack.3d.up", value: AppTab.pipeline) {
                PipelineView()
            }
        }
        .frame(minWidth: 760, minHeight: 540)
    }
}
