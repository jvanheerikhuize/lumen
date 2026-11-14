---
id: FCT-0005
title: Peer-to-peer stream candidates vary widely in active seeders, bitrate, and codec compatibility
status: active
evidence: "Verified by source ranking unit tests and live addon queries: unranked candidate lists frequently place 0-seed swarms, 40 GB 4K HEVC remuxes, or low-quality cam rips ahead of high-seed 1080p H.264 streams."
---

## Statement
Stream discovery providers return heterogeneous candidate lists containing dead swarms (`0` seeders), oversized uncompressed remuxes (`>15 GB`), incompatible codecs (`HEVC`, Dolby Vision), and tiny sample/cam files (`<300 MB`) alongside healthy, directly playable `1080p`/`720p` streams. Without deterministic streamability ranking and automatic failover on stall, playback frequently hangs during initial buffering.
