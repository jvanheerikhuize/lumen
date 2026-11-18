---
id: REQ-0009
title: Live broadcast schedule and series air-date status enrichment
status: active
because: [FCT-0001]
---

## Statement
The system MUST enrich series discovery and detail views with live broadcast schedule data:
1. **"What's On Today" row**: Aggregate series airing new episodes on the current date (resolved by IMDb ID or title lookup) and display them in a dedicated Home carousel with episode badges (`S2 · E5`).
2. **Series status badge**: In the detail modal for any series, display whether the show is returning or ended alongside the next scheduled episode (`Next: Sx · Ey · YYYY-MM-DD`) or most recent episode air date.

## Acceptance
- Given a returning television series with an upcoming or recently aired episode
- When a user opens the series detail modal
- Then the modal displays the series status and the next or latest episode code and air date
