import Foundation

/// Phase 0 keeps settings in code. Edit these two values for your server.
enum Config {
    /// Where the Bloom server container is listening (Tower on your LAN).
    static let serverURL = URL(string: "http://192.168.1.177:8484")!
    /// Matches BLOOM_TOKEN on the server. Leave empty if the server has no token.
    static let token = ""
}
