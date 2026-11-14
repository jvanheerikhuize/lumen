---
id: QST-0001
title: Should a future rebuild support multiple household viewer profiles or keep a single shared household state?
status: answered
blocks: [REQ-0008]
answered_by: FCT-0011
---

## Question
Currently, watch progress, favorite live channels, and taste profiles represent a single shared household state (`40` watch progress entries, `200` favorite channels). To preserve Lumen's zero-account, zero-prompt launch experience across living-room TVs, laptops, and tablets, the system intentionally maintains a single shared household state synchronized via atomic per-item timestamped upserts (`FCT-0011`) rather than requiring a viewer profile picker on startup.
