import SwiftUI

/// Phase 0 library: a plain list so we can test playback. The real Home screen
/// (hero art, Continue Watching, shelves) comes in Phase 1.
struct LibraryView: View {
    private let api = APIClient()

    @State private var items: [Item] = []
    @State private var errorText: String?
    @State private var playing: Item?

    var body: some View {
        NavigationStack {
            List(items) { item in
                Button {
                    playing = item
                } label: {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(item.title).font(.headline)
                        Text("\(item.resolution) · \(item.hdr) · \(item.durationText) · \(item.plan.mode)")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .disabled(!item.isPlayable)
            }
            .navigationTitle("Bloom")
            .overlay {
                if let errorText {
                    Text(errorText).multilineTextAlignment(.center).padding()
                } else if items.isEmpty {
                    ProgressView()
                }
            }
        }
        .task { await load() }
        .fullScreenCover(item: $playing) { item in
            PlayerView(url: api.playlistURL(for: item), token: api.token)
                .ignoresSafeArea()
        }
    }

    private func load() async {
        do {
            items = try await api.items()
            errorText = items.isEmpty ? "The server has no titles yet. Wait for the first scan to finish." : nil
        } catch {
            errorText = "Could not reach the Bloom server at \(api.baseURL.absoluteString).\n\(error.localizedDescription)"
        }
    }
}
