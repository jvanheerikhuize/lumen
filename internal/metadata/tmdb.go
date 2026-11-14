package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
	"github.com/jvanheerikhuize/lumen/internal/types"
)

const (
	tmdbBaseURL = "https://api.themoviedb.org/3"
	tmdbImgURL  = "https://image.tmdb.org/t/p"
)

// FeaturedHero is the curated fallback hero item.
var FeaturedHero = types.MediaItem{
	ID:           "featured-1",
	TmdbID:       types.IntPtr(693134),
	ImdbID:       "tt15239678",
	Title:        "Dune: Part Two",
	Overview:     "Paul Atreides unites with Chani and the Fremen while seeking revenge against the conspirators who destroyed his family. Facing a choice between the love of his life and the fate of the universe, he endeavors to prevent a terrible future.",
	PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1534447677768-be436bb09401?w=800&auto=format&fit=crop&q=80"),
	BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=1920&auto=format&fit=crop&q=80"),
	MediaType:    "movie",
	ReleaseDate:  "2024-03-01",
	VoteAverage:  8.8,
	VoteCount:    types.IntPtr(9400),
	Genres:       []string{"Sci-Fi", "Adventure", "Action", "Drama"},
	Duration:     types.IntPtr(166),
}

// SampleMediaCatalog is the curated fallback catalog when external APIs or keys are unavailable.
var SampleMediaCatalog = []types.MediaItem{
	{
		ID:           "movie-1",
		TmdbID:       types.IntPtr(693134),
		ImdbID:       "tt15239678",
		Title:        "Dune: Part Two",
		Overview:     "Paul Atreides unites with Chani and the Fremen while seeking revenge against the conspirators who destroyed his family.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1534447677768-be436bb09401?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2024-03-01",
		VoteAverage:  8.8,
		VoteCount:    types.IntPtr(9400),
		Genres:       []string{"Sci-Fi", "Adventure", "Action"},
		Duration:     types.IntPtr(166),
	},
	{
		ID:           "movie-2",
		TmdbID:       types.IntPtr(872585),
		ImdbID:       "tt15398776",
		Title:        "Oppenheimer",
		Overview:     "The story of J. Robert Oppenheimer’s role in the development of the atomic bomb during World War II.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1509198397868-475647b2a1e5?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1509198397868-475647b2a1e5?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2023-07-21",
		VoteAverage:  8.9,
		VoteCount:    types.IntPtr(12500),
		Genres:       []string{"Drama", "History", "Biography"},
		Duration:     types.IntPtr(180),
	},
	{
		ID:           "movie-3",
		TmdbID:       types.IntPtr(157336),
		ImdbID:       "tt0816692",
		Title:        "Interstellar",
		Overview:     "A team of explorers travel through a wormhole in space in an attempt to ensure humanity’s survival.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1451187580459-43490279c0fa?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1451187580459-43490279c0fa?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2014-11-05",
		VoteAverage:  9.1,
		VoteCount:    types.IntPtr(34000),
		Genres:       []string{"Sci-Fi", "Drama", "Adventure"},
		Duration:     types.IntPtr(169),
	},
	{
		ID:           "movie-4",
		TmdbID:       types.IntPtr(335984),
		ImdbID:       "tt1856101",
		Title:        "Blade Runner 2049",
		Overview:     "Young Blade Runner K’s discovery of a long-buried secret leads him to track down former Blade Runner Rick Deckard.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1578632767115-351597cf2477?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2017-10-06",
		VoteAverage:  8.7,
		VoteCount:    types.IntPtr(15800),
		Genres:       []string{"Sci-Fi", "Cyberpunk", "Mystery"},
		Duration:     types.IntPtr(164),
	},
	{
		ID:           "movie-5",
		TmdbID:       types.IntPtr(27205),
		ImdbID:       "tt1375666",
		Title:        "Inception",
		Overview:     "A thief who steals corporate secrets through the use of dream-sharing technology is given the inverse task of planting an idea.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2010-07-16",
		VoteAverage:  8.8,
		VoteCount:    types.IntPtr(36000),
		Genres:       []string{"Sci-Fi", "Action", "Thriller"},
		Duration:     types.IntPtr(148),
	},
	{
		ID:           "movie-6",
		TmdbID:       types.IntPtr(603),
		ImdbID:       "tt0133093",
		Title:        "The Matrix",
		Overview:     "A computer hacker learns from mysterious rebels about the true nature of his reality and his role in the war against its controllers.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "1999-03-31",
		VoteAverage:  8.7,
		VoteCount:    types.IntPtr(25000),
		Genres:       []string{"Sci-Fi", "Action", "Cyberpunk"},
		Duration:     types.IntPtr(136),
	},
	{
		ID:           "sample-cosmos",
		ImdbID:       "tt4957236",
		Title:        "Cosmos Laundromat",
		Overview:     "On a desolate island, a suicidal sheep named Franck meets a quirky salesman who offers him the gift of a lifetime.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1534447677768-be436bb09401?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2015-08-10",
		VoteAverage:  8.7,
		VoteCount:    types.IntPtr(1420),
		Genres:       []string{"Animation", "Sci-Fi", "Short"},
		Duration:     types.IntPtr(12),
	},
	{
		ID:           "sample-bbb",
		ImdbID:       "tt1254207",
		Title:        "Big Buck Bunny",
		Overview:     "A large and lovable rabbit deals with bullying forest creatures in this iconic open-source animated short film.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1579783900882-c0d3dad7b119?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1579783900882-c0d3dad7b119?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2008-04-10",
		VoteAverage:  7.9,
		VoteCount:    types.IntPtr(3200),
		Genres:       []string{"Animation", "Comedy", "Short"},
		Duration:     types.IntPtr(10),
	},
	{
		ID:           "sample-tos",
		ImdbID:       "tt2285752",
		Title:        "Tears of Steel",
		Overview:     "Set in a dystopian future Amsterdam, a group of scientists and soldiers attempt to rewrite history.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1536440136628-849c177e76a1?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1536440136628-849c177e76a1?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2012-09-26",
		VoteAverage:  8.2,
		VoteCount:    types.IntPtr(1850),
		Genres:       []string{"Sci-Fi", "Action", "VFX"},
		Duration:     types.IntPtr(12),
	},
	{
		ID:           "sample-sintel",
		ImdbID:       "tt1727587",
		Title:        "Sintel",
		Overview:     "A lonely young woman searches for a baby dragon she befriended and nursed back to health.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "movie",
		ReleaseDate:  "2010-09-27",
		VoteAverage:  8.4,
		VoteCount:    types.IntPtr(4200),
		Genres:       []string{"Animation", "Fantasy", "Adventure"},
		Duration:     types.IntPtr(15),
	},
	{
		ID:           "tv-1",
		TmdbID:       types.IntPtr(1396),
		ImdbID:       "tt0903747",
		Title:        "Breaking Bad",
		Overview:     "A chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine with a former student.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "tv",
		FirstAirDate: "2008-01-20",
		VoteAverage:  9.5,
		VoteCount:    types.IntPtr(22000),
		Genres:       []string{"Drama", "Crime", "Thriller"},
		Seasons: []types.Season{
			{
				SeasonNumber: 1,
				Name:         "Season 1",
				EpisodeCount: 7,
				Episodes: []types.Episode{
					{ID: 101, EpisodeNumber: 1, SeasonNumber: 1, Name: "Pilot", Overview: "Walter White learns he has terminal cancer and teams up with Jesse Pinkman.", StillPath: nil, VoteAverage: types.Float64Ptr(9.1)},
					{ID: 102, EpisodeNumber: 2, SeasonNumber: 1, Name: "Cat's in the Bag...", Overview: "Walt and Jesse attempt to dispose of two bodies in an unorthodox manner.", StillPath: nil, VoteAverage: types.Float64Ptr(8.8)},
					{ID: 103, EpisodeNumber: 3, SeasonNumber: 1, Name: "...And the Bag's in the River", Overview: "Walt wrestles with a fateful moral dilemma in Jesse's basement.", StillPath: nil, VoteAverage: types.Float64Ptr(8.9)},
				},
			},
		},
	},
	{
		ID:           "tv-2",
		TmdbID:       types.IntPtr(66732),
		ImdbID:       "tt4574334",
		Title:        "Stranger Things",
		Overview:     "When a young boy vanishes, a small town uncovers a mystery involving secret experiments, terrifying supernatural forces, and one strange little girl.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1509198397868-475647b2a1e5?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1509198397868-475647b2a1e5?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "tv",
		FirstAirDate: "2016-07-15",
		VoteAverage:  8.9,
		VoteCount:    types.IntPtr(18000),
		Genres:       []string{"Sci-Fi", "Mystery", "Drama"},
		Seasons: []types.Season{
			{
				SeasonNumber: 1,
				Name:         "Season 1",
				EpisodeCount: 8,
				Episodes: []types.Episode{
					{ID: 201, EpisodeNumber: 1, SeasonNumber: 1, Name: "The Vanishing of Will Byers", Overview: "A boy goes missing near a top secret lab.", StillPath: nil, VoteAverage: types.Float64Ptr(8.7)},
					{ID: 202, EpisodeNumber: 2, SeasonNumber: 1, Name: "The Weirdo on Maple Street", Overview: "The boys shelter a girl with supernatural telekinesis.", StillPath: nil, VoteAverage: types.Float64Ptr(8.9)},
				},
			},
		},
	},
	{
		ID:           "tv-3",
		TmdbID:       types.IntPtr(100088),
		ImdbID:       "tt3581920",
		Title:        "The Last of Us",
		Overview:     "Twenty years after a fungal pandemic ravages the planet, a hardened survivor takes charge of a 14-year-old girl who may be humanity's last hope.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1536440136628-849c177e76a1?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1536440136628-849c177e76a1?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "tv",
		FirstAirDate: "2023-01-15",
		VoteAverage:  9.0,
		VoteCount:    types.IntPtr(14000),
		Genres:       []string{"Drama", "Sci-Fi", "Action"},
		Seasons: []types.Season{
			{
				SeasonNumber: 1,
				Name:         "Season 1",
				EpisodeCount: 9,
				Episodes: []types.Episode{
					{ID: 301, EpisodeNumber: 1, SeasonNumber: 1, Name: "When You're Lost in the Darkness", Overview: "Twenty years after a fungal outbreak destroys society, Joel takes on a smuggling mission.", StillPath: nil, VoteAverage: types.Float64Ptr(9.3)},
					{ID: 302, EpisodeNumber: 2, SeasonNumber: 1, Name: "Infected", Overview: "Joel, Tess, and Ellie navigate through a flooded and infected downtown Boston.", StillPath: nil, VoteAverage: types.Float64Ptr(9.2)},
				},
			},
		},
	},
	{
		ID:           "tv-4",
		TmdbID:       types.IntPtr(87108),
		ImdbID:       "tt8712704",
		Title:        "Chernobyl",
		Overview:     "In April 1986, an explosion at the Chernobyl nuclear power plant in the USSR becomes one of the world's worst man-made catastrophes.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1578632767115-351597cf2477?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "tv",
		FirstAirDate: "2019-05-06",
		VoteAverage:  9.4,
		VoteCount:    types.IntPtr(16000),
		Genres:       []string{"Drama", "History", "Thriller"},
		Seasons: []types.Season{
			{
				SeasonNumber: 1,
				Name:         "Miniseries",
				EpisodeCount: 5,
				Episodes: []types.Episode{
					{ID: 401, EpisodeNumber: 1, SeasonNumber: 1, Name: "1:23:45", Overview: "Plant workers and firefighters desperately attempt to control the catastrophic core explosion.", StillPath: nil, VoteAverage: types.Float64Ptr(9.5)},
					{ID: 402, EpisodeNumber: 2, SeasonNumber: 1, Name: "Please Remain Calm", Overview: "With millions at risk, nuclear physicist Ulana Khomyuk warns of a second imminent explosion.", StillPath: nil, VoteAverage: types.Float64Ptr(9.6)},
				},
			},
		},
	},
	{
		ID:           "tv-5",
		TmdbID:       types.IntPtr(60574),
		ImdbID:       "tt2575988",
		Title:        "Silicon Valley",
		Overview:     "Follows the struggle of Richard Hendricks, a Silicon Valley engineer trying to build his compression algorithm company Pied Piper.",
		PosterPath:   types.StringPtr("https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?w=800&auto=format&fit=crop&q=80"),
		BackdropPath: types.StringPtr("https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?w=1920&auto=format&fit=crop&q=80"),
		MediaType:    "tv",
		FirstAirDate: "2014-04-06",
		VoteAverage:  8.6,
		VoteCount:    types.IntPtr(7800),
		Genres:       []string{"Comedy", "Tech"},
		Seasons: []types.Season{
			{
				SeasonNumber: 1,
				Name:         "Season 1",
				EpisodeCount: 8,
				Episodes: []types.Episode{
					{ID: 501, EpisodeNumber: 1, SeasonNumber: 1, Name: "Minimum Viable Product", Overview: "Richard develops a revolutionary compression algorithm.", StillPath: nil, VoteAverage: types.Float64Ptr(8.8)},
				},
			},
		},
	},
}

