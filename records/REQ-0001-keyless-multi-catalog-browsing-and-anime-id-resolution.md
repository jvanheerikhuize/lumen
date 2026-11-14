---
id: REQ-0001
title: Keyless multi-catalog browsing, cross-catalog search, and anime identifier resolution
status: active
because: [FCT-0001, FCT-0002]
---

## Statement
The system MUST provide zero-configuration browsing and search across Movies, Series, and Anime catalogs without requiring user accounts or mandatory API keys.
- **Movies & Series catalogs**: Support trending, popular, top-rated, genre-filtered, and release-year-filtered views, plus full title details with IMDb identifiers, cast, runtime, ratings, and season/episode lists.
- **Anime catalogs**: Support trending, current-season, popular, and top-rated views, and automatically cross-reference each seasonal anime entry (`anilist:<id>`) against the community anime mapping dataset (refreshed every `24 hours` with fallback mirror support) to resolve its `kitsuId` (for episode stream queries) and `imdbId` + `season` number (for subtitle queries).
- **Unified search**: Query Movie, Series, and Anime catalogs concurrently for any search query of at least `2` characters.

## Acceptance
- Given a fresh deployment with no environment variables or API keys configured
- When a user browses the Home, Movies, Series, or Anime views or searches for a title
- Then the system returns populated catalog items with posters, ratings, genres, and episode lists

- Given an anime entry selected from the Anime catalog (`anilist:<id>`)
- When the user opens a specific episode for playback
- Then the system resolves the corresponding Kitsu identifier for stream discovery and the IMDb identifier + season number for subtitle lookup
