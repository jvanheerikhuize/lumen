---
id: ASM-0002
title: Heuristic genre affinity weights and a 12-item hero history window keep discovery fresh and personal
status: open
confidence: 0.75
---

## Statement
While weighted shuffling (`score + random * 8`) and the `12`-item `recentHeroIds` history mechanically prevent consecutive repeats and adapt recommendations to pinned genres (`FCT-0013`), real household viewers across `30` application launches still prefer starting playback from the Hero banner or `"Picked for You"` carousel rather than repeatedly re-rolling or falling back to manual search.

## How we validate it
Algorithmic non-repetition and genre convergence are verified in `FCT-0013`; subjective household engagement remains `open` until privacy-local `discoveryMetrics` counters (`launches`, `heroOpens`, `pickedForYouOpens`, `searchOpens`, `rerolls`) accumulate across `30` real household launches and confirm that discovery-driven playback exceeds manual re-rolls and search fallbacks.
