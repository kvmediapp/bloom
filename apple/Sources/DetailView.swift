import SwiftUI

/// Opening this screen asks the server to prepare the title, which hides the
/// 10 to 15 second first-play indexing behind the time spent choosing Play.
struct DetailView: View {
    let item: Item
    let api: APIClient

    @State private var playing = false

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
                playing = true
            } label: {
                Label("Play", systemImage: "play.fill").padding(.horizontal, 24)
            }
            .disabled(!item.isPlayable)
            if !item.isPlayable {
                Text((item.plan.reasons ?? ["This title cannot be played yet."]).joined(separator: "\n"))
                    .font(.caption).foregroundStyle(.secondary)
            }
            Spacer()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(60)
        .task { if item.isPlayable { await api.prepare(item) } }
        .fullScreenCover(isPresented: $playing) {
            PlayerView(url: api.playlistURL(for: item), token: api.token)
                .ignoresSafeArea()
        }
    }
}
