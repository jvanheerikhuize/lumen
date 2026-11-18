---
id: QST-0002
title: How should the home screen balance personal taste affinity against discovery freshness on each open?
status: answered
blocks: [REQ-0007]
answered_by: ASM-0002
---

## Question
If the home screen sorts strictly by taste score, the same top-scored title appears every time; if it shuffles purely at random, low-quality or disliked genres dominate. The chosen approach is recorded in `ASM-0002`: compute a per-title affinity score from genre interactions, pinned genres, and community rating, add bounded random jitter (`0..8`), and exclude the last `12` hero IDs.
