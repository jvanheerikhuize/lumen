package metadata

import (
	"reflect"
	"testing"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

func TestAnilistToMediaItem(t *testing.T) {
	var m AnilistMedia
	m.ID = 154587
	m.Title.Romaji = "Sousou no Frieren"
	m.Title.English = "Frieren: Beyond Journey's End"
	m.Description = "An elf mage reflects on her journey.<br><br><i>Second paragraph.</i> (Source: Crunchyroll)"
	m.CoverImage.ExtraLarge = "https://s4.anilist.co/cover.jpg"
	m.BannerImage = "https://s4.anilist.co/banner.jpg"
	m.Format = "TV"
	m.AverageScore = 91
	m.Genres = []string{"Adventure", "Drama", "Fantasy", "Ecchi"}
	m.StartDate.Year = 2023
	m.StartDate.Month = 9
	m.StartDate.Day = 29

	item := AnilistToMediaItem(m)

	if item.ID != "al-154587" {
		t.Fatalf("expected id al-154587, got %v", item.ID)
	}
	if item.Title != "Frieren: Beyond Journey's End" {
		t.Fatalf("expected English title, got %s", item.Title)
	}
	if item.OriginalTitle != "Sousou no Frieren" {
		t.Fatalf("expected romaji originalTitle, got %s", item.OriginalTitle)
	}
	if item.Overview != "An elf mage reflects on her journey.\n\nSecond paragraph." {
		t.Fatalf("unexpected overview: %q", item.Overview)
	}
	if item.MediaType != "tv" {
		t.Fatalf("expected tv, got %s", item.MediaType)
	}
	if item.FirstAirDate != "2023-09-29" {
		t.Fatalf("expected 2023-09-29, got %s", item.FirstAirDate)
	}
	if item.VoteAverage != 9.1 {
		t.Fatalf("expected 9.1, got %v", item.VoteAverage)
	}
	wantGenres := []string{"Anime", "Adventure", "Drama", "Fantasy"}
	if !reflect.DeepEqual(item.Genres, wantGenres) {
		t.Fatalf("expected genres %v, got %v", wantGenres, item.Genres)
	}
	if item.Anime == nil || item.Anime.AnilistID != 154587 {
		t.Fatalf("expected anime ref with anilistId 154587, got %+v", item.Anime)
	}
}

func TestParseAnimeMappingCrossReference(t *testing.T) {
	raw := []byte(`[
		{"anilist_id": 154587, "kitsu_id": 46474, "imdb_id": "tt22248376", "season": {"tvdb": 1}}
	]`)
	m, err := ParseAnimeMapping(raw)
	if err != nil {
		t.Fatalf("ParseAnimeMapping failed: %v", err)
	}
	ids, ok := m[154587]
	if !ok {
		t.Fatalf("expected anilist_id 154587 in map")
	}
	if ids.KitsuID == nil || *ids.KitsuID != 46474 || ids.ImdbID != "tt22248376" || ids.Season == nil || *ids.Season != 1 {
		t.Fatalf("unexpected ids: %+v", ids)
	}
}

func TestCinemetaToMediaItemMovie(t *testing.T) {
	item := CinemetaToMediaItem(CinemetaMeta{
		ImdbID:      "tt15239678",
		MoviedbID:   693134,
		Name:        "Dune: Part Two",
		Description: "Paul Atreides unites with Chani and the Fremen.",
		Released:    "2024-03-01T00:00:00.000Z",
		ImdbRating:  "8.8",
		Genres:      []string{"Sci-Fi", "Adventure"},
		Runtime:     "2 h 46 min",
		Trailers:    []CinemetaTrailer{{Source: "Way9Dexny3w"}},
	}, "movie")

	if item.ID != "tt15239678" || item.ImdbID != "tt15239678" {
		t.Fatalf("expected tt15239678, got %v / %s", item.ID, item.ImdbID)
	}
	if item.TmdbID == nil || *item.TmdbID != 693134 {
		t.Fatalf("expected tmdbId 693134, got %v", item.TmdbID)
	}
	if item.ReleaseDate != "2024-03-01" {
		t.Fatalf("expected releaseDate 2024-03-01, got %s", item.ReleaseDate)
	}
	if item.Duration == nil || *item.Duration != 166 {
		t.Fatalf("expected duration 166, got %v", item.Duration)
	}
	if item.VoteAverage != 8.8 {
		t.Fatalf("expected voteAverage 8.8, got %v", item.VoteAverage)
	}
	if item.PosterPath == nil || *item.PosterPath != "https://images.metahub.space/poster/medium/tt15239678/img" {
		t.Fatalf("unexpected posterPath: %v", item.PosterPath)
	}
	if item.BackdropPath == nil || *item.BackdropPath != "https://images.metahub.space/background/medium/tt15239678/img" {
		t.Fatalf("unexpected backdropPath: %v", item.BackdropPath)
	}
	if item.TrailerURL != "https://www.youtube.com/watch?v=Way9Dexny3w" {
		t.Fatalf("unexpected trailerUrl: %s", item.TrailerURL)
	}
}

func TestCinemetaToMediaItemTVSeasons(t *testing.T) {
	item := CinemetaToMediaItem(CinemetaMeta{
		ID:   "tt0903747",
		Name: "Breaking Bad",
		Year: "2008–2013",
		Videos: []CinemetaVideo{
			{Season: 0, Episode: 1, Name: "Special"},
			{Season: 2, Episode: 1, Name: "Seven Thirty-Seven"},
			{Season: 1, Episode: 2, Name: "Cat's in the Bag..."},
			{Season: 1, Episode: 1, Name: "Pilot", Released: "2008-01-20T00:00:00.000Z"},
		},
	}, "tv")

	if item.FirstAirDate != "2008-01-01" {
		t.Fatalf("expected 2008-01-01, got %s", item.FirstAirDate)
	}
	if len(item.Seasons) != 2 {
		t.Fatalf("expected 2 seasons, got %d", len(item.Seasons))
	}
	if item.Seasons[0].SeasonNumber != 1 || item.Seasons[0].EpisodeCount != 2 {
		t.Fatalf("unexpected season 1: %+v", item.Seasons[0])
	}
	names := []string{item.Seasons[0].Episodes[0].Name, item.Seasons[0].Episodes[1].Name}
	wantNames := []string{"Pilot", "Cat's in the Bag..."}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("expected %v, got %v", wantNames, names)
	}
	if item.Seasons[1].SeasonNumber != 2 {
		t.Fatalf("expected season 2, got %d", item.Seasons[1].SeasonNumber)
	}
}

