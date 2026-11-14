---
id: SPC-0002
title: Embedded and external subtitle pipeline
status: active
tier: feature
depends_on: [SPC-0001]
requirements: [REQ-0006]
---

## Purpose
Layers multi-language subtitle support onto the playback skeleton: extracting embedded text subtitle tracks from media containers and querying external subtitle providers by canonical IMDb/season/episode identifiers, normalized to WebVTT with live `±0.5s` synchronization adjustment.
