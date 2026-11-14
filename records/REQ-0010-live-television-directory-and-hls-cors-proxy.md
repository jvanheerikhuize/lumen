---
id: REQ-0010
title: Free-to-air live television directory, strict country filtering, liveness verification, and HLS CORS proxy
status: active
because: [FCT-0007, FCT-0008]
---

## Statement
The system MUST provide a searchable, filterable directory of free-to-air live television channels with strict per-country filtering, background on-air verification, and a server-side HLS stream relay:
1. **Channel aggregation & strict country filtering**: Merge curated public broadcasters and community IPTV country playlists (~1,700 channels) with canonical country names and ISO 3166-1 alpha-2 country codes (`DE`, `US`, `FR`, `JP`, `TR`, `NL`, `GB`, etc., or `International` for global feeds). Filtering by country MUST strictly return only channels attributed to the selected country without pinning unrelated curated channels from other countries on top; additional filters support category (`News`, `Sports`, `Entertainment`, `Movies`, `Music`, `Science`, `Kids`, `General`), search query, and pinned favorites.
2. **Batch liveness probing (`/api/live/status`)**: Probe up to `24` stream URLs per request (`concurrency = 8`, `6s` timeout, `15-minute` server cache TTL) by verifying that the target returns a valid `#EXTM3U` playlist or `video/*` / `application/octet-stream` response, hiding confirmed-offline channels when the **"On Air Only"** filter is active.
3. **HLS manifest & segment relay (`/api/live/proxy`)**: Proxy `.m3u8` manifests and media segments with `Access-Control-Allow-Origin: *`, rewriting all non-comment lines (variant playlists, relative segment paths, and `URI="..."` attributes inside `#EXT-X-KEY` / `#EXT-X-MAP`) to route through `/api/live/proxy?url=...`.

## Acceptance
- Given a viewer in the Live TV directory who selects a specific country (such as Netherlands or Japan)
- When the channel grid renders
- Then only channels attributed to that selected country are displayed, without unrelated curated channels from other countries appearing at the top

- Given a live television channel whose upstream `.m3u8` playlist uses relative segment paths and lacks CORS headers
- When the client plays the channel through the live proxy
- Then the proxy rewrites every segment and key URI to `/api/live/proxy?url=<resolved-absolute-url>` and streams the video without cross-origin errors
