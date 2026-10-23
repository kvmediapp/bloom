import SwiftUI

/// Opening this screen asks the server to prepare the title, which hides the
/// 10 to 15 second first-play indexing behind the time spent choosing Play.
struct DetailView: View {
    let item: Item
    let api: APIClient

    @State private var playing = false
    @State private var preparing = false
    @State private var message: String?
    @State private var playTask: Task<Void, Never>?

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            if let show = item.show {
                Text("\(show) · S\(item.season ?? 0) E\(item.episode ?? 0)")
                    .font(.headline).foregroundStyle(.secondary)
            }
            Text(item.title).font(.largeTitle).bold()
            Text(item.detailLine).foregroundStyle(.secondary)
            Text("\(item.plan.mode) · \(item.plan.video) · \(item.plan.audio)")
                .font(.caption).foregroundStyle(.secondary)
            Button {
                playTask = Task { await startPlayback() }
            } label: {
                if preparing {
                    HStack(spacing: 12) {
                        ProgressView()
                        Text("Preparing…")
                    }.padding(.horizontal, 24)
                } else {
                    Label("Play", systemImage: "play.fill").padding(.horizontal, 24)
                }
            }
            .disabled(!item.isPlayable || preparing)
            if let message {
                Text(message).font(.caption).foregroundStyle(.secondary)
            }
            if !item.isPlayable {
                Text((item.plan.reasons ?? ["This title cannot be played yet."]).joined(separator: "\n"))
                    .font(.caption).foregroundStyle(.secondary)
            }
            Spacer()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(60)
        .task { if item.isPlayable { await api.prepare(item) } }
        .onDisappear { playTask?.cancel(); preparing = false }
        .fullScreenCover(isPresented: $playing) {
            PlayerView(url: api.playlistURL(for: item), token: api.token)
                .ignoresSafeArea()
        }
    }

    /// Starts the title on the server and waits until it says playback can begin.
    /// The player is opened only then, because it gives up after about 20 seconds.
    private func startPlayback() async {
        preparing = true
        message = nil
        defer { preparing = false }
        var state = await api.prepare(item, play: true)
        let deadline = Date().addingTimeInterval(15 * 60)
        while state != .ready {
            if state == .unsupported {
                message = "This title needs a transcode, which Bloom does not do yet."
                return
            }
            if Task.isCancelled || Date() > deadline { return }
            try? await Task.sleep(nanoseconds: 1_000_000_000)
            state = await api.status(item)
            if state == .idle { state = await api.prepare(item, play: true) }
            if state == .failed {
                message = "Could not reach the server. Trying again…"
                state = .preparing
            }
        }
        playing = true
    }
}
