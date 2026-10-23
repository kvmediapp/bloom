import SwiftUI

/// Phase 1 library: Movies and Shows tabs, shows grouped by season, and a detail
/// screen per title. Posters and the hero Home screen arrive with TMDB metadata.
struct LibraryView: View {
    private let api = APIClient()

    @State private var items: [Item] = []
    @State private var errorText: String?

    private var movies: [Item] { items.filter(\.isMovie) }
    private var shows: [Show] { Show.group(items) }

    var body: some View {
        TabView {
            MoviesList(movies: movies, api: api)
                .tabItem { Label("Movies", systemImage: "film") }
            ShowsList(shows: shows, api: api)
                .tabItem { Label("Shows", systemImage: "tv") }
        }
        .overlay {
            if let errorText {
                Text(errorText).multilineTextAlignment(.center).padding()
                    .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 16))
            } else if items.isEmpty {
                ProgressView()
            }
        }
        .task { await load() }
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

private struct MoviesList: View {
    let movies: [Item]
    let api: APIClient

    var body: some View {
        NavigationStack {
            List(movies) { movie in
                NavigationLink(value: movie) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(movie.title).font(.headline)
                        Text(movie.detailLine).font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
            .navigationTitle("Movies")
            .navigationDestination(for: Item.self) { DetailView(item: $0, api: api) }
        }
    }
}

private struct ShowsList: View {
    let shows: [Show]
    let api: APIClient

    var body: some View {
        NavigationStack {
            List(shows) { show in
                NavigationLink(value: show) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(show.name).font(.headline)
                        Text("\(show.seasons.count) season\(show.seasons.count == 1 ? "" : "s") · \(show.episodes.count) episodes")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
            .navigationTitle("Shows")
            .navigationDestination(for: Show.self) { ShowView(show: $0, api: api) }
            .navigationDestination(for: Item.self) { DetailView(item: $0, api: api) }
        }
    }
}

private struct ShowView: View {
    let show: Show
    let api: APIClient

    var body: some View {
        List {
            ForEach(show.seasons, id: \.self) { season in
                Section(season == 0 ? "Specials" : "Season \(season)") {
                    ForEach(show.episodes(in: season)) { ep in
                        NavigationLink(value: ep) {
                            Text("\(ep.episode ?? 0). \(ep.title)")
                        }
                    }
                }
            }
        }
        .navigationTitle(show.name)
    }
}
