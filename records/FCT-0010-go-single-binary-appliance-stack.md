---
id: FCT-0010
title: The reference implementation runs as a single-binary Go 1.24+, embedded Vite/React 19 SPA, pure-Go SQLite, anacrolix/torrent, and FFmpeg appliance
status: active
evidence: "Verified in go.mod, cmd/lumen/main.go, web/embed.go, vite.config.mts, and Dockerfile: Go 1.26.8 single binary (~41 MB unstripped) embedding web/dist via //go:embed, github.com/anacrolix/torrent v1.62.0, modernc.org/sqlite v1.49.1, Vite 8.3.8, React 19.3.1, Tailwind CSS 4, hls.js 1.7.4, and Alpine 3.21 runtime with ffmpeg/ffprobe."
---

## Statement
The production reference implementation of Lumen is built and packaged as a single-binary Go appliance with an embedded React single-page application:
- **Runtime & Packaging**: Pure-Go (`CGO_ENABLED=0`) static binary compiled from `./cmd/lumen` (`go 1.24+` toolchain) and packaged in a minimal `alpine:3.21` container alongside system `ffmpeg`, `ffprobe`, and `ca-certificates`. The compiled frontend bundle (`web/dist`) is embedded directly into the binary via Go's `//go:embed all:dist` directive (`web/embed.go`) and served alongside `/api/*` on port `3000`.
- **Frontend & UI**: Vite `8` single-page application (`index.html`, `src/main.tsx`, `src/app/page.tsx`) with React `19.3.1`, TypeScript `5`, Tailwind CSS `4` (`@tailwindcss/postcss`), `lucide-react` (`1.72.0`) icons, native HTML5 `<video>` playback with `hls.js` (`1.7.4`) for MSE-based HTTP Live Streaming (`HLS`), and WebVTT `<track>` rendering.
- **Backend & Media Pipeline**: Go `net/http` server (`internal/server`) wiring `internal/metadata`, `internal/scrapers`, `internal/streaming` (`github.com/anacrolix/torrent` `v1.62.0` with piece-level priority scheduling for container headers, seek windows, and tail indices plus 20-minute idle swarm eviction), `internal/subtitles` (SRT-to-WebVTT conversion with cue shift and language normalization), `internal/live` (curated + Free-TV channel directory, HLS manifest rewriter, and concurrent liveness prober), `internal/personalization` (taste scoring and non-repeating hero selection), `internal/netguard` (`net.Dialer.Control` SSRF/DNS-rebinding guard), and `internal/state` (`modernc.org/sqlite` `v1.49.1` WAL database in `LUMEN_DATA_DIR/lumen.db` with `state.json` mirror).
- **Verification & Spec Tooling**: Table-driven Go unit and HTTP integration tests (`go test ./cmd/... ./internal/... ./web/...`), Node.js test runner (`tests/**/*.test.ts`), and Python `3` (`tools/spec.py`) for validating and rendering the ASDLC Meta-Specs record graph.
