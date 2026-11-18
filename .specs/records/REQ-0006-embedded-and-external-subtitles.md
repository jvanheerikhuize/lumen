---
id: REQ-0006
title: Embedded and external subtitle extraction, WebVTT normalization, and live sync offset
status: active
because: [FCT-0002, FCT-0003]
---

## Statement
The system MUST provide both embedded container subtitles and external multi-language subtitles in WebVTT format with live synchronization controls:
1. **Embedded subtitles**: Extract text-based subtitle streams (`subrip`, `srt`, `ass`, `ssa`, `webvtt`, `mov_text`) detected during media inspection directly to WebVTT via stream index, cached in memory per input + track index.
2. **External subtitles**: Query external subtitle providers by canonical IMDb ID (plus `season` and `episode` for series/anime), filter by user-selected language codes (defaulting to English, Spanish, French, German, Dutch, Portuguese, Italian), and convert SubRip (`SRT`) responses (`max 2 MB`) to valid `WEBVTT` cues.
3. **Timeline & sync adjustment**: When remuxing/transcoding starts at a seek offset (`start > 0`) or when the user adjusts subtitle delay (`G` / `H` in `0.5s` increments), shift all parsed cue start/end timestamps accordingly without refetching from the network.

## Acceptance
- Given an MKV stream with an embedded English SubRip track and external Dutch subtitles from OpenSubtitles
- When the player opens the subtitle menu and the user presses `H` (`+0.5s`)
- Then both embedded and external tracks appear as selectable WebVTT tracks and all cue timestamps shift forward by 0.5 seconds immediately
