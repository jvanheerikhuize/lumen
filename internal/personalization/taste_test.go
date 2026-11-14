package personalization

import (
	"testing"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

func makeMedia(id, title string, genres []string, vote float64) types.MediaItem {
	return types.MediaItem{
		ID:           id,
		ImdbID:       id,
		Title:        title,
		Overview:     title + " overview",
		PosterPath:   types.StringPtr("https://example.com/" + id + "-poster.jpg"),
		BackdropPath: types.StringPtr("https://example.com/" + id + "-backdrop.jpg"),
		ReleaseDate:  "2025-01-01",
		VoteAverage:  vote,
		MediaType:    "movie",
		Genres:       genres,
	}
}

func TestScoreMediaForUser(t *testing.T) {
	sciFi := makeMedia("tt1", "Dune", []string{"Sci-Fi", "Adventure"}, 8.4)
	comedy := makeMedia("tt2", "Superbad", []string{"Comedy"}, 7.6)
	profile := NewTasteProfile()
	profile.FavoriteGenres = []string{"Sci-Fi"}
	profile.GenreScores["Sci-Fi"] = 6

	if !(ScoreMediaForUser(sciFi, profile) > ScoreMediaForUser(comedy, profile)) {
		t.Fatalf("expected sciFi score > comedy score")
	}
}

func TestPickRandomHeroAvoidsRecentAndExclude(t *testing.T) {
	m1 := makeMedia("tt1", "Movie One", []string{"Action"}, 7.5)
	m2 := makeMedia("tt2", "Movie Two", []string{"Sci-Fi"}, 7.5)
	m3 := makeMedia("tt3", "Movie Three", []string{"Thriller"}, 7.5)
	rows := []types.CatalogRow{{Key: "popular-movies", Items: []types.MediaItem{m1, m2, m3}}}

	profile := NewTasteProfile()
	profile.RecentHeroIDs = []string{"tt1"}

	picked := PickRandomHero(rows, profile, "tt2")
	if picked == nil || picked.IDString() != "tt3" {
		t.Fatalf("expected tt3, got %+v", picked)
	}
}

func TestPersonalizeRows(t *testing.T) {
	m1 := makeMedia("tt1", "Blade Runner 2049", []string{"Sci-Fi", "Thriller"}, 8.1)
	m2 := makeMedia("tt2", "Arrival", []string{"Sci-Fi", "Drama"}, 7.9)
	m3 := makeMedia("tt3", "The Notebook", []string{"Romance", "Drama"}, 7.8)

	profile := NewTasteProfile()
	profile.GenreScores["Sci-Fi"] = 12
	profile.FavoriteGenres = []string{"Sci-Fi"}
	profile.LikedItems = []types.MediaItem{m1}

	rows := PersonalizeRows([]types.CatalogRow{
		{Key: "popular-movies", Title: "Popular Movies", Items: []types.MediaItem{m3, m2, m1}},
		{Key: "genre-romance", Title: "Romance", Items: []types.MediaItem{m3}},
		{Key: "genre-sci-fi", Title: "Sci-Fi", Items: []types.MediaItem{m1, m2}},
	}, profile)

	if rows[0].Key != "for-you" || rows[0].Title != "Picked for You" {
		t.Fatalf("expected first row to be for-you, got %+v", rows[0])
	}
	if rows[1].Key != "my-likes" || rows[1].Title != "Your Liked Titles" {
		t.Fatalf("expected second row to be my-likes, got %+v", rows[1])
	}
	sciFiIdx, romanceIdx := -1, -1
	for i, r := range rows {
		if r.Key == "genre-sci-fi" {
			sciFiIdx = i
		}
		if r.Key == "genre-romance" {
			romanceIdx = i
		}
	}
	if !(sciFiIdx < romanceIdx) {
		t.Fatalf("expected genre-sci-fi (%d) before genre-romance (%d)", sciFiIdx, romanceIdx)
	}
}
