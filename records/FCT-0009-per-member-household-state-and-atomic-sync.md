---
id: FCT-0009
title: Per-member household profile partitioning with atomic timestamped upserts isolates viewer state across devices
status: active
promoted_from: ASM-0004
evidence: "Verified by multi-member profile isolation tests and concurrent multi-device state synchronization tests confirming zero cross-member leakage and conflict-free per-item timestamp merging."
---

## Statement
Different members in a household watch shared series at different paces, prefer different live television channels, and have distinct genre affinities. Partitioning household state by member profile (`profileId`)—and synchronizing each member's watch progress (`(profile_id, media_id)`), per-episode resume maps (`season:episode`), and favorite live channels (`(profile_id, channel_id)`) via atomic per-item timestamped (`updatedAt`) upserts—ensures that each household member has their own isolated state while still allowing seamless multi-device resumption across TVs, laptops, and tablets.
