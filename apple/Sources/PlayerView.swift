import SwiftUI
import AVKit

/// System player (AVPlayerViewController) pointed at the server's HLS playlist.
/// Using the system player gives Siri Remote gestures, HDR and Dolby Vision
/// mode switching, Atmos output, and the standard transport UI for free.
struct PlayerView: UIViewControllerRepresentable {
    let url: URL
    let token: String

    func makeCoordinator() -> Coordinator { Coordinator() }

    func makeUIViewController(context: Context) -> AVPlayerViewController {
        var options: [String: Any] = [:]
        if !token.isEmpty {
            options["AVURLAssetHTTPHeaderFieldsKey"] = ["Authorization": "Bearer \(token)"]
        }
        let asset = AVURLAsset(url: url, options: options)
        let player = AVPlayer(playerItem: AVPlayerItem(asset: asset))

        let controller = AVPlayerViewController()
        controller.player = player
        context.coordinator.reportFirstFrame(of: player)
        player.play()
        return controller
    }

    func updateUIViewController(_ controller: AVPlayerViewController, context: Context) {}

    static func dismantleUIViewController(_ controller: AVPlayerViewController, coordinator: Coordinator) {
        controller.player?.pause()
    }

    /// Prints how long playback took to start. This is the number to watch in Phase 0.
    final class Coordinator {
        private var observation: NSKeyValueObservation?
        private var reported = false
        private let startedAt = Date()

        func reportFirstFrame(of player: AVPlayer) {
            observation = player.observe(\.timeControlStatus, options: [.new]) { [weak self] player, _ in
                guard let self, !self.reported, player.timeControlStatus == .playing else { return }
                self.reported = true
                let seconds = Date().timeIntervalSince(self.startedAt)
                print(String(format: "Bloom: playback started after %.2f s", seconds))
            }
        }
    }
}
