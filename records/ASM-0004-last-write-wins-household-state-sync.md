---
id: ASM-0004
title: Timestamped per-item state synchronization is sufficient for household multi-device usage
status: validated
confidence: 0.95
promoted_to: FCT-0011
---

## Statement
Persisting up to `40` watch progress items and `200` favorite channel IDs on the server via per-item timestamped upserts (`updatedAt` comparison and per-episode progress map merging) is sufficient for a single household switching between a living-room television, laptop, and tablet without overwriting concurrent progress across different titles.

## How we validate it
Validated via concurrent multi-device synchronization tests (`TestConcurrentHouseholdStateSync` in `internal/state/store_test.go`), confirming that simultaneous updates from multiple household devices preserve newer per-title and per-episode timestamps without cross-title clobbering.
