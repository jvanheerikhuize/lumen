---
id: ASM-0003
title: A 94% playback completion threshold separates finished movies from in-progress movies
status: invalidated
confidence: 0.1
---

## Statement
When a viewer reaches at least `94%` of a movie's total duration, the movie has reached its end credits and can be automatically removed from the "Continue Watching" row, whereas episodes of a series should retain per-episode progress and advance the series card to the next episode.

## How we validate it
Empirically tested against single-part non-25fps English subtitle dialogue timestamps and catalog runtimes across 14 films spanning `81m–181m`. The `94%` threshold cut **before** final non-stinger dialogue or closing montage audio on **10 of 14 films (`71.4%`)**, including *Toy Story* (`99.60%`, `-272s`), *Finding Nemo* (`100.42%`, `-385s`), *The Hangover* (`95.97%`, `-118s`), *Groundhog Day* (`96.96%`, `-180s`), *Cars* pre-stinger (`95.98%`, `-139s`), *Forrest Gump* (`94.33%`, `-28s`), *The Dark Knight* (`94.99%`, `-90s`), *Pulp Fiction* (`96.89%`, `-267s`), *Interstellar* (`96.90%`, `-294s`), and *Oppenheimer* (`96.28%`, `-248s`; final line at `95.81%`). Superseded by empirically grounded `FCT-0012` (`>= 97%`).
