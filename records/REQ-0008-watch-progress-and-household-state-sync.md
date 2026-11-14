---
id: REQ-0008
title: Per-member household profiles, watch progress persistence, episode resume memory, and state sync
status: provisional
because: [FCT-0001, ASM-0003, FCT-0009]
---

## Statement
The system MUST isolate viewer state per household member profile (`profileId`), track playback progress across movies and series episodes, surface an actionable **"Continue Watching"** row for the active member, auto-advance episodes, and synchronize member state across devices on the home network:
1. **Per-member household state isolation**: Support switching and adding household member profiles in the navigation bar without login credentials; each member profile maintains its own independent `watchProgress`, per-episode resume positions, `favoriteChannelIds`, and genre/liked `tasteProfile`.
2. **Progress recording (`>= 5s`)**: Persist playback position and duration per member profile (up to `40` entries, stripping nested season trees to keep storage compact). For movies reaching `>= 94%` of duration, automatically remove the movie from that member's Continue Watching row as completed.
3. **Per-episode resume memory**: For series and anime, store a per-episode map keyed by `${season}:${episode}` so switching between episodes or seasons preserves each episode's exact resume timestamp and completion percentage bar (`>= 90%` marked watched).
4. **"Up Next" episode auto-advance**: During the final `30 seconds` of an episode (or upon `ended`), display an "Up Next" prompt with a `10-second` countdown that automatically launches the next episode in the season (or Season `N+1` Episode `1`).
5. **Household state sync (`/api/state?profile=<id>`)**: Synchronize each member's `watchProgress` (max `40`), `favoriteChannelIds` (max `200`), and the household member roster (`profiles`) with a persistent server-side state store, applying atomic per-item timestamped upserts (`updatedAt`) and per-episode map merges on the server and on client startup.

## Acceptance
- Given two household members (`alice` and `bob`) watching the same series at different episodes
- When the viewer switches the active household profile between `alice` and `bob`
- Then each member sees only their own isolated Continue Watching progress, episode resume positions, pinned live channels, and personalized recommendations

- Given a viewer watching Season 1 Episode 2 of a series at 14:20 who switches to Episode 3 and later returns to Episode 2
- When Episode 2 is reopened or inspected in the season episode list
- Then Episode 2 resumes from 14:20 and displays its saved progress bar in the episode guide

- Given a movie played past 94% of its total duration
- When playback progress is saved
- Then the movie is automatically cleared from the active member's "Continue Watching" row
