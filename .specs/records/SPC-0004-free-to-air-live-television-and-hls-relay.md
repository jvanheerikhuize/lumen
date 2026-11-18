---
id: SPC-0004
title: Free-to-air live television directory and cross-origin HLS relay
status: active
tier: feature
depends_on: [SPC-0001]
requirements: [REQ-0010]
---

## Purpose
Layers live television onto the streaming dashboard: serving ~1,700 free-to-air IPTV channels directly (with curated international broadcaster fallback) alongside `Kids` channel classification, category/country/favorite filters, bounded batch on-air stream verification, and a server-side HLS manifest and segment proxy that rewrites relative and attribute URIs.