type tmdbListItem struct {
	ID            int     `json:"id"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	PosterPath    *string `json:"poster_path"`
	BackdropPath  *string `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
}

type tmdbDetails struct {
	tmdbListItem
	IMDbID      string `json:"imdb_id"`
	ExternalIDs struct {
		IMDbID string `json:"imdb_id"`
	} `json:"external_ids"`
	Videos struct {
		Results []struct {
			Type string `json:"type"`
			Site string `json:"site"`
			Key  string `json:"key"`
		} `json:"results"`
	} `json:"videos"`
	Genres []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Runtime        int   `json:"runtime"`
	EpisodeRunTime []int `json:"episode_run_time"`
}

// TMDBService provides TMDB API access with keyless fallback to Cinemeta/AniList/SampleCatalog.
type TMDBService struct {
	apiKey string
}

// NewTMDBService constructs a TMDBService using the provided API key or TMDB_API_KEY env var.
func NewTMDBService(apiKey string) *TMDBService {
	if apiKey == "" {
		apiKey = os.Getenv("TMDB_API_KEY")
	}
	return &TMDBService{apiKey: apiKey}
}

var DefaultTMDBService = NewTMDBService("")

var (
	reAnilistID = regexp.MustCompile(`^al-(\d+)$`)
	reIMDbID    = regexp.MustCompile(`^tt\d+$`)
)

