---
id: REQ-0004
title: Pre-playback stream inspection and three-tier adaptive media pipeline
status: active
because: [FCT-0003, FCT-0004]
---

## Statement
Before starting playback, the system MUST inspect the media input to determine container format, duration, video codec, audio tracks (preferring English `eng`/`en`, then default disposition, then first track), and embedded text subtitle streams, and route playback through one of three tiers:
1. **Tier 1 — Direct Play**: When the container is `mp4` or `webm` (excluding `.mkv`/`.avi`), the video codec is in `{h264, vp8, vp9, av1}`, and the preferred audio codec is in `{aac, mp3, opus, vorbis, flac}` as the first audio track, serve the raw byte stream directly via HTTP `206 Partial Content`.
2. **Tier 2 — Lossless Video Remux**: When the video codec is already client-compatible (`{h264, vp8, vp9, av1}`) but the container (e.g., `mkv`) or audio track (`ac3`, `eac3`, `dts`, or non-first audio track) is incompatible, copy the video bitstream without re-encoding (`-c:v copy`) and transcode only audio to stereo `AAC` (`192 kbps`) inside a fragmented MP4 (`fMP4`) stream.
3. **Tier 3 — Full On-the-Fly Transcode**: When the video codec is incompatible (`hevc`/`h265`, etc.), transcode video to `H.264` (downscaling only if height `> 1080p`, with optional hardware acceleration `vaapi`, `nvenc`, `qsv`, or `videotoolbox` and automatic fallback to software `libx264 ultrafast CRF 23`) and audio to stereo `AAC` inside a fragmented MP4 (`fMP4`) stream, supporting arbitrary time-offset seeking (`start` seconds) via timeline offset adjustment.

## Acceptance
- Given an MKV media stream containing 1080p H.264 video and 5.1 EAC3 audio
- When the client probes the stream and starts playback
- Then the system reports `directPlayable: false, copyVideo: true, copyAudio: false` and streams a fragmented MP4 with copied H.264 video and transcoded stereo AAC audio

- Given a transcoded or remuxed stream where the viewer seeks to 00:45:00 (`start=2700`)
- When the client requests the transcode stream at `start=2700`
- Then the server restarts the media pipeline at `-ss 2700` and the player timeline maps seek positions relative to the 2700-second offset
