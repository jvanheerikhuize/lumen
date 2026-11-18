---
id: FCT-0013
title: Weighted random ranking over a 12-item hero exclusion window prevents repeats across 30 launches and adapts immediately to pinned genres
status: active
evidence: "Measured across 6,000 simulated launches (200 x 30-launch sequences) on Lumen's live 405-item catalog (393 hero-eligible candidates) and verified unit suites: 0/6,000 consecutive hero repeats, 12/12 distinct heroes per 12-launch window, 28.8-29.0 average distinct heroes per 30 launches, non-empty cold-start carousels, and 100% hero and top-10 Picked for You genre alignment after pinning and interacting with Sci-Fi."
---

## Statement
Combining cumulative genre-affinity weights (`+1` view, `+3` play, `+5`/`-4` title like/unlike, `+8`/`-6` genre pin/unpin), bounded random jitter (`0..8`), top-`8` hero candidate sampling, and a `12`-item `recentHeroIds` exclusion window mathematically guarantees zero back-to-back hero repeats, avoids cold-start empty rows, and shifts top recommendations immediately toward the member's pinned or interacted genres. Subjective human playback satisfaction without searching across `30` real household launches remains tracked separately in `ASM-0002`.
