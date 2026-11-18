---
id: SPC-0005
title: Network boundary protection and self-hosted indexer integration
status: active
tier: feature
depends_on: [SPC-0001]
requirements: [REQ-0011, REQ-0012]
---

## Purpose
Hardens all outbound server fetches against SSRF, DNS rebinding, redirect hops, and unbounded responses while allowing users to attach custom stream addon manifests and self-hosted Prowlarr/Jackett torrent indexers with live connection testing.
