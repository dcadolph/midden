import SwiftUI

/// MiddenApp is the application entry point.
@main
struct MiddenApp: App {
    /// store is the single vault handle shared by every screen.
    @StateObject private var store = VaultStore()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(store)
        }
    }
}

/// RootView is the tab shell holding capture, today, and the week.
struct RootView: View {
    /// store supplies entries and accepts new ones.
    @EnvironmentObject private var store: VaultStore
    /// scenePhase drives a sync when the app returns to the foreground.
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        TabView {
            CaptureView()
                .tabItem { Label("Capture", systemImage: "mic.circle") }
            TodayView()
                .tabItem { Label("Today", systemImage: "text.book.closed") }
            WeekView()
                .tabItem { Label("Week", systemImage: "calendar") }
            BrowseView()
                .tabItem { Label("Browse", systemImage: "magnifyingglass") }
            LifeView()
                .tabItem { Label("Record", systemImage: "chart.bar.doc.horizontal") }
        }
        .onChange(of: scenePhase) { _, phase in
            // Coming back to the app is the moment another device's entries are
            // most likely waiting, so pull them in then.
            if phase == .active {
                Task { await store.sync() }
            }
        }
        .alert("Vault error", isPresented: $store.showingError, presenting: store.errorMessage) { _ in
            Button("OK", role: .cancel) {}
        } message: { message in
            Text(message)
        }
    }
}
