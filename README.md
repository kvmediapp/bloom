# Bloom

A self-hosted media server and Apple-native apps, built for a polished home
experience first. Phase 0 proves the core idea: one file travels from the
server to an Apple TV through our own code, with no re-encoding.

## Status

Phase 0 (spike). The server scans a library, decides how each file can be
played, and serves it as HLS. A minimal tvOS app lists titles and plays them in
the system player.

| Piece | State |
| --- | --- |
| `server/` Go server: scan, playback plan, remux to HLS | Built and tested (`make test`) |
| `docker-compose.yml`, `server/Dockerfile` | Written; not yet run on Unraid |
| `apple/BloomTV/` tvOS app sources | Written; **not yet built in Xcode** |
| `scripts/remux-test.sh` | Written; not yet run on real files |

## Layout

```
server/            Go server (bloomd)
  cmd/bloomd/        entry point
  internal/media/    ffprobe scanning and the playback decision
  internal/hls/      ffmpeg remux to fragmented-MP4 HLS
  internal/api/      HTTP routes
apple/BloomTV/     SwiftUI sources for the tvOS app
scripts/           helper scripts
docs/              plan and notes
```

## Run the server on Unraid

1. Edit the two host paths in `docker-compose.yml` (your media share, and an
   appdata folder for the cache).
2. Optionally set `BLOOM_TOKEN` to a long random string.
3. `docker compose up -d --build`
4. Check it: `curl http://TOWER_IP:8484/health`, then
   `curl http://TOWER_IP:8484/api/items`. The first scan runs in the
   background; large libraries take a while.

Media is mounted read-only. Plex is unaffected.

### API

| Route | Purpose |
| --- | --- |
| `GET /health` | Liveness and item count (no token needed) |
| `GET /api/items` | The library with each file's playback plan |
| `POST /api/scan` | Rescan the media folder |
| `GET /stream/{id}/index.m3u8` | Start playback (starts the remux on first request) |

If `BLOOM_TOKEN` is set, send `Authorization: Bearer <token>` on everything
except `/health`.

## Build the tvOS app

See [apple/README.md](apple/README.md).

## How playback works

For each file the server picks the cheapest path:

- **remux**: video and audio are copied into HLS segments unchanged (H.264 or
  HEVC video with AAC, AC-3 or E-AC-3 audio).
- **remux+audio**: video is copied; other audio (DTS, TrueHD, FLAC and so on)
  is converted to E-AC-3 or AAC. Atmos and lossless detail are lost.
- **unsupported**: needs a video transcode (AV1, VC-1, 10-bit H.264). Returns
  422 for now. Quick Sync transcoding is planned for Phase 2.

The playlist is an EVENT playlist: playback starts after the first two
segments exist, and the playlist is finalized when ffmpeg finishes. That keeps
start-up fast, but seeking far ahead before the remux finishes is limited.
Phase 1 replaces this with a full VOD playlist built from a keyframe index and
segments created on demand.

## Known limits (Phase 0)

- No subtitles (dropped), one audio track, no chapters.
- Dolby Vision profile 7 files are flagged in the plan but not yet converted
  to profile 8.1, so they may not play correctly.
- No metadata or artwork; titles come from file names.
- Watch progress is not saved.
- Access is a single optional shared token. Do not expose the server to the
  internet.

## Phase 0 checklist

- [ ] Container runs on Tower and `/api/items` lists your library
- [ ] `scripts/remux-test.sh` works on a typical HEVC MKV
- [ ] tvOS app builds in Xcode and installs on one Apple TV
- [ ] A 4K HDR file plays from Tower; note the "playback started after N s" line in Xcode's console
- [ ] Output passes Apple's `mediastreamvalidator`
