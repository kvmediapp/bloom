import Foundation

struct APIClient {
    var baseURL: URL = Config.serverURL
    var token: String = Config.token

    private func request(_ path: String, method: String = "GET") -> URLRequest {
        var request = URLRequest(url: baseURL.appendingPathComponent(path))
        request.httpMethod = method
        if !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return request
    }

    func items() async throws -> [Item] {
        let (data, response) = try await URLSession.shared.data(for: request("api/items"))
        guard (response as? HTTPURLResponse)?.statusCode == 200 else {
            throw URLError(.badServerResponse)
        }
        return try JSONDecoder().decode([Item].self, from: data)
    }

    /// Asks the server to start preparing a title so Play is instant. Failures are ignored.
    func prepare(_ item: Item) async {
        _ = try? await URLSession.shared.data(for: request("api/items/\(item.id)/prepare", method: "POST"))
    }

    func playlistURL(for item: Item) -> URL {
        baseURL.appendingPathComponent("stream/\(item.id)/index.m3u8")
    }
}
