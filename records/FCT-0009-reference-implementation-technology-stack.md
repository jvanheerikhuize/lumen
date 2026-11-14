---
id: FCT-0009
title: The legacy reference implementation ran as a single-container Node.js 22+, Next.js 16, React 19, TypeScript 5, and FFmpeg stack
status: deprecated
evidence: "Superseded by FCT-0010 (Go 1.24+ single-binary appliance with embedded Vite/React 19 SPA, anacrolix/torrent, and modernc.org/sqlite). Historically verified in package.json, next.config.ts, and Dockerfile."
---

## Statement
The reference implementation of Lumen is built and packaged with the following concrete technology stack:
- **Runtime & Packaging**: Node.js `>=22.18` (`node:25-bookworm-slim` in Docker) running a Next.js `16.4.6` standalone server (`output: "standalone"`), with system `ffmpeg` and `ffprobe` binaries installed on `PATH` and persistent household state stored as JSON (`LUMEN_DATA_DIR/state.json`) alongside browser `localStorage`.
- **Frontend & UI**: React `19.3.1`, TypeScript `5`, Tailwind CSS `4` (`@tailwindcss/postcss`), `lucide-react` (`1.72.0`) icons, native HTML5 `<video>` playback with `hls.js` (`1.7.4`) for MSE-based HTTP Live Streaming (`HLS`), and WebVTT `<track>` rendering.
- **Backend & Media Pipeline**: Next.js App Router API routes (`src/app/api/*`) running on the Node.js runtime, `torrent-stream` (`1.2.1`) for peer-to-peer BitTorrent swarm management and piece prioritization, child-process `ffprobe` (JSON stream inspection) and `ffmpeg` (fragmented MP4 remuxing/transcoding with optional `vaapi`, `nvenc`, `qsv`, or `videotoolbox` hardware acceleration and SRT subtitle extraction), and `fast-xml-parser` (`5.12.1`) for Jackett Torznab XML parsing.
- **Verification & Spec Tooling**: Node.js built-in test runner (`node --test "tests/**/*.test.ts"`), ESLint `10`, and Python `3` (`tools/spec.py` with `PyYAML`) for validating and rendering the ASDLC Meta-Specs record graph.