// GetTrending fetches trending items from TMDB or returns SampleMediaCatalog if keyless.
func (s *TMDBService) GetTrending(ctx context.Context, mediaType, timeWindow string) []types.MediaItem {
	if mediaType == "" {
		mediaType = "all"
	}
	if timeWindow == "" {
		timeWindow = "week"
	}
	key := s.apiKey
	if key == "" {
		key = os.Getenv("TMDB_API_KEY")
	}
	if key == "" {
		return SampleMediaCatalog
	}

	reqURL := fmt.Sprintf("%s/trending/%s/%s?api_key=%s", tmdbBaseURL, mediaType, timeWindow, url.QueryEscape(key))
	res, err := netguard.GuardedFetch(ctx, reqURL, netguard.FetchOptions{
		Timeout:      10 * time.Second,
		AllowPrivate: false,
	})
	if err != nil {
		return SampleMediaCatalog
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return SampleMediaCatalog
	}
	var data struct {
		Results []tmdbListItem `json:"results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return SampleMediaCatalog
	}
	return transformTMDBList(data.Results)
}

// Search searches TMDB when configured, or falls back to Cinemeta search.
func (s *TMDBService) Search(ctx context.Context, query string) []types.MediaItem {
	if strings.TrimSpace(query) == "" {
		return []types.MediaItem{}
	}
	key := s.apiKey
	if key == "" {
		key = os.Getenv("TMDB_API_KEY")
	}
	if key == "" {
		return SearchCinemeta(ctx, query)
	}

	reqURL := fmt.Sprintf("%s/search/multi?api_key=%s&query=%s", tmdbBaseURL, url.QueryEscape(key), url.QueryEscape(query))
	res, err := netguard.GuardedFetch(ctx, reqURL, netguard.FetchOptions{
		Timeout:      10 * time.Second,
		AllowPrivate: false,
	})
	if err != nil {
		return []types.MediaItem{}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return []types.MediaItem{}
	}
	var data struct {
		Results []tmdbListItem `json:"results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return []types.MediaItem{}
	}
	var filtered []tmdbListItem
	for _, item := range data.Results {
		if item.MediaType == "movie" || item.MediaType == "tv" {
			filtered = append(filtered, item)
		}
	}
	return transformTMDBList(filtered)
}

