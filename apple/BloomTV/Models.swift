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
    let durationSec: Double
    let resolution: String
    let hdr: String
    let plan: Plan

    var isPlayable: Bool { plan.mode != "unsupported" }

    var durationText: String {
        let minutes = Int(durationSec / 60)
        return minutes >= 60 ? "\(minutes / 60)h \(minutes % 60)m" : "\(minutes)m"
    }
}