func TestTVMazeFilterAndRankAiringToday(t *testing.T) {
	entries := []tvmazeScheduleEntry{
		{
			Show: &tvmazeShow{
				ID:     1,
				Name:   "Evening News",
				Type:   "News",
				Weight: 99,
				Image: &struct {
					Medium   string `json:"medium"`
					Original string `json:"original"`
				}{Original: "https://img/1.jpg"},
				Externals: &struct {
					IMDb string `json:"imdb"`
				}{IMDb: "tt0000001"},
			},
		},
		{
			Show: &tvmazeShow{
				ID:      2,
				Name:    "Severance",
				Type:    "Scripted",
				Weight:  98,
				Summary: "<p>Mark leads a team of office workers.</p>",
				Image: &struct {
					Medium   string `json:"medium"`
					Original string `json:"original"`
				}{Original: "https://img/2.jpg"},
				Externals: &struct {
					IMDb string `json:"imdb"`
				}{IMDb: "tt11280740"},
			},
		},
	}

	got := FilterAndRankAiringToday(entries, 30)
	if len(got) != 1 {
		t.Fatalf("expected 1 scripted show (filtering News), got %d: %+v", len(got), got)
	}
	if got[0].ImdbID != "tt11280740" || got[0].Overview != "Mark leads a team of office workers." {
		t.Fatalf("unexpected item: %+v", got[0])
	}
	_ = types.MediaItem{}
}
