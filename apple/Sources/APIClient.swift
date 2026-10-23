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

    /// Where a title is in preparation: "idle", "preparing", "ready" or "unsupported".
    enum PrepareState: String { case idle, preparing, ready, unsupported, failed }

    private struct StateResponse: Decodable { let state: String }

    private func state(from data: Data, response: URLResponse) -> PrepareState {
        guard let http = response as? HTTPURLResponse else { return .failed }
        let decoded = (try? JSONDecoder().decode(StateResponse.self, from: data))?.state
        if http.statusCode == 422 { return .unsupported }
        guard (200..<300).contains(http.statusCode), let decoded, let value = PrepareState(rawValue: decoded) else {
            return .failed
        }
        return value
    }

    /// Asks the server to start preparing a title. `play: true` starts it even
    /// when the server is busy with background preparation.
    @discardableResult
    func prepare(_ item: Item, play: Bool = false) async -> PrepareState {
        var req = request("api/items/\(item.id)/prepare", method: "POST")
        if play, var parts = URLComponents(url: req.url!, resolvingAgainstBaseURL: false) {
            parts.queryItems = [URLQueryItem(name: "play", value: "1")]
            req.url = parts.url
        }
        guard let (data, response) = try? await URLSession.shared.data(for: req) else { return .failed }
        return state(from: data, response: response)
    }

    func status(_ item: Item) async -> PrepareState {
        guard let (data, response) = try? await URLSession.shared.data(for: request("api/items/\(item.id)/status")) else {
            return .failed
        }
        return state(from: data, response: response)
    }

    func playlistURL(for item: Item) -> URL {
        baseURL.appendingPathComponent("stream/\(item.id)/index.m3u8")
    }
}
