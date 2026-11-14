---
id: QST-0001
title: Should a future rebuild support multiple household viewer profiles or keep a single shared household state?
status: open
blocks: [REQ-0008]
---

## Question
Currently, watch progress, favorite live channels, and taste profiles represent a single shared household state (`40` watch progress entries, `200` favorite channels). Before locking in the persistence schema for a future rebuild, should the system partition state by viewer profile (e.g., "Who's watching?" profile picker) or remain zero-prompt with one shared household profile?
