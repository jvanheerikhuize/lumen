---
id: ASM-0004
title: Per-member isolated profiles with atomic timestamped item synchronization prevent household state collisions
status: validated
confidence: 0.95
promoted_to: FCT-0009
---

## Statement
Partitioning household state by member profile (`profileId`) and persisting up to `40` watch progress items and `200` favorite channel IDs per member via atomic per-item timestamped upserts (`updatedAt` comparison and per-episode progress map merging) allows different members in the household to maintain independent state across living-room televisions, laptops, and tablets without overwriting each other's progress or clobbering concurrent updates.

## How we validate it
Validated via automated multi-member profile isolation and concurrent multi-device synchronization tests confirming that different household members (`alice`, `bob`, `kids`, `default`) preserve completely independent watch histories, episode positions, favorite channels, and taste profiles, and that simultaneous device updates within a profile merge atomically by item timestamp.
