---
id: ASM-0003
title: A 94% playback completion threshold separates finished movies from in-progress movies
status: open
confidence: 0.8
---

## Statement
When a viewer reaches at least `94%` of a movie's total duration, the movie has reached its end credits and can be automatically removed from the "Continue Watching" row, whereas episodes of a series should retain per-episode progress and advance the series card to the next episode.

## How we validate it
Check across varied movie runtimes (90-minute films through 3-hour epics) whether `94%` consistently falls inside end credits without prematurely clearing movies before the final scene.
