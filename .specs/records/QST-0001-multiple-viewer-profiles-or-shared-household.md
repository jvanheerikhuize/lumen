---
id: QST-0001
title: Should household members each have their own isolated state or share a single household state?
status: answered
blocks: [REQ-0008]
answered_by: FCT-0009
---

## Question
Should different members in a household share a single global watch history and favorites list, or should each household member have their own isolated state?

## Answer
Different members in the household each have their own isolated state (`FCT-0009`). Every household member profile maintains its own separate watch progress (`Continue Watching` up to `40` entries), per-episode resume timestamps, pinned favorite live channels (up to `200` entries), and personalized genre/liked taste profile, while still launching immediately into the active member profile with zero login credentials or cloud accounts.
