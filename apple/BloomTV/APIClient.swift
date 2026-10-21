import Foundation

struct APIClient {
    var baseURL: URL = Config.serverURL
    var token: String = Config.token

    func items() async throws -> [Item] {
        var request = URLRequest(url: baseURL.appendingPathComponent("api/items"))
        if !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        let (data, response) = try await URLSession.shared.data(for: request)
        guard (response as? HTTPURLResponse)?.statusCode == 200 else {
            throw URLError(.badServerResponse)
        }
        return try JSONDecoder().decode([Item].self, from: data)
    }

    func playlistURL(for item: Item) -> URL {
        baseURL.appendingPathComponent("stream/\(item.id)/index.m3u8")
    }
}
