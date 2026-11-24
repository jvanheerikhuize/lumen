# Lumen — Technology Stack Reference

This document lives **outside** the epistemic specification corpus (`.specs/records/`) so that all assumptions (`ASM`), facts (`FCT`), questions (`QST`), requirements (`REQ`), and specifications (`SPC`) remain 100% technology-stack-neutral.

Spec-Blueprint: asdlc-meta-specs@latest
Design-System: asdlc-meta-designsystem@latest

---

## Current Production Appliance Stack

Lumen compiles and ships as a single self-contained binary with the frontend bundle embedded directly into the executable (`//go:embed`).

| Layer | Technology | Role |
| :--- | :--- | :--- |
| **Backend & HTTP Appliance** | **Go 1.24+** (`net/http`, `cmd/lumen`, `internal/server`) | Single-binary HTTP server, API router, static SPA asset server (`io/fs`), and SSRF-guarded outbound proxy. |
| **Frontend SPA** | **Vite 8** + **React 19** + **TypeScript 5** (`src/`) | Client-side single-page application compiled to `web/dist` and embedded into the Go binary via `web/embed.go`. |
| **Styling & Icons** | **Tailwind CSS 4** + **Lucide React** | Dark-mode glassmorphic UI, viewport-aware custom dropdowns, 4-layer playback timeline scrubber, and responsive layout. |
| **App Design Tokens** | **ASDLC Meta-DesignSystem** (`asdlc-meta-designsystem@latest`) | `tokens.css` imported in a low-priority layer by `src/app/globals.css`, with fallbacks; used by the page canvas and the player controls. |
| **App Components** | **ASDLC Meta-DesignSystem** atoms, vendored at a pinned commit | `ui-button`, `ui-chip`, `ui-switch`, `ui-badge`, `ui-input` in `src/vendor/design-system`, wrapped by `src/components/ds.tsx` and bundled so the app never loads remote code. |
| **Specification Workspace UI** | **ASDLC Meta-DesignSystem** (`asdlc-meta-designsystem@latest`) | Zero-build Web Component Atoms, Molecules, Organisms, and Templates powering `site/`. |
| **BitTorrent Streaming Engine** | **`github.com/anacrolix/torrent`** (`internal/streaming`) | Real-time torrent swarm ingestion, custom piece prioritization (head/tail + dynamic readhead window), contiguous downloaded-range calculation, and 20-minute idle eviction. |
| **Media Remuxing & Transcoding** | **`ffmpeg` / `ffprobe`** (`internal/streaming`) | Runtime codec probing (`h264`, `hevc`, `aac`, `ac3`, `eac3`), zero-CPU fragmented MP4 (`fMP4`) copy-remuxing, and on-the-fly audio/video transcoding with `-ss` seek support. |
| **Live TV HLS Playback** | **`hls.js` 1.7** + Native `<video>` (`src/components/VideoPlayerModal.tsx`, `internal/live`) | Adaptive HLS live stream playback paired with `/api/live/proxy` manifest rewriting and CORS relay. |
| **Household State Persistence** | **`modernc.org/sqlite`** (`internal/state`) | Pure-Go (CGO-free) embedded SQLite database (`WAL` mode) storing isolated per-member household profiles (`name`, `nickname`, `color`, `is_kids`), watch progress, per-episode resume positions, and favorite live channels. |
| **Network Security Guard** | **`net.Dialer.Control`** (`internal/netguard`) | Socket-level IP validation at connection time to block RFC1918, loopback, link-local, CGNAT, and cloud metadata addresses and eliminate DNS-rebinding TOCTOU windows. |

---

## Directory Layout

- `cmd/lumen/main.go` — Single-binary entrypoint (`PORT`, `LUMEN_DATA_DIR`).
- `internal/server/` — HTTP handlers for `/api/health`, `/api/media/*` (including `?kids=1` filtering in `kids.go`), `/api/stream`, `/api/stream/status`, `/api/subtitles/*`, `/api/live/*`, and `/api/state`.
- `internal/metadata/` — Keyless Cinemeta, AniList GraphQL, `Fribb/anime-lists` cross-reference, and optional TMDB client with genre ID mapping.
- `internal/scrapers/` — Concurrent Torrentio, TPB (`apibay.org`), YTS, Nyaa/AnimeTosho, and custom Stremio addon scrapers with source ranking.
- `internal/streaming/` — `anacrolix/torrent` engine, piece prioritization, `downloadedRanges` telemetry, and `ffmpeg`/`ffprobe` remux/transcode pipeline.
- `internal/live/` — Free-TV/IPTV M3U parser with ISO country attribution & `Kids` classification, fallback broadcaster list, and HLS manifest/segment CORS proxy.
- `internal/subtitles/` — OpenSubtitles REST v3 lookup and SRT-to-WebVTT converter.
- `internal/state/` — Embedded SQLite per-member household state store (`profile_watch_progress`, `profile_favorite_channels`, `household_profiles`).
- `internal/personalization/` — Genre affinity scoring and row personalization.
- `internal/netguard/` — SSRF and DNS-rebinding socket-level dialer guard.
- `src/` — React 19 + TypeScript SPA source (`Navbar`, `HeroBanner`, `MediaCarousel`, `PosterCard`, `CatalogBrowser`, `MediaDetailModal`, `LiveChannelsGrid`, `VideoPlayerModal`, `SettingsModal`, `ProfileManagerModal`, `SelectDropdown`, `kidsFilter.ts`, `watchProgress.ts`).
- `web/` — `//go:embed all:dist` package embedding the production Vite bundle.
