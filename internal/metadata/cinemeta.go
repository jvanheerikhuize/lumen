package metadata

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

const (
	cinemetaURL      = "https://v3-cinemeta.strem.io"
	CinemetaPageSize = 50
)

var CinemetaGenres = []string{
	"Action", "Adventure", "Animation", "Biography", "Comedy", "Crime", "Documentary",
	"Drama", "Family", "Fantasy", "History", "Horror", "Mystery", "Romance",
	"Sci-Fi", "Sport", "Thriller", "War", "Western",
}

var (
	hoursRe = regexp.MustCompile(`(?i)(\d+)\s*h`)
	minsRe  = regexp.MustCompile(`(?i)(\d+)\s*min`)
)

// CinemetaVideo represents an episode entry in a Cinemeta series.
type CinemetaVideo struct {
	Season     any    `json:"season"`
	Episode    any    `json:"episode"`
	Number     any    `json:"number"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Overview   string `json:"overview"`
	Desc       string `json:"description"`
	Thumbnail  string `json:"thumbnail"`
	Released   string `json:"released"`
	FirstAired string `json:"firstAired"`
	Rating     any    `json:"rating"`
}

// CinemetaTrailer represents a trailer entry in Cinemeta metadata.
type CinemetaTrailer struct {
	Source string `json:"source"`
}

// CinemetaMeta represents the raw Cinemeta metadata object.
type CinemetaMeta struct {
	ID          string            `json:"id"`
	ImdbID      string            `json:"imdb_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Poster      string            `json:"poster"`
	Background  string            `json:"background"`
	Released    string            `json:"released"`
	Year        any               `json:"year"`
	MoviedbID   int               `json:"moviedb_id"`
	ImdbRating  any               `json:"imdbRating"`
	Genres      []string          `json:"genres"`
	Genre       []string          `json:"genre"`
	Runtime     string            `json:"runtime"`
	Trailers    []CinemetaTrailer `json:"trailers"`
	Videos      []CinemetaVideo   `json:"videos"`
}

func toStremioType(mediaType string) string {
	if mediaType == "tv" {
		return "series"
	}
	return "movie"
}

func parseRuntime(runtime string) *int {
	if runtime == "" {
		return nil
	}
	total := 0
	if m := hoursRe.FindStringSubmatch(runtime); len(m) == 2 {
		if h, err := strconv.Atoi(m[1]); err == nil {
			total += h * 60
		}
	}
	if m := minsRe.FindStringSubmatch(runtime); len(m) == 2 {
		if mins, err := strconv.Atoi(m[1]); err == nil {
			total += mins
		}
	}
	if total <= 0 {
		return nil
	}
	return &total
}

func toIsoDate(val string) string {
	if val == "" {
		return ""
	}
	return strings.Split(val, "T")[0]
}

func anyToInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	}
	return 0
}

func anyToFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

func buildSeasons(videos []CinemetaVideo) []types.Season {
	bySeason := make(map[int]*types.Season)
	for _, v := range videos {
		seasonNumber := anyToInt(v.Season)
		episodeNumber := anyToInt(v.Episode)
		if episodeNumber == 0 {
			episodeNumber = anyToInt(v.Number)
		}
		if seasonNumber <= 0 || episodeNumber <= 0 {
			continue
		}
		s, ok := bySeason[seasonNumber]
		if !ok {
			s = &types.Season{
				SeasonNumber: seasonNumber,
				Name:         fmt.Sprintf("Season %d", seasonNumber),
				Episodes:     []types.Episode{},
			}
			bySeason[seasonNumber] = s
		}
		name := v.Name
		if name == "" {
			name = v.Title
		}
		if name == "" {
			name = fmt.Sprintf("Episode %d", episodeNumber)
		}
		overview := v.Overview
		if overview == "" {
			overview = v.Desc
		}
		airDate := toIsoDate(v.Released)
		if airDate == "" {
			airDate = toIsoDate(v.FirstAired)
		}
		var voteAvg *float64
		if r := anyToFloat(v.Rating); r > 0 {
			voteAvg = &r
		}
		s.Episodes = append(s.Episodes, types.Episode{
			ID:            seasonNumber*10000 + episodeNumber,
			EpisodeNumber: episodeNumber,
			SeasonNumber:  seasonNumber,
			Name:          name,
			Overview:      overview,
			StillPath:     types.StringPtr(v.Thumbnail),
			AirDate:       airDate,
			VoteAverage:   voteAvg,
		})
	}

	seasons := make([]types.Season, 0, len(bySeason))
	for _, s := range bySeason {
		slices.SortFunc(s.Episodes, func(a, b types.Episode) int {
			return cmp.Compare(a.EpisodeNumber, b.EpisodeNumber)
		})
		s.EpisodeCount = len(s.Episodes)
		seasons = append(seasons, *s)
	}
	slices.SortFunc(seasons, func(a, b types.Season) int {
		return cmp.Compare(a.SeasonNumber, b.SeasonNumber)
	})
	return seasons
}