// GetDetails resolves media details across SampleMediaCatalog, AniList ("al-*"), Cinemeta ("tt*"), or TMDB.
func (s *TMDBService) GetDetails(ctx context.Context, id string, mediaType string) *types.MediaItem {
	if mediaType == "" {
		mediaType = "movie"
	}
	key := s.apiKey
	if key == "" {
		key = os.Getenv("TMDB_API_KEY")
	}

	for _, m := range SampleMediaCatalog {
		if m.IDString() == id || (key == "" && m.TmdbID != nil && strconv.Itoa(*m.TmdbID) == id) {
			cp := m
			return &cp
		}
	}

	if m := reAnilistID.FindStringSubmatch(id); len(m) == 2 {
		alID, err := strconv.Atoi(m[1])
		if err == nil {
			return GetAnimeDetails(ctx, alID)
		}
	}

	if reIMDbID.MatchString(id) {
		return GetCinemetaMeta(ctx, id, mediaType)
	}

	if key == "" {
		return nil
	}

	reqURL := fmt.Sprintf("%s/%s/%s?api_key=%s&append_to_response=videos,credits,external_ids", tmdbBaseURL, mediaType, url.PathEscape(id), url.QueryEscape(key))
	res, err := netguard.GuardedFetch(ctx, reqURL, netguard.FetchOptions{
		Timeout:      10 * time.Second,
		AllowPrivate: false,
	})
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil
	}
	var data tmdbDetails
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil
	}

	trailerURL := ""
	if len(data.Videos.Results) > 0 {
		chosenKey := ""
		for _, v := range data.Videos.Results {
			if v.Type == "Trailer" && v.Site == "YouTube" && v.Key != "" {
				chosenKey = v.Key
				break
			}
		}
		if chosenKey == "" && data.Videos.Results[0].Key != "" {
			chosenKey = data.Videos.Results[0].Key
		}
		if chosenKey != "" {
			trailerURL = "https://www.youtube.com/watch?v=" + chosenKey
		}
	}

	imdbID := data.ExternalIDs.IMDbID
	if imdbID == "" {
		imdbID = data.IMDbID
	}
	var seasons []types.Season
	if mediaType == "tv" && imdbID != "" {
		cm := GetCinemetaMeta(ctx, imdbID, "tv")
		if cm != nil {
			seasons = cm.Seasons
		}
	}

	title := data.Title
	if title == "" {
		title = data.Name
	}
	origTitle := data.OriginalTitle
	if origTitle == "" {
		origTitle = data.OriginalName
	}

	var posterPath, backdropPath *string
	if data.PosterPath != nil && *data.PosterPath != "" {
		p := tmdbImgURL + "/w780" + *data.PosterPath
		posterPath = &p
	}
	if data.BackdropPath != nil && *data.BackdropPath != "" {
		b := tmdbImgURL + "/original" + *data.BackdropPath
		backdropPath = &b
	}

	genres := make([]string, 0, len(data.Genres))
	for _, g := range data.Genres {
		genres = append(genres, g.Name)
	}

	var duration *int
	if data.Runtime > 0 {
		duration = types.IntPtr(data.Runtime)
	} else if len(data.EpisodeRunTime) > 0 && data.EpisodeRunTime[0] > 0 {
		duration = types.IntPtr(data.EpisodeRunTime[0])
	}

	return &types.MediaItem{
		ID:            data.ID,
		TmdbID:        types.IntPtr(data.ID),
		ImdbID:        imdbID,
		Title:         title,
		OriginalTitle: origTitle,
		Overview:      data.Overview,
		PosterPath:    posterPath,
		BackdropPath:  backdropPath,
		MediaType:     mediaType,
		ReleaseDate:   data.ReleaseDate,
		FirstAirDate:  data.FirstAirDate,
		VoteAverage:   data.VoteAverage,
		VoteCount:     types.IntPtr(data.VoteCount),
		Genres:        genres,
		Duration:      duration,
		TrailerURL:    trailerURL,
		Seasons:       seasons,
	}
}

