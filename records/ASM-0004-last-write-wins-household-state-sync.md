---
id: ASM-0004
title: Whole-list last-write-wins state synchronization is sufficient for household multi-device usage
status: open
confidence: 0.7
---

## Statement
Persisting up to `40` watch progress items and `200` favorite channel IDs on the server via whole-list last-write-wins replacement (merged by `updatedAt` timestamp on client hydration) is sufficient for a single household switching between a living-room television, laptop, and tablet.

## How we validate it
Test concurrent playback and channel favoriting across two household devices on the same local network and verify whether any watch progress entry is unexpectedly overwritten.
