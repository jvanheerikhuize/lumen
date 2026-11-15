# Changelog

All notable changes to **Lumen** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-11-15

### Added
- **Household Profile Manager & Nicknames (`REQ-0008`):** Manage household members right from the navigation bar (`ProfileManagerModal`), customize editable `nickname` labels (`nickname || name`), pick avatar color swatches, create or remove profiles, and toggle per-profile `Kids account` mode (`isKids`) synced via SQLite (`/api/state`).
- **Balanced Kids Account (`?kids=1`):** Dedicated Kids experience filtering Home rows, Hero picks, Search (`Search Kids`), Taste Genres, Catalogs (`internal/server/kids.go`, `src/lib/kidsFilter.ts`), and **Kids Live TV** (`Kids` channel category across `All countries`).
- **Viewport-Aware Custom Dropdowns (`SelectDropdown`):** Custom searchable/frosted selector replacing native `<select>` controls and automatically flipping upward or clamping horizontally inside viewports and `<dialog>` modals.

### Changed
- **UI Checkup & Top Menu Legibility:** Strengthened fixed-header gradient scrims, eliminated outer header backdrop-root conflicts so frosted popovers blur cleanly behind them, unified modal/player popup surfaces (`bg-zinc-950/[0.98]` / `bg-zinc-900/95 backdrop-blur-2xl`), centered pill/button icons & text metrics (with descender protection for `.truncate` labels), and eliminated mobile popover horizontal overflow.
- **Direct Free-TV Station Directory (`REQ-0010`):** Stopped prepending hardcoded curated stations onto the front of Live TV rows (keeping curated channels strictly as offline fallback), cleared broken `redbull-tv` logo URLs, and classified Free-TV `Kids` broadcasters.

## [0.2.0] - 2026-11-14

### Changed
- **Single-binary Go appliance rewrite (`FCT-0010`):** Replaced the Next.js/Node.js runtime (`FCT-0009`) with a self-contained pure-Go (`CGO_ENABLED=0`, Go 1.24+) server (`cmd/lumen`, `internal/*`) embedding the compiled Vite 8 + React 19 + Tailwind CSS 4 SPA (`web/embed.go` via `//go:embed all:dist`).
- **BitTorrent swarm engine (`anacrolix/torrent`):** Native Go swarm manager (`internal/streaming/torrent_engine.go`) with piece-level priority scheduling (`headTarget = clamp(2% of fileSize, 16 MB, 64 MB)`, `4 MB` tail for container `moov`/`Cues`, `32 MB` dynamic read-ahead window), HTTP `206 Partial Content` streaming, and 20-minute idle swarm eviction.
- **Atomic SQLite household state sync (`FCT-0011`, promoted from `ASM-0004`):** Replaced whole-file `state.json` overwrite with an embedded pure-Go SQLite (`modernc.org/sqlite`) WAL database (`LUMEN_DATA_DIR/lumen.db`) performing transactional per-item `updatedAt` upserts and per-episode progress merges (with automatic migration from and mirroring to `state.json`).
- **Multi-stage Alpine container (`Dockerfile`):** Compiles the Vite SPA and static Go binary into a minimal `alpine:3.21` + `ffmpeg` runtime image.

### Security
- **Socket-level DNS-rebinding & SSRF guard (`FCT-0012`):** `internal/netguard` verifies the resolved destination IP address inside `net.Dialer.Control` immediately prior to `connect(2)`, eliminating the time-of-check to time-of-use (TOCTOU) window between hostname resolution and socket connection across initial connections and manual redirect hops.

## [0.1.0] - 2026-11-14

### Added
- **Zero-config catalogs:** Movies and TV series via Cinemeta, Anime via AniList + Fribb ID mapping, and airing-today schedule via TVmaze.
- **Personalization & discovery:** Randomized Hero banner (never repeating recent titles), automatic genre taste learning, interactive **"Your taste"** genre chips, **"Surprise me"** / **"Shuffle picks"**, **"Picked for You"**, and **"Your Liked Titles"** rows.
- **Continue Watching & Up Next:** Per-episode and movie resume memory, **"Continue Watching"** carousel with progress bars, **"Up Next"** auto-advance overlay in the final 35s, and LAN state sync (`/api/state`).
- **Unified global search:** Parallel search across Movies, TV Series, Anime (`TMDB`/`Cinemeta` + `AniList`), and Live TV channels with `/` keyboard shortcut.
- **Torrent streaming & on-the-fly conversion:** BitTorrent swarm engine with head/tail piece prioritization (`moov` atom first), HTTP range serving, `ffprobe` codec inspection, on-the-fly `ffmpeg` remuxing/transcoding to fragmented MP4 (`fMP4`), optional hardware acceleration (`LUMEN_HWACCEL`), and concurrent transcode limiting.
- **In-player controls:** Source switcher, audio track switcher, embedded MKV subtitle extraction (`/api/subtitles/embedded`), OpenSubtitles SRT-to-WebVTT proxy with live `±0.5s` sync (`G`/`H`), playback speed (`0.5x–2x`), Picture-in-Picture, `navigator.mediaSession`, and mobile tap/double-tap gestures.
- **Live TV:** ~1,700 free-to-air channels from Free-TV/IPTV + curated feeds with category/country/favorite (`★`) filters, batch on-air verification (`/api/live/status`), Prev/Next channel surfing, and automatic CORS HLS relay (`/api/live/proxy`).
- **Docker, PWA & self-hosted ecosystem:** Multi-arch `Dockerfile`, `compose.yaml`, `/api/health` endpoint, PWA manifest, and CasaOS / Unraid / Homepage templates.
