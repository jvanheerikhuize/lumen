---
id: REQ-0002
title: Multi-provider stream discovery and deterministic streamability ranking
status: active
because: [FCT-0003, FCT-0005]
---

## Statement
The system MUST query configured stream addons and indexers in parallel, normalize magnet URIs, info-hashes, file indices, quality labels, seed counts, and file sizes, and rank candidate streams by a deterministic **streamability score** before selecting a default source:
- **Direct HTTP / debrid streams**: `+1000` over peer-to-peer torrents.
- **Resolution preference**: `1080p` (`+300`), `720p` (`+260`), `480p` (`+120`), `Auto` (`+100`), `4K` (`+40`).
- **Codec & format adjustments**: `H.264`/`AVC` (`+40`), `MP4`/`WEB-DL`/`WEBRip` (`+30`), `HEVC`/`x265` (`-250`), `HDR`/`Dolby Vision` (`-150`), `Remux`/`BDRemux` (`-200`), `CAM`/`TS` (`-300`).
- **Swarm health (torrents)**: `0` seeds receives `-400`; positive seed count adds `min(250, log2(seeds + 1) * 45)`.
- **File size bounds**: `>15 GB` (`-250`), `>6 GB` (`-100`), `<0.3 GB` (`-80`).

## Acceptance
- Given candidate streams for a title that include a 4K HEVC remux with 5 seeds, a 0-seed 1080p torrent, and a 1080p H.264 WEB-DL torrent with 80 seeds
- When the system ranks the available sources
- Then the 1080p H.264 WEB-DL source with 80 seeds is ranked first and selected by default
