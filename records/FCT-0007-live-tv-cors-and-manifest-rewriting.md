---
id: FCT-0007
title: Public live television playlists mix offline feeds with cross-origin HLS manifests that require proxy URI rewriting
status: active
evidence: "Verified across ~1,700 Free-TV/IPTV channels and curated live feeds: browsers block cross-origin HLS playlists and segments lacking CORS headers, and relative URIs or EXT-X-KEY/EXT-X-MAP URIs inside proxied M3U8 manifests break unless rewritten to absolute proxy URLs."
---

## Statement
Public free-to-air IPTV channel lists contain a mix of live and dead feeds, and many live broadcasters serve HTTP Live Streaming (`HLS` / `.m3u8`) manifests and media segments without cross-origin resource sharing (`CORS`) headers. Client players cannot play these streams unless the server verifies channel liveness and proxies both the manifest and its referenced variant playlists, encryption keys (`EXT-X-KEY`), initialization maps (`EXT-X-MAP`), and media segments with rewritten URIs.
