---
id: FCT-0004
title: Media containers store stream and seek index tables at either the start or the final megabytes of the file
status: active
promoted_from: ASM-0001
evidence: "Verified in the peer-to-peer streaming engine: MP4 files without fast-start optimization place their moov atom at the end of the file, and Matroska files place Cues at either boundary; prioritizing the first 2% (16–64 MB) and final 4 MB allows stream inspection and playback to start while the middle of the file downloads sequentially."
---

## Statement
Container formats such as MP4 and Matroska store essential stream headers and seek index tables (`moov` atom or `Cues`) at either the beginning of the file or within the final few megabytes. A sequential downloader that does not prioritize both the leading byte range and the trailing byte range of the selected media file cannot inspect stream codecs or begin playback until the entire file has finished downloading.
