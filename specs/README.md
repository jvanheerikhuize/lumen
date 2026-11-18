# Specifications Index

> **Auto-generated from `.specs/records/`.** Do not hand-edit files in `specs/`; update the atomic records in `.specs/records/` and re-render.

## Topological Build & Reading Order

| Spec | Tier | Status | Readiness | Depends On | Title |
| :--- | :--- | :--- | :--- | :--- | :--- |
| [`SPC-0001`](SPC-0001.md) | `skeleton` | `active` | **100%** | — | Core catalog discovery, stream resolution, and adaptive playback skeleton |
| [`SPC-0002`](SPC-0002.md) | `feature` | `active` | **100%** | SPC-0001 | Embedded and external subtitle pipeline |
| [`SPC-0003`](SPC-0003.md) | `feature` | `draft` | **67%** | SPC-0001 | Taste personalization, fresh discovery, and household progress sync |
| [`SPC-0004`](SPC-0004.md) | `feature` | `active` | **100%** | SPC-0001 | Free-to-air live television directory and cross-origin HLS relay |
| [`SPC-0005`](SPC-0005.md) | `feature` | `active` | **100%** | SPC-0001 | Network boundary protection and self-hosted indexer integration |

## Open Assumptions (Provisional Grounds)

- **`ASM-0002`**: Heuristic genre affinity weights and a 12-item hero history window keep discovery fresh and personal