func transformTMDBList(results []tmdbListItem) []types.MediaItem {
	items := make([]types.MediaItem, 0, len(results))
	for _, item := range results {
		title := item.Title
		if title == "" {
			title = item.Name
		}
		origTitle := item.OriginalTitle
		if origTitle == "" {
			origTitle = item.OriginalName
		}
		var posterPath, backdropPath *string
		if item.PosterPath != nil && *item.PosterPath != "" {
			p := tmdbImgURL + "/w500" + *item.PosterPath
			posterPath = &p
		}
		if item.BackdropPath != nil && *item.BackdropPath != "" {
			b := tmdbImgURL + "/w1280" + *item.BackdropPath
			backdropPath = &b
		}
		mediaType := "movie"
		if item.MediaType == "tv" || item.FirstAirDate != "" {
			mediaType = "tv"
		}
		vote := math.Round(item.VoteAverage*10) / 10
		items = append(items, types.MediaItem{
			ID:            item.ID,
			TmdbID:        types.IntPtr(item.ID),
			Title:         title,
			OriginalTitle: origTitle,
			Overview:      item.Overview,
			PosterPath:    posterPath,
			BackdropPath:  backdropPath,
			MediaType:     mediaType,
			ReleaseDate:   item.ReleaseDate,
			FirstAirDate:  item.FirstAirDate,
			VoteAverage:   vote,
			VoteCount:     types.IntPtr(item.VoteCount),
			Genres:        []string{},
		})
	}
	return items
}
