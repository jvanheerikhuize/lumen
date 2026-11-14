package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

const (
	anilistEndpoint = "https://graphql.anilist.co"
	anilistTTL      = 30 * time.Minute
	AnilistPageSize = 50
)

var AnimeCatalogs = []string{"trending", "popular", "top", "season"}

var AnimeGenres = []string{
	"Action", "Adventure", "Comedy", "Drama", "Fantasy", "Horror", "Mecha", "Music", "Mystery",
	"Psychological", "Romance", "Sci-Fi", "Slice of Life", "Sports", "Supernatural", "Thriller",
}

var anilistSortMap = map[string]string{
	"trending": "TRENDING_DESC",
	"popular":  "POPULARITY_DESC",
	"top":      "SCORE_DESC",
	"season":   "POPULARITY_DESC",
}

const anilistMediaFields = `
  id
  title { romaji english }
  description(asHtml: false)
  coverImage { extraLarge large }
  bannerImage
  format
  status
  episodes
  duration
  averageScore
  genres
  startDate { year month day }
  nextAiringEpisode { episode airingAt }
`

var (
	brTagRe     = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe   = regexp.MustCompile(`<[^>]+>`)
	sourceTagRe = regexp.MustCompile(`(?i)\(Source:[^)]*\)`)
	multiNLRe   = regexp.MustCompile(`\n{3,}`)
	epTitleRe   = regexp.MustCompile(`(?i)^Episode\s+(\d+)\s*[-:]\s*(.+)$`)

	gqlMu    sync.Mutex
	gqlStore = make(map[string]gqlCacheItem)
)

type gqlCacheItem struct {
	expires time.Time
	raw     json.RawMessage
}

// AnilistMedia represents the raw AniList GraphQL Media object.
type AnilistMedia struct {
	ID    int `json:"id"`
	Title struct {
		Romaji  string `json:"romaji"`
		English string `json:"english"`
	} `json:"title"`
	Description string `json:"description"`
	CoverImage  struct {
		ExtraLarge string `json:"extraLarge"`
		Large      string `json:"large"`
	} `json:"coverImage"`
	BannerImage  string   `json:"bannerImage"`
	Format       string   `json:"format"`
	Status       string   `json:"status"`
	Episodes     int      `json:"episodes"`
	Duration     int      `json:"duration"`
	AverageScore float64  `json:"averageScore"`
	Genres       []string `json:"genres"`
	StartDate    struct {
		Year  int `json:"year"`
		Month int `json:"month"`
		Day   int `json:"day"`
	} `json:"startDate"`
	NextAiringEpisode *struct {
		Episode  int `json:"episode"`
		AiringAt int `json:"airingAt"`
	} `json:"nextAiringEpisode"`
	StreamingEpisodes []struct {
		Title string `json:"title"`
	} `json:"streamingEpisodes"`
}

