---
id: REQ-0005
title: Automatic source failover on stall and keyboard-driven playback controls
status: active
because: [FCT-0005]
---

## Statement
The player MUST automatically fail over to the next highest-ranked stream candidate when a source fails to initialize, returns a fatal playback error, or stalls for `25 seconds` before the first frame is rendered, and MUST support full keyboard navigation:
- `/` focuses the global search input when the player is closed.
- `Space` or `K` toggles play/pause; `Left` / `Right` seeks `-10s` / `+10s`.
- `Up` / `Down` switches to the previous/next channel during Live TV playback.
- `F` toggles fullscreen, `P` toggles Picture-in-Picture, `M` toggles mute, `C` toggles subtitles, `G` / `H` shifts subtitle timing by `-0.5s` / `+0.5s`, and `Esc` closes open menus or exits the player.

## Acceptance
- Given a movie with 3 ranked stream sources where Source #1 stalls with 0 bytes or fails during initial buffering
- When 25 seconds elapse without playback readiness or an error occurs
- Then the player automatically releases Source #1 and switches to Source #2 without user intervention
