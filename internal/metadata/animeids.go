package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sync"
	"time"
)

var animeMappingURLs = []string{
	"https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-mini.json",
	"https://cdn.jsdelivr.net/gh/Fribb/anime-lists@master/anime-list-mini.json",
}

const animeRefreshInterval = 24 * time.Hour

// AnimeIDs holds resolved Kitsu and IMDb identifiers for an AniList entry.
type AnimeIDs struct {
	KitsuID *int
	ImdbID  string
	Season  *int
}

type rawMappingEntry struct {
	AnilistID int             `json:"anilist_id"`
	KitsuID   int             `json:"kitsu_id"`
	ImdbID    json.RawMessage `json:"imdb_id"`
	Season    *struct {
		Tvdb int `json:"tvdb"`
		Tmdb int `json:"tmdb"`
	} `json:"season"`
}

var (
	imdbRe        = regexp.MustCompile(`^tt\d+$`)
	animeMapMu    sync.Mutex
	animeMapCache map[int]AnimeIDs
	animeMapAt    time.Time
)

// ParseAnimeMapping parses a Fribb/anime-lists JSON payload into an anilistId -> AnimeIDs lookup map.
func ParseAnimeMapping(data []byte) (map[int]AnimeIDs, error) {
	var list []rawMappingEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	m := make(map[int]AnimeIDs, len(list))
	for _, e := range list {
		if e.AnilistID == 0 {
			continue
		}
		var imdb string
		if len(e.ImdbID) > 0 {
			var single string
			if err := json.Unmarshal(e.ImdbID, &single); err == nil {
				imdb = single
			} else {
				var arr []string
				if err := json.Unmarshal(e.ImdbID, &arr); err == nil && len(arr) > 0 {
					imdb = arr[0]
				}
			}
		}
		if !imdbRe.MatchString(imdb) {
			imdb = ""
		}

		var kitsuPtr *int
		if e.KitsuID > 0 {
			k := e.KitsuID
			kitsuPtr = &k
		}
		var seasonPtr *int
		if e.Season != nil {
			s := e.Season.Tvdb
			if s == 0 {
				s = e.Season.Tmdb
			}
			if s > 0 {
				seasonPtr = &s
			}
		}
		m[e.AnilistID] = AnimeIDs{
			KitsuID: kitsuPtr,
			ImdbID:  imdb,
			Season:  seasonPtr,
		}
	}
	return m, nil
}

func downloadAnimeMap(ctx context.Context, urls []string) (map[int]AnimeIDs, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	for _, u := range urls {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			cancel()
			continue
		}
		var raw json.RawMessage
		err = json.NewDecoder(resp.Body).Decode(&raw)
		_ = resp.Body.Close()
		cancel()
		if err != nil {
			continue
		}
		m, err := ParseAnimeMapping(raw)
		if err == nil && len(m) > 0 {
			return m, nil
		}
	}
	return nil, errors.New("anime id mapping unavailable")
}

// GetAnimeIDs resolves an AniList ID to its Kitsu ID, IMDb ID, and season number.
func GetAnimeIDs(ctx context.Context, anilistID int) (AnimeIDs, bool) {
	animeMapMu.Lock()
	if animeMapCache != nil && time.Since(animeMapAt) < animeRefreshInterval {
		res, ok := animeMapCache[anilistID]
		animeMapMu.Unlock()
		return res, ok
	}
	animeMapMu.Unlock()

	m, err := downloadAnimeMap(ctx, animeMappingURLs)
	animeMapMu.Lock()
	defer animeMapMu.Unlock()
	if err == nil {
		animeMapCache = m
		animeMapAt = time.Now()
	} else {
		if animeMapCache == nil {
			animeMapCache = make(map[int]AnimeIDs)
		}
		animeMapAt = time.Now().Add(-animeRefreshInterval + 15*time.Minute)
	}
	res, ok := animeMapCache[anilistID]
	return res, ok
}
