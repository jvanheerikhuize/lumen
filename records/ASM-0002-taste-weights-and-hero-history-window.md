---
id: ASM-0002
title: Heuristic genre affinity weights and a 12-item hero history window keep discovery fresh and personal
status: open
confidence: 0.75
---

## Statement
Boosting genre affinity scores by `+1` on detail view, `+3` on playback start, `+5` on explicit title like (`-4` on unlike), and `+8` on pinned favorite genre (`-6` on unpin), combined with weighted random shuffling (`score + random * 8`) and excluding the last `12` shown hero IDs, produces relevant recommendations without cold-start friction.

## How we validate it
Observe household sessions over 30 application launches and measure whether users start playback from the hero banner or the "Picked for You" carousel without manually re-rolling or searching every time.
