---
id: FCT-0002
title: Anime catalogs index by seasonal entry IDs while stream and subtitle providers index by canonical Kitsu or IMDb IDs
status: active
evidence: Verified in the anime identifier cross-reference module using the community Fribb/anime-lists dataset (~40,000 entries), which maps seasonal anime catalog IDs to Kitsu IDs (for episode streams) and IMDb IDs + season numbers (for external subtitles).
---

## Statement
Anime metadata catalogs assign a distinct identifier to each season or sequel entry, whereas stream addons index anime episodes by Kitsu identifier and subtitle providers index by canonical IMDb series identifier and season number. Without a cross-reference mapping table, anime catalog entries cannot resolve playable streams or external subtitles.
