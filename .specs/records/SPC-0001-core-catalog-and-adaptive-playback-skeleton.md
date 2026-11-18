---
id: SPC-0001
title: Core catalog discovery, stream resolution, and adaptive playback skeleton
status: active
tier: skeleton
requirements: [REQ-0001, REQ-0002, REQ-0003, REQ-0004, REQ-0005]
---

## Purpose
The walking skeleton of Lumen: zero-configuration browsing and search across Movies, Series, and Anime catalogs, multi-provider stream discovery with streamability ranking, peer-to-peer head/tail piece prioritization, three-tier adaptive media playback (direct byte-range play, lossless video remux, and full on-the-fly transcoding with seek support), and automatic source failover on stall.

## Non-goals
- Do not require user accounts, cloud authentication, or external media library servers to browse or play content.
- Do not persist downloaded media files permanently outside the bounded ephemeral playback cache.
