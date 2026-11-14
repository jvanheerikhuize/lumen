---
id: REQ-0003
title: Peer-to-peer head-and-tail piece prioritization, byte-range streaming, and idle swarm eviction
status: active
because: [FCT-0004]
---

## Statement
When streaming from a peer-to-peer swarm, the system MUST:
1. Augment magnet URIs with a daily-refreshed list of live public tracker announce URLs alongside built-in fallback trackers.
2. Select the target video file (by explicit `fileIdx`, exact season/episode pattern match `SxxExx` / `1x02` / anime episode number, or largest video file fallback).
3. Immediately prioritize piece ranges covering the **head** (`2%` of file size, clamped between `16 MB` and `64 MB`) and the **tail** (final `4 MB` of the file) with high priority, deselecting non-essential pieces until initial buffering succeeds.
4. Expose real-time buffer progress (`state`, `headBytes`, `headTarget`, `tailReady`, `aheadBytes`, `peers`, `downloadSpeed`) and serve the selected file over HTTP byte-range (`206 Partial Content`) responses.
5. Automatically tear down and remove torrent engines and temporary cache files after `20 minutes` of inactivity or explicit session release.

## Acceptance
- Given a magnet link for a 2 GB media file whose container index resides in the final 4 MB
- When the stream prepare endpoint is polled prior to playback
- Then the engine prioritizes the leading 40 MB (2%) and trailing 4 MB piece ranges and reports `ready` once either the head+tail targets or 1.5x the head target are contiguous on disk

- Given an active torrent swarm that has received no read or status requests for 20 minutes
- When the periodic cleanup sweep runs
- Then the swarm engine is destroyed and its temporary cache files are removed from disk
