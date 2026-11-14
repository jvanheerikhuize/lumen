---
id: FCT-0011
title: Atomic per-item timestamped state upserts prevent concurrent multi-device watch progress overwrites
status: active
promoted_from: ASM-0004
evidence: "Verified in internal/state/store.go and internal/state/store_test.go (TestConcurrentHouseholdStateSync): per-item transactional upserts guarded by updated_at comparisons and per-episode map merging in modernc.org/sqlite (WAL mode) preserve concurrent progress updates from multiple LAN devices while capping storage at 40 watch progress items and 200 favorite channels."
---

## Statement
Whole-file JSON replacement of `state.json` is vulnerable to lost updates when two household devices (for example, a living-room television and a tablet) push watch progress concurrently. Persisting `watchProgress` (up to `40` items) and `favoriteChannelIds` (up to `200` items) via transactional per-item upserts—where each media item is updated only when the incoming `updatedAt` timestamp is newer (`>=`) than the stored record and per-episode progress maps (`${season}:${episode}`) are merged by episode `updatedAt`—eliminates cross-title clobbering across LAN devices while maintaining a single shared household state without login prompts.
