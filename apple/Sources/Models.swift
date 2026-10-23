import Foundation

struct Plan: Decodable, Hashable {
    let mode: String       // "remux", "remux+audio" or "unsupported"
    let video: String
    let audio: String
    let reasons: [String]?
}

struct Item: Decodable, Identifiable, Hashable {
    let id: String
    let title: String
    let kind: String       // "movie" or "episode"
    let year: Int?
    let show: String?
    let season: Int?
    let episode: Int?
    let durationSec: Double
    let resolution: String
    let hdr: String
    let plan: Plan

    var isPlayable: Bool { plan.mode != "unsupported" }
    var isMovie: Bool { kind == "movie" }

    var durationText: String {
        let minutes = Int(durationSec / 60)
        return minutes >= 60 ? "\(minutes / 60)h \(minutes % 60)m" : "\(minutes)m"
    }

    var detailLine: String {
        var parts: [String] = []
        if let year, year > 0 { parts.append(String(year)) }
        parts.append(durationText)
        parts.append(resolution)
        if hdr != "SDR" { parts.append(hdr) }
        return parts.joined(separator: " · ")
    }
}

/// A show with its episodes, built on the device from the flat item list.
struct Show: Identifiable, Hashable {
    let name: String
    let year: Int?
    let episodes: [Item]
    var id: String { name }

    var seasons: [Int] { Array(Set(episodes.map { $0.season ?? 0 })).sorted() }
    func episodes(in season: Int) -> [Item] {
        episodes.filter { ($0.season ?? 0) == season }
            .sorted { ($0.episode ?? 0) < ($1.episode ?? 0) }
    }

    static func group(_ items: [Item]) -> [Show] {
        Dictionary(grouping: items.filter { !$0.isMovie }, by: { $0.show ?? "Unknown" })
            .map { Show(name: $0.key, year: $0.value.first?.year, episodes: $0.value) }
            .sorted { $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending }
    }
}