// CinemetaToMediaItem converts a CinemetaMeta object into a canonical types.MediaItem.
func CinemetaToMediaItem(meta CinemetaMeta, mediaType string) types.MediaItem {
	imdbID := meta.ImdbID
	if imdbID == "" {
		imdbID = meta.ID
	}
	released := toIsoDate(meta.Released)
	if released == "" && meta.Year != nil {
		yStr := fmt.Sprintf("%v", meta.Year)
		if len(yStr) >= 4 {
			released = yStr[:4] + "-01-01"
		}
	}

	poster := meta.Poster
	if poster == "" && imdbID != "" {
		poster = fmt.Sprintf("https://images.metahub.space/poster/medium/%s/img", imdbID)
	}
	backdrop := meta.Background
	if backdrop == "" && imdbID != "" {
		backdrop = fmt.Sprintf("https://images.metahub.space/background/medium/%s/img", imdbID)
	}

	var tmdbID *int
	if meta.MoviedbID > 0 {
		id := meta.MoviedbID
		tmdbID = &id
	}

	genres := meta.Genres
	if len(genres) == 0 {
		genres = meta.Genre
	}
	if genres == nil {
		genres = []string{}
	}

	var trailerURL string
	if len(meta.Trailers) > 0 && meta.Trailers[0].Source != "" {
		trailerURL = "https://www.youtube.com/watch?v=" + meta.Trailers[0].Source
	}

	var releaseDate, firstAirDate string
	if mediaType == "movie" {
		releaseDate = released
	} else {
		firstAirDate = released
	}

	var seasons []types.Season
	if mediaType == "tv" && meta.Videos != nil {
		seasons = buildSeasons(meta.Videos)
	}

	return types.MediaItem{
		ID:           imdbID,
		TmdbID:       tmdbID,
		ImdbID:       imdbID,
		Title:        meta.Name,
		Overview:     meta.Description,
		PosterPath:   types.StringPtr(poster),
		BackdropPath: types.StringPtr(backdrop),
		MediaType:    mediaType,
		ReleaseDate:  releaseDate,
		FirstAirDate: firstAirDate,
		VoteAverage:  anyToFloat(meta.ImdbRating),
		Genres:       genres,
		Duration:     parseRuntime(meta.Runtime),
		TrailerURL:   trailerURL,
		Seasons:      seasons,
	}
}

// CinemetaCatalogOptions configures Cinemeta GetCatalog.
type CinemetaCatalogOptions struct {
	Catalog string
	Genre   string
	Skip    int
}

