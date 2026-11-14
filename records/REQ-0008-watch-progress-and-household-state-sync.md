---
id: REQ-0008
title: Watch progress persistence, per-episode resume memory, auto-advance, and household state sync
status: provisional
because: [FCT-0001, ASM-0003, FCT-0011]
---

## Statement
The system MUST track playback progress across movies and series episodes, surface an actionable **"Continue Watching"** row, auto-advance episodes, and synchronize state across devices on the home network:
1. **Progress recording (`>= 5s`)**: Persist playback position and duration (up to `40` entries, stripping nested season trees to keep storage compact). For movies reaching `>= 94%` of duration, automatically remove the movie from Continue Watching as completed.
2. **Per-episode resume memory**: For series and anime, store a per-episode map keyed by `${season}:${episode}` so switching between episodes or seasons preserves each episode's exact resume timestamp and completion percentage bar (`>= 90%` marked watched).
3. **"Up Next" episode auto-advance**: During the final `30 seconds` of an episode (or upon `ended`), display an "Up Next" prompt with a `10-second` countdown that automatically launches the next episode in the season (or Season `N+1` Episode `1`).
4. **Household state sync (`/api/state`)**: Synchronize `watchProgress` (max `40`) and `favoriteChannelIds` (max `200`) with a persistent server-side state store (`lumen.db` and `state.json`), applying atomic per-item timestamped upserts (`updatedAt`) and per-episode map merges on the server and on client startup.

## Acceptance
- Given a viewer watching Season 1 Episode 2 of a series at 14:20 who switches to Episode 3 and later returns to Episode 2
- When Episode 2 is reopened or inspected in the season episode list
- Then Episode 2 resumes from 14:20 and displays its saved progress bar in the episode guide

- Given a movie played past 94% of its total duration
- When playback progress is saved
- Then the movie is automatically cleared from the "Continue Watching" row
