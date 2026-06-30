import SwiftUI

struct ContentView: View {
    @Environment(AppState.self) private var appState
    @State private var selectedTab: AppTab = .dashboard
    @State private var showOnboarding = false

    enum AppTab: Hashable { case dashboard, pipeline, insights }

    var body: some View {
        TabView(selection: $selectedTab) {
            Tab("Dashboard", systemImage: "newspaper", value: AppTab.dashboard) {
                DashboardView()
            }
            Tab("Pipeline", systemImage: "square.stack.3d.up", value: AppTab.pipeline) {
                PipelineView()
            }
            Tab("Insights", systemImage: "chart.bar.xaxis", value: AppTab.insights) {
                InsightsView()
            }
        }
        .frame(minWidth: 520, minHeight: 540)
        .onAppear {
            if !appState.hasCompletedOnboarding {
                showOnboarding = true
            }
        }
        .onChange(of: appState.hasCompletedOnboarding) { _, completed in
            // Lets the Help → “Replay Welcome Guide” command re-open onboarding.
            if !completed { showOnboarding = true }
        }
        .sheet(isPresented: $showOnboarding) {
            WelcomeView { showOnboarding = false }
                .environment(appState)
        }
        .sheet(isPresented: helpBinding) {
            HelpView { appState.showHelp = false }
                .environment(appState)
        }
    }

    private var helpBinding: Binding<Bool> {
        Binding(get: { appState.showHelp }, set: { appState.showHelp = $0 })
    }
}
