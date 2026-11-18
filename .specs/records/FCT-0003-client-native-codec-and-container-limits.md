---
id: FCT-0003
title: Standard playback clients decode only a subset of video and audio codecs natively
status: active
evidence: "Verified by media stream probing and playback tests: standard web playback clients natively decode H.264, VP8, VP9, and AV1 video plus AAC, MP3, Opus, Vorbis, and FLAC audio inside MP4 or WebM containers, whereas community streams frequently use MKV/AVI containers with HEVC video or AC3/EAC3/DTS surround audio."
---

## Statement
Standard client media players natively decode only `h264`, `vp8`, `vp9`, and `av1` video streams and `aac`, `mp3`, `opus`, `vorbis`, and `flac` audio streams packaged in `mp4` or `webm` containers. Peer-to-peer and community media releases frequently package video in `matroska` (`mkv`) or `avi` containers or encode audio with `ac3`, `eac3`, or `dts` and video with `hevc`, which fail client playback unless remuxed or transcoded into a fragmented stream container.
