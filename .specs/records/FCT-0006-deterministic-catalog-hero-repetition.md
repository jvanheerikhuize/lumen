---
id: FCT-0006
title: Deterministic catalog ordering displays the exact same top-ranked title on the home hero banner on every session
status: active
evidence: "Observed during live user testing: fetching trending catalogs in their default upstream order caused the home screen hero banner and carousels to show the exact same lead movie every time the application was opened."
---

## Statement
Upstream trending and popular catalogs change slowly from hour to hour. If the home screen selects its featured hero title and carousel order deterministically from the top of the upstream response without excluding recently shown hero IDs and applying weighted randomization, users see the exact same title every time they open the application.
