---
id: REQ-0012
title: Pluggable stream addon manifests and self-hosted torrent indexer integration
status: active
because: [FCT-0005, FCT-0008]
---

## Statement
The system MUST allow users to extend stream discovery with custom Stremio-compatible addon manifests and self-hosted torrent indexers (Prowlarr and Jackett Torznab):
1. **Custom addon manifests**: Normalize user-supplied addon URLs (stripping `/manifest.json`, enforcing SSRF host checks unless private LAN addons are opted in), query `/stream/{type}/{id}.json` in parallel with built-in addons, and tag returned sources with the addon's hostname.
2. **Self-hosted indexers**: Support configuring Prowlarr (`/api/v1/search`) and Jackett (`/api/v2.0/indexers/all/results/torznab/api`) via environment variables or runtime Settings UI, resolving magnet URIs and info-hashes for movie and series queries (`SxxExx`), and providing an interactive connection health check endpoint (`/api/integrations/test`).

## Acceptance
- Given a user who configures a Prowlarr or Jackett URL and API key in the Settings modal and clicks "Test"
- When the server queries the indexer health endpoint and subsequent stream searches run
- Then the health check reports connection latency and status, and matching indexer torrents are merged into the ranked stream source list
