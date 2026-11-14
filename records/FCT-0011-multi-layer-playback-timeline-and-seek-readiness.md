---
id: FCT-0011
title: Multi-layer timeline buffer visualization and hover seek-readiness feedback eliminate peer-to-peer seek ambiguity
status: active
evidence: "Verified by normalized piece-range coalescing tests and interactive playback timeline scrubber verification across direct-play, remux, and peer-to-peer streams."
---

## Statement
During peer-to-peer and adaptive streaming playback, a conventional two-state progress bar (played vs. unplayed) conflates three distinct buffer states:
1. **In-memory decoded browser buffer** (`HTMLMediaElement.buffered`), where seeking is instantaneous (`0 ms`).
2. **On-disk verified swarm piece ranges** (`downloadedRanges`), where bytes are already stored locally on disk and seeking requires only fast local container demuxing without waiting on remote peers.
3. **Unloaded swarm regions**, where seeking must request missing pieces from peers before playback can resume.

Rendering a four-layer timeline scrubber (unloaded track, subtle sky-tinted on-disk downloaded piece segments, brighter white in-memory decoded buffer segments, and rose-to-orange played progress with a glowing playhead), surfacing a `100% Downloaded` emerald completion state when all file bytes are verified on disk, displaying a hover timestamp tooltip with real-time seek readiness (`Instant seek · Buffered in memory`, `Fast seek · Downloaded on disk`, or `Will buffer from peers`), and providing an interactive stream status legend eliminates user confusion about timeline colors and seek latency.