// GetCinemetaCatalog fetches a single page of a Cinemeta catalog.
func GetCinemetaCatalog(ctx context.Context, mediaType string, opts CinemetaCatalogOptions) []types.MediaItem {
	cat := opts.Catalog
	if cat == "" {
		cat = "top"
	}
	var extras []string
	if opts.Genre != "" {
		extras = append(extras, "genre="+url.QueryEscape(opts.Genre))
	}
	if opts.Skip > 0 {
		extras = append(extras, fmt.Sprintf("skip=%d", opts.Skip))
	}
	extraPath := ""
	if len(extras) > 0 {
		extraPath = "/" + strings.Join(extras, "&")
	}
	targetURL := fmt.Sprintf("%s/catalog/%s/%s%s.json", cinemetaURL, toStremioType(mediaType), cat, extraPath)

	data, err := CachedJSON[struct {
		Metas []CinemetaMeta `json:"metas"`
	}](ctx, targetURL, 10*time.Second)
	if err != nil || data == nil {
		return []types.MediaItem{}
	}

	items := make([]types.MediaItem, 0, len(data.Metas))
	for _, m := range data.Metas {
		if m.Name != "" && (m.ImdbID != "" || m.ID != "") {
			items = append(items, CinemetaToMediaItem(m, mediaType))
		}
	}
	return items
}

// GetCinemetaHighlyRated fetches popular pages and filters/sorts by IMDb rating >= minRating.
func GetCinemetaHighlyRated(ctx context.Context, mediaType string, minRating float64, pages int) []types.MediaItem {
	if minRating <= 0 {
		minRating = 7.5
	}
	if pages <= 0 {
		pages = 4
	}
	results := make([][]types.MediaItem, pages)
	var wg sync.WaitGroup
	wg.Add(pages)
	for i := 0; i < pages; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = GetCinemetaCatalog(ctx, mediaType, CinemetaCatalogOptions{
				Catalog: "top",
				Skip:    idx * CinemetaPageSize,
			})
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool)
	var out []types.MediaItem
	for _, page := range results {
		for _, item := range page {
			id := item.IDString()
			if item.VoteAverage < minRating || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, item)
		}
	}
	slices.SortFunc(out, func(a, b types.MediaItem) int {
		return cmp.Compare(b.VoteAverage, a.VoteAverage)
	})
	return out
}

// SearchCinemeta searches Cinemeta for movies and series concurrently and interleaves results.
func SearchCinemeta(ctx context.Context, query string) []types.MediaItem {
	q := url.QueryEscape(strings.TrimSpace(query))
	if q == "" {
		return []types.MediaItem{}
	}
	type metaResp struct {
		Metas []CinemetaMeta `json:"metas"`
	}
	var (
		movies, series *metaResp
		wg             sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		movies, _ = CachedJSON[metaResp](ctx, fmt.Sprintf("%s/catalog/movie/top/search=%s.json", cinemetaURL, q), 10*time.Second)
	}()
	go func() {
		defer wg.Done()
		series, _ = CachedJSON[metaResp](ctx, fmt.Sprintf("%s/catalog/series/top/search=%s.json", cinemetaURL, q), 10*time.Second)
	}()
	wg.Wait()

	var movieItems, seriesItems []types.MediaItem
	if movies != nil {
		for _, m := range movies.Metas {
			if m.Name != "" {
				movieItems = append(movieItems, CinemetaToMediaItem(m, "movie"))
			}
		}
	}
	if series != nil {
		for _, s := range series.Metas {
			if s.Name != "" {
				seriesItems = append(seriesItems, CinemetaToMediaItem(s, "tv"))
			}
		}
	}

	maxLen := max(len(movieItems), len(seriesItems))
	merged := make([]types.MediaItem, 0, len(movieItems)+len(seriesItems))
	for i := 0; i < maxLen; i++ {
		if i < len(movieItems) {
			merged = append(merged, movieItems[i])
		}
		if i < len(seriesItems) {
			merged = append(merged, seriesItems[i])
		}
	}
	return merged
}

// GetCinemetaMeta fetches full movie or series metadata (including all seasons and episodes) by IMDb ID.
func GetCinemetaMeta(ctx context.Context, imdbID string, mediaType string) *types.MediaItem {
	targetURL := fmt.Sprintf("%s/meta/%s/%s.json", cinemetaURL, toStremioType(mediaType), url.PathEscape(imdbID))
	data, err := CachedJSON[struct {
		Meta *CinemetaMeta `json:"meta"`
	}](ctx, targetURL, 10*time.Second)
	if err != nil || data == nil || data.Meta == nil || data.Meta.Name == "" {
		return nil
	}
	item := CinemetaToMediaItem(*data.Meta, mediaType)
	return &item
}
