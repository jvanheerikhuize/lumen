---
id: ASM-0001
title: Prioritizing the first 2% of a media file (16–64 MB) and final 4 MB tail enables fast playback startup
status: validated
confidence: 0.9
promoted_to: FCT-0004
---

## Statement
If the peer-to-peer streaming engine prioritizes downloading the leading `2%` of the selected media file (clamped between `16 MB` and `64 MB`, roughly 2–3 minutes of video) alongside the final `4 MB` tail, container headers and index tables become available quickly enough for stream probing and playback to begin without full-file download.

## How we validate it
Stream standard 720p and 1080p MP4 and MKV releases via the peer-to-peer engine and verify that `ffprobe` inspection and initial playback succeed as soon as the head and tail ranges complete.
