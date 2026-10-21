# Bloom build plan

Goal: a self-hosted media server and Apple-native apps with a gold-standard look
and feel, for the household first. Remote access for family comes last.

Guiding rule: do the least work per file. Direct play if the Apple player can
open it, remux for most MKV files, transcode only when nothing else works.

## Setup

- Server: Unraid box "Tower" (i5-13600K, Quick Sync), Docker.
- Development: MacBook Air with Xcode; test on the household's 4K Apple TVs.
- Cost: free Apple ID for Phase 0 and most of Phase 1, then the $99 per year
  Apple Developer Program for TestFlight and CloudKit.

## Phases (rough, one developer, part-time)

| Phase | Scope | Estimate |
| --- | --- | --- |
| 0. Spike | One file from Tower to an Apple TV through our code | 1 to 2 weeks |
| 1. MVP | Library, TMDB metadata, tvOS Home and detail, resume, daily use | 2 to 3 months |
| 2. Quality | Dolby Vision 7 to 8.1, display matching, subtitles (WebVTT, ASS, PGS via OCR), audio menu, chapters, fast start, Quick Sync fallback | 2 to 3 months |
| 3. Remote | iPhone app, Sign in with Apple, invites, direct connections (iroh) with optional relay, CloudKit sync | 2 to 3 months |

## Phase 0 exit criteria

One 4K HDR file plays over the LAN from Tower on an Apple TV, with a measured
time to first frame.

## Open decisions

- Server stays in Go unless there is a reason to change.
- Whether to prototype later UI against Jellyfin's API first.
- Metadata sources and licensing if Bloom is ever shared beyond the household.
