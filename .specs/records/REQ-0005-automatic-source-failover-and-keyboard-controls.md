---
id: REQ-0005
title: Automatic source failover, multi-layer timeline buffer visualization, and keyboard playback controls
status: active
because: [FCT-0005, FCT-0011]
---

## Statement
The player MUST automatically fail over to the next highest-ranked stream candidate when a source fails to initialize, returns a fatal playback error, or stalls for `25 seconds` before the first frame is rendered, MUST render a multi-layer timeline scrubber with seek-readiness feedback, and MUST support full keyboard navigation:
- **Multi-layer timeline scrubber**: Render four distinct visual layers on the playback timeline—unloaded track, subtle sky-tinted verified on-disk swarm piece ranges (`downloadedRanges`, transitioning to an emerald tint with a `100% Downloaded` badge when `complete` is true), brighter white in-memory decoded browser buffer ranges (`HTMLMediaElement.buffered`), and rose-to-orange played progress with an interactive playhead thumb.
- **Hover seek-readiness preview & legend**: Hovering the timeline displays the target timestamp alongside its seek readiness (`Instant seek · Buffered in memory`, `Fast seek · Downloaded on disk`, or `Will buffer from peers`), and clicking the stream status pill opens a legend explaining each timeline layer and live swarm telemetry.
- **Keyboard shortcuts**:
  - `/` focuses the global search input when the player is closed.
  - `Space` or `K` toggles play/pause; `Left` / `Right` seeks `-10s` / `+10s`.
  - `Up` / `Down` switches to the previous/next channel during Live TV playback.
  - `F` toggles fullscreen, `P` toggles Picture-in-Picture, `M` toggles mute, `C` toggles subtitles, `G` / `H` shifts subtitle timing by `-0.5s` / `+0.5s`, and `Esc` closes open menus or exits the player.

## Acceptance
- Given a movie with 3 ranked stream sources where Source #1 stalls with 0 bytes or fails during initial buffering
- When 25 seconds elapse without playback readiness or an error occurs
- Then the player automatically releases Source #1 and switches to Source #2 without user intervention

- Given an active peer-to-peer playback session with partially or fully downloaded piece ranges
- When the viewer inspects or hovers the playback timeline scrubber
- Then the timeline renders separate layers for played progress, in-memory decoded buffer, and on-disk downloaded piece ranges, and displays the hovered timestamp's seek-readiness status
