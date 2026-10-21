#!/usr/bin/env bash
# Phase 0 check: remux one file to HLS the same way the Bloom server does and
# report how long it took. Run it on Tower (or anywhere ffmpeg is installed).
#
#   scripts/remux-test.sh "/mnt/user/data/media/movies/Some Film (2024).mkv"
#
# Then copy the output folder to a Mac and validate it with Apple's tool:
#   mediastreamvalidator /path/to/out/index.m3u8
set -euo pipefail

src="${1:?usage: remux-test.sh /path/to/file.mkv [output-dir]}"
out="${2:-/tmp/bloom-remux-test}"
rm -rf "$out"
mkdir -p "$out"

probe() { ffprobe -v error -select_streams "$1" -show_entries stream=codec_name -of default=nw=1:nk=1 "$src" | head -n1; }
vcodec="$(probe v:0)"
acodec="$(probe a:0)"
echo "video: ${vcodec:-none}   audio: ${acodec:-none}"

vargs=(-c:v copy)
[[ "$vcodec" == "hevc" ]] && vargs+=(-tag:v hvc1)
[[ "$vcodec" != "hevc" && "$vcodec" != "h264" ]] && { echo "video codec '$vcodec' needs a transcode; not supported yet"; exit 2; }

amap=()
aargs=()
if [[ -n "$acodec" ]]; then
  amap=(-map 0:a:0)
  case "$acodec" in
    aac|ac3|eac3) aargs=(-c:a copy) ;;
    *) aargs=(-c:a eac3 -b:a 640k); echo "audio '$acodec' will be converted to eac3" ;;
  esac
fi

start=$(date +%s)
ffmpeg -hide_banner -loglevel warning -nostdin -y -i "$src" \
  -map 0:v:0 "${amap[@]}" -sn -dn "${vargs[@]}" "${aargs[@]}" \
  -f hls -hls_time 6 -hls_playlist_type vod -hls_segment_type fmp4 \
  -hls_flags independent_segments -hls_fmp4_init_filename init.mp4 \
  -hls_segment_filename "$out/seg_%05d.m4s" "$out/index.m3u8"
echo "remuxed in $(( $(date +%s) - start )) s -> $out"
ls -1 "$out" | head -n 5