func cleanDescription(text string) string {
	s := brTagRe.ReplaceAllString(text, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = sourceTagRe.ReplaceAllString(s, "")
	s = multiNLRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func formatIsoDate(year, month, day int) string {
	if year == 0 {
		return ""
	}
	if month <= 0 {
		month = 1
	}
	if day <= 0 {
		day = 1
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func airedEpisodes(m AnilistMedia) int {
	if m.NextAiringEpisode != nil && m.NextAiringEpisode.Episode > 0 {
		if m.NextAiringEpisode.Episode-1 > 0 {
			return m.NextAiringEpisode.Episode - 1
		}
		return 0
	}
	if m.Episodes > 0 {
		return m.Episodes
	}
	if m.Status == "RELEASING" {
		return 0
	}
	return 1
}

// AnilistToMediaItem converts an AnilistMedia struct into a canonical types.MediaItem.
func AnilistToMediaItem(m AnilistMedia) types.MediaItem {
	isMovie := m.Format == "MOVIE"
	date := formatIsoDate(m.StartDate.Year, m.StartDate.Month, m.StartDate.Day)

	title := m.Title.English
	if title == "" {
		title = m.Title.Romaji
	}
	if title == "" {
		title = "Untitled"
	}

	poster := m.CoverImage.ExtraLarge
	if poster == "" {
		poster = m.CoverImage.Large
	}

	genres := []string{"Anime"}
	for _, g := range m.Genres {
		if g != "Ecchi" && g != "Hentai" {
			genres = append(genres, g)
		}
	}

	var voteAvg float64
	if m.AverageScore > 0 {
		voteAvg = math.Round(m.AverageScore) / 10.0
	}

	mediaType := "tv"
	var releaseDate, firstAirDate string
	var duration *int
	if isMovie {
		mediaType = "movie"
		releaseDate = date
		if m.Duration > 0 {
			d := m.Duration
			duration = &d
		}
	} else {
		firstAirDate = date
	}

	return types.MediaItem{
		ID:            fmt.Sprintf("al-%d", m.ID),
		Title:         title,
		OriginalTitle: m.Title.Romaji,
		Overview:      cleanDescription(m.Description),
		PosterPath:    types.StringPtr(poster),
		BackdropPath:  types.StringPtr(m.BannerImage),
		MediaType:     mediaType,
		ReleaseDate:   releaseDate,
		FirstAirDate:  firstAirDate,
		VoteAverage:   voteAvg,
		Genres:        genres,
		Duration:      duration,
		Anime:         &types.AnimeRef{AnilistID: m.ID},
	}
}

func currentAnimeSeason(now time.Time) (string, int) {
	month := int(now.Month())
	year := now.Year()
	if month == 12 {
		return "WINTER", year + 1
	}
	if month <= 2 {
		return "WINTER", year
	}
	if month <= 5 {
		return "SPRING", year
	}
	if month <= 8 {
		return "SUMMER", year
	}
	return "FALL", year
}

func queryAnilist(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return nil, err
	}
	key := string(payload)

	gqlMu.Lock()
	if hit, ok := gqlStore[key]; ok && time.Now().Before(hit.expires) {
		raw := hit.raw
		gqlMu.Unlock()
		return raw, nil
	}
	gqlMu.Unlock()

	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, anilistEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		gqlMu.Lock()
		hit := gqlStore[key].raw
		gqlMu.Unlock()
		return hit, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		gqlMu.Lock()
		hit := gqlStore[key].raw
		gqlMu.Unlock()
		return hit, fmt.Errorf("anilist HTTP %d", resp.StatusCode)
	}

	var parsed struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	gqlMu.Lock()
	if len(gqlStore) > 300 {
		for k := range gqlStore {
			delete(gqlStore, k)
			break
		}
	}
	gqlStore[key] = gqlCacheItem{
		expires: time.Now().Add(anilistTTL),
		raw:     parsed.Data,
	}
	gqlMu.Unlock()

	return parsed.Data, nil
}

// AnimeCatalogOptions configures GetAnimeCatalog.
type AnimeCatalogOptions struct {
	Catalog string
	Genre   string
	Page    int
	PerPage int
}

// GetAnimeCatalog queries AniList for a paginated anime catalog.
func GetAnimeCatalog(ctx context.Context, opts AnimeCatalogOptions) ([]types.MediaItem, bool) {
	cat := opts.Catalog
	if _, ok := anilistSortMap[cat]; !ok {
		cat = "trending"
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = AnilistPageSize
	}

	vars := map[string]any{
		"page":    page,
		"perPage": perPage,
		"sort":    []string{anilistSortMap[cat]},
	}
	if opts.Genre != "" && slices.Contains(AnimeGenres, opts.Genre) {
		vars["genre"] = opts.Genre
	}
	if cat == "season" {
		season, year := currentAnimeSeason(time.Now())
		vars["season"] = season
		vars["seasonYear"] = year
	}

	query := fmt.Sprintf(`
    query ($page: Int, $perPage: Int, $sort: [MediaSort], $genre: String, $season: MediaSeason, $seasonYear: Int) {
      Page(page: $page, perPage: $perPage) {
        pageInfo { hasNextPage }
        media(type: ANIME, isAdult: false, sort: $sort, genre: $genre, season: $season, seasonYear: $seasonYear,
              format_in: [TV, TV_SHORT, MOVIE, ONA]) { %s }
      }
    }`, anilistMediaFields)

	raw, err := queryAnilist(ctx, query, vars)
	if err != nil || len(raw) == 0 {
		return []types.MediaItem{}, false
	}
	var data struct {
		Page struct {
			PageInfo struct {
				HasNextPage bool `json:"hasNextPage"`
			} `json:"pageInfo"`
			Media []AnilistMedia `json:"media"`
		} `json:"Page"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return []types.MediaItem{}, false
	}

	items := make([]types.MediaItem, 0, len(data.Page.Media))
	for _, m := range data.Page.Media {
		items = append(items, AnilistToMediaItem(m))
	}
	return items, data.Page.PageInfo.HasNextPage
}

// SearchAnime searches AniList for anime matching the query string.
func SearchAnime(ctx context.Context, search string, perPage int) []types.MediaItem {
	if perPage <= 0 {
		perPage = 10
	}
	query := fmt.Sprintf(`query ($search: String, $perPage: Int) {
      Page(perPage: $perPage) { media(type: ANIME, isAdult: false, search: $search, sort: SEARCH_MATCH) { %s } }
    }`, anilistMediaFields)
	raw, err := queryAnilist(ctx, query, map[string]any{"search": search, "perPage": perPage})
	if err != nil || len(raw) == 0 {
		return []types.MediaItem{}
	}
	var data struct {
		Page struct {
			Media []AnilistMedia `json:"media"`
		} `json:"Page"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return []types.MediaItem{}
	}
	items := make([]types.MediaItem, 0, len(data.Page.Media))
	for _, m := range data.Page.Media {
		items = append(items, AnilistToMediaItem(m))
	}
	return items
}

// GetAnimeDetails fetches full anime metadata, episodes, and cross-referenced Kitsu/IMDb IDs.
func GetAnimeDetails(ctx context.Context, anilistID int) *types.MediaItem {
	var (
		raw json.RawMessage
		ids AnimeIDs
		wg  sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		q := fmt.Sprintf(`query ($id: Int) { Media(id: $id, type: ANIME) { %s streamingEpisodes { title } } }`, anilistMediaFields)
		raw, _ = queryAnilist(ctx, q, map[string]any{"id": anilistID})
	}()
	go func() {
		defer wg.Done()
		ids, _ = GetAnimeIDs(ctx, anilistID)
	}()
	wg.Wait()

	if len(raw) == 0 {
		return nil
	}
	var data struct {
		Media *AnilistMedia `json:"Media"`
	}
	if err := json.Unmarshal(raw, &data); err != nil || data.Media == nil {
		return nil
	}

	item := AnilistToMediaItem(*data.Media)
	item.Anime = &types.AnimeRef{
		AnilistID: anilistID,
		KitsuID:   ids.KitsuID,
		ImdbID:    ids.ImdbID,
		Season:    ids.Season,
	}
	item.ImdbID = ""

	if item.MediaType == "tv" {
		count := airedEpisodes(*data.Media)
		titles := make(map[int]string)
		for _, se := range data.Media.StreamingEpisodes {
			if m := epTitleRe.FindStringSubmatch(se.Title); len(m) == 3 {
				if n, err := strconv.Atoi(m[1]); err == nil {
					titles[n] = strings.TrimSpace(m[2])
				}
			}
		}
		episodes := make([]types.Episode, 0, count)
		for i := 0; i < count; i++ {
			epNum := i + 1
			name := titles[epNum]
			if name == "" {
				name = fmt.Sprintf("Episode %d", epNum)
			}
			episodes = append(episodes, types.Episode{
				ID:            anilistID*10000 + epNum,
				EpisodeNumber: epNum,
				SeasonNumber:  1,
				Name:          name,
				Overview:      "",
				StillPath:     nil,
			})
		}
		item.Seasons = []types.Season{
			{
				SeasonNumber: 1,
				Name:         "Episodes",
				EpisodeCount: count,
				Episodes:     episodes,
			},
		}
	}

	return &item
}
