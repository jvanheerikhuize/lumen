package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

var kidsBlockedGenres = map[string]bool{
	"horror":        true,
	"crime":         true,
	"thriller":      true,
	"war":           true,
	"psychological": true,
	"mystery":       true,
	"romance":       true,
	"ecchi":         true,
	"hentai":        true,
	"adult":         true,
	"erotica":       true,
	"film-noir":     true,
	"cyberpunk":     true,
}

var kidsPrimaryGenres = map[string]bool{
	"family":   true,
	"kids":     true,
	"children": true,
}

var kidsAnimeAllowedGenres = map[string]bool{
	"adventure":     true,
	"comedy":        true,
	"fantasy":       true,
	"music":         true,
	"slice of life": true,
	"sports":        true,
	"sci-fi":        true,
	"family":        true,
	"kids":          true,
}

var kidsCinemetaGenres = []string{
	"Family", "Animation", "Adventure", "Comedy", "Fantasy", "Music", "Sport",
}

var kidsAnimeGenres = []string{
	"Adventure", "Comedy", "Fantasy", "Music", "Slice of Life", "Sports",
}

func isKidsModeRequest(r *http.Request) bool {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kids")))
	return v == "1" || v == "true" || v == "yes"
}

// IsKidsSafeMedia reports whether a MediaItem is age-appropriate for Kids profiles.
func IsKidsSafeMedia(item types.MediaItem) bool {
	if len(item.Genres) == 0 {
		return false
	}
	hasPrimary := false
	hasMatureTone := false
	hasAnimeAllowed := false
	isAnime := item.Anime != nil
	for _, g := range item.Genres {
		norm := strings.ToLower(strings.TrimSpace(g))
		if norm == "" {
			continue
		}
		if kidsBlockedGenres[norm] {
			return false
		}
		if kidsPrimaryGenres[norm] {
			hasPrimary = true
		}
		if norm == "action" || norm == "drama" {
			hasMatureTone = true
		}
		if isAnime && kidsAnimeAllowedGenres[norm] {
			hasAnimeAllowed = true
		}
	}
	return hasPrimary || (hasAnimeAllowed && !hasMatureTone)
}

// FilterKidsSafeMedia filters items to age-appropriate Kids titles.
func FilterKidsSafeMedia(items []types.MediaItem) []types.MediaItem {
	out := make([]types.MediaItem, 0, len(items))
	for _, item := range items {
		if IsKidsSafeMedia(item) {
			out = append(out, item)
		}
	}
	return out
}

func dedupeMediaItems(items []types.MediaItem) []types.MediaItem {
	seen := make(map[string]bool, len(items))
	out := make([]types.MediaItem, 0, len(items))
	for _, item := range items {
		key := item.ImdbID
		if key == "" {
			key = item.IDString()
		}
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(item.Title))
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func isAllowedKidsGenre(genre string, allowed []string) bool {
	return slices.Contains(allowed, genre)
}
