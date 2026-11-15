---
id: REQ-0007
title: Non-repeating weighted-random hero selection and genre taste personalization
status: provisional
because: [FCT-0006, FCT-0013, ASM-0002]
---

## Statement
The system SHOULD learn the user's genre preferences from implicit and explicit interactions, record privacy-local launch and discovery-origin counters (`launches`, `heroOpens`, `pickedForYouOpens`, `otherOpens`, `searchOpens`, `rerolls`) on the active taste profile, and randomize home screen discovery on every launch so the same featured title is never repeated back-to-back:
1. **Taste profile scoring**: Track cumulative genre affinity (`+1` on detail modal open, `+3` on playback start, `+5` on title heart/like, `-4` on unlike, `+8` on pinned favorite genre chip, `-6` on unpin) plus up to `60` liked items and the last `12` shown hero IDs (`recentHeroIds`).
2. **Per-item personalization score**: `score = (voteAverage || 6.5) * 0.6 + sum(favoriteGenre ? 10 : 0) + sum(min(15, genreScore * 1.5)) + (isLiked ? 6 : 0)`.
3. **Weighted-random hero & carousels**: Exclude `recentHeroIds` and items without backdrop imagery, rank candidates by `score + random * 8`, pick from the top `8` candidates at random, and record the chosen hero ID in `recentHeroIds`. Provide a manual **"Surprise Me"** re-roll button, interactive genre filter chips, a **"Picked for You"** row, and a **"Your Liked Titles"** row.

## Acceptance
- Given a user who opens the Home view three times in a row or clicks "Surprise Me"
- When the hero banner selects a featured title from the candidate pool
- Then each selection is excluded from the next 12 hero selections and no two consecutive opens show the same hero title

- Given a user who pins "Sci-Fi" and "Thriller" on the taste bar or likes a Sci-Fi movie
- When the Home view renders
- Then the "Picked for You" row and hero pool prioritize Sci-Fi and Thriller titles while keeping carousel ordering fresh via weighted random jitter
