package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/live"
	"github.com/jvanheerikhuize/lumen/internal/metadata"
	"github.com/jvanheerikhuize/lumen/internal/netguard"
	"github.com/jvanheerikhuize/lumen/internal/scrapers"
	"github.com/jvanheerikhuize/lumen/internal/state"
	"github.com/jvanheerikhuize/lumen/internal/streaming"
	"github.com/jvanheerikhuize/lumen/internal/subtitles"
	"github.com/jvanheerikhuize/lumen/internal/types"
)

// Server encapsulates the HTTP mux, embedded SPA filesystem, SQLite state store, and BitTorrent engine manager.
type Server struct {
	mux        *http.ServeMux
	spaFS      fs.FS
	stateStore *state.Store
	torrents   *streaming.TorrentEngineManager
	tmdb       *metadata.TMDBService

	ffmpegOnce sync.Once
	ffmpegOK   bool
}

// Config configures a new Lumen HTTP Server.
type Config struct {
	SPAFS          fs.FS
	StateStore     *state.Store
	TorrentManager *streaming.TorrentEngineManager
	TMDBService    *metadata.TMDBService
}

// New constructs a new Server and registers all API and SPA routes.
func New(cfg Config) (*Server, error) {
	store := cfg.StateStore
	if store == nil {
		var err error
		store, err = state.NewStore("")
		if err != nil {
			return nil, err
		}
	}
	tm := cfg.TorrentManager
	if tm == nil {
		tm = streaming.DefaultTorrentManager
	}
	tmdb := cfg.TMDBService
	if tmdb == nil {
		tmdb = metadata.DefaultTMDBService
	}

	s := &Server{
		mux:        http.NewServeMux(),
		spaFS:      cfg.SPAFS,
		stateStore: store,
		torrents:   tm,
		tmdb:       tmdb,
	}
	s.registerRoutes()
	return s, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/media/trending", s.handleTrending)
	s.mux.HandleFunc("GET /api/media/{id}", s.handleMediaDetails)
	s.mux.HandleFunc("GET /api/catalog", s.handleCatalog)
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/tvmaze/show", s.handleTVMazeShow)

	s.mux.HandleFunc("GET /api/stream", s.handleStream)
	s.mux.HandleFunc("HEAD /api/stream", s.handleStream)
	s.mux.HandleFunc("GET /api/stream/prepare", s.handleStreamPrepare)
	s.mux.HandleFunc("POST /api/stream/release", s.handleStreamRelease)

	s.mux.HandleFunc("GET /api/playback/probe", s.handlePlaybackProbe)
	s.mux.HandleFunc("GET /api/playback/transcode", s.handlePlaybackTranscode)

	s.mux.HandleFunc("GET /api/subtitles", s.handleSubtitles)
	s.mux.HandleFunc("GET /api/subtitles/vtt", s.handleSubtitlesVTT)
	s.mux.HandleFunc("GET /api/subtitles/embedded", s.handleSubtitlesEmbedded)

	s.mux.HandleFunc("GET /api/live", s.handleLive)
	s.mux.HandleFunc("GET /api/live/proxy", s.handleLiveProxy)
	s.mux.HandleFunc("POST /api/live/status", s.handleLiveStatus)

	s.mux.HandleFunc("GET /api/state", s.handleGetState)
	s.mux.HandleFunc("PUT /api/state", s.handlePutState)

	s.mux.HandleFunc("GET /api/integrations", s.handleIntegrations)
	s.mux.HandleFunc("POST /api/integrations/test", s.handleIntegrationsTest)

	if s.spaFS != nil {
		s.mux.HandleFunc("/", s.handleSPA)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) checkFfmpeg() bool {
	s.ffmpegOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := exec.CommandContext(ctx, "ffmpeg", "-version").Run()
		s.ffmpegOK = err == nil
	})
	return s.ffmpegOK
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"ffmpeg":    s.checkFfmpeg(),
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

type genreRowSpec struct {
	genre     string
	mediaType string
	title     string
	subtitle  string
}

var trendingGenreRows = []genreRowSpec{
	{genre: "Action", mediaType: "movie", title: "Action & Adrenaline", subtitle: "Popular action movies"},
	{genre: "Comedy", mediaType: "movie", title: "Comedy Picks", subtitle: "Popular comedies"},
	{genre: "Horror", mediaType: "movie", title: "Horror & Thrillers", subtitle: "Popular horror movies"},
	{genre: "Sci-Fi", mediaType: "movie", title: "Science Fiction", subtitle: "Popular sci-fi movies"},
	{genre: "Animation", mediaType: "movie", title: "Animated Features", subtitle: "Popular animated movies"},
	{genre: "Documentary", mediaType: "movie", title: "Documentaries", subtitle: "Popular documentaries"},
	{genre: "Crime", mediaType: "tv", title: "Crime Series", subtitle: "Popular crime and detective shows"},
	{genre: "Comedy", mediaType: "tv", title: "Comedy Series", subtitle: "Popular sitcoms and comedy shows"},
}

const rowLimit = 40

func interleaveMedia(a, b []types.MediaItem) []types.MediaItem {
	maxLen := max(len(a), len(b))
	out := make([]types.MediaItem, 0, len(a)+len(b))
	for i := 0; i < maxLen; i++ {
		if i < len(a) {
			out = append(out, a[i])
		}
		if i < len(b) {
			out = append(out, b[i])
		}
	}
	return out
}

func truncateMedia(items []types.MediaItem, limit int) []types.MediaItem {
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Server) handleTrending(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var (
		airingToday   []types.MediaItem
		animeTrending []types.MediaItem
		tmdbTrending  []types.MediaItem
		popularMovies []types.MediaItem
		popularSeries []types.MediaItem
		topMovies     []types.MediaItem
		topSeries     []types.MediaItem
		genreResults  = make([][]types.MediaItem, len(trendingGenreRows))
		wg            sync.WaitGroup
	)

	wg.Add(7 + len(trendingGenreRows))
	go func() {
		defer wg.Done()
		airingToday = metadata.GetAiringToday(ctx, 30)
	}()
	go func() {
		defer wg.Done()
		animeTrending, _ = metadata.GetAnimeCatalog(ctx, metadata.AnimeCatalogOptions{Catalog: "trending", PerPage: 30})
	}()
	go func() {
		defer wg.Done()
		tmdbTrending = s.tmdb.GetTrending(ctx, "all", "week")
	}()
	go func() {
		defer wg.Done()
		popularMovies = metadata.GetCinemetaCatalog(ctx, "movie", metadata.CinemetaCatalogOptions{Catalog: "top"})
	}()
	go func() {
		defer wg.Done()
		popularSeries = metadata.GetCinemetaCatalog(ctx, "tv", metadata.CinemetaCatalogOptions{Catalog: "top"})
	}()
	go func() {
		defer wg.Done()
		topMovies = metadata.GetCinemetaHighlyRated(ctx, "movie", 7.5, 4)
	}()
	go func() {
		defer wg.Done()
		topSeries = metadata.GetCinemetaHighlyRated(ctx, "tv", 7.5, 4)
	}()
	for i, gr := range trendingGenreRows {
		go func(idx int, spec genreRowSpec) {
			defer wg.Done()
			genreResults[idx] = metadata.GetCinemetaCatalog(ctx, spec.mediaType, metadata.CinemetaCatalogOptions{
				Catalog: "top",
				Genre:   spec.genre,
			})
		}(i, gr)
	}
	wg.Wait()

	hasTmdb := os.Getenv("TMDB_API_KEY") != "" && len(tmdbTrending) > 0
	var fallbackMovies, fallbackSeries, openMovies []types.MediaItem
	for _, m := range metadata.SampleMediaCatalog {
		if m.MediaType == "movie" {
			fallbackMovies = append(fallbackMovies, m)
		} else if m.MediaType == "tv" {
			fallbackSeries = append(fallbackSeries, m)
		}
		if strings.HasPrefix(m.IDString(), "sample-") {
			openMovies = append(openMovies, m)
		}
	}

	movies := popularMovies
	if len(movies) == 0 {
		movies = fallbackMovies
	}
	series := popularSeries
	if len(series) == 0 {
		series = fallbackSeries
	}

	trendingItems := truncateMedia(interleaveMedia(movies, series), rowLimit)
	trendingSub := "What people are watching right now"
	if hasTmdb {
		trendingItems = tmdbTrending
		trendingSub = "Trending this week on TMDB"
	}

	candidateRows := []types.CatalogRow{
		{Key: "trending", Title: "Trending now", Subtitle: trendingSub, Items: trendingItems},
		{Key: "popular-movies", Title: "Popular Movies", Subtitle: "The most-watched films right now", Items: truncateMedia(movies, rowLimit)},
		{Key: "popular-series", Title: "Popular Series", Subtitle: "The most-watched shows right now", Items: truncateMedia(series, rowLimit)},
		{Key: "airing-today", Title: "New episodes today", Subtitle: "Series with a new episode out today", Items: airingToday},
		{Key: "trending-anime", Title: "Trending anime", Subtitle: "Popular on AniList this week", Items: animeTrending},
		{Key: "top-movies", Title: "Highly Rated Movies", Subtitle: "Best-reviewed popular films (IMDb 7.5+)", Items: truncateMedia(topMovies, rowLimit)},
		{Key: "top-series", Title: "Highly Rated Series", Subtitle: "Best-reviewed popular shows (IMDb 7.5+)", Items: truncateMedia(topSeries, rowLimit)},
	}
	for i, gr := range trendingGenreRows {
		candidateRows = append(candidateRows, types.CatalogRow{
			Key:      fmt.Sprintf("genre-%s-%s", gr.mediaType, strings.ToLower(gr.genre)),
			Title:    gr.title,
			Subtitle: gr.subtitle,
			Items:    truncateMedia(genreResults[i], rowLimit),
		})
	}
	candidateRows = append(candidateRows, types.CatalogRow{
		Key:      "open-movies",
		Title:    "Open Movies",
		Subtitle: "Blender Foundation originals with instant playback",
		Items:    openMovies,
	})

	var rows []types.CatalogRow
	for _, r := range candidateRows {
		if len(r.Items) > 0 {
			rows = append(rows, r)
		}
	}

	var heroPool []types.MediaItem
	for _, list := range [][]types.MediaItem{topMovies, movies, topSeries, series} {
		for _, m := range list {
			if m.BackdropPath != nil && *m.BackdropPath != "" && m.Overview != "" && m.ImdbID != "" {
				heroPool = append(heroPool, m)
			}
		}
	}
	featured := metadata.FeaturedHero
	if len(heroPool) > 0 {
		limit := min(20, len(heroPool))
		featured = heroPool[rand.IntN(limit)]
	}

	trendingOut := metadata.SampleMediaCatalog
	if len(rows) > 0 && len(rows[0].Items) > 0 {
		trendingOut = rows[0].Items
	}

	w.Header().Set("Cache-Control", "public, s-maxage=900, stale-while-revalidate=1800")
	writeJSON(w, http.StatusOK, map[string]any{
		"featured": featured,
		"rows":     rows,
		"trending": trendingOut,
		"movies":   movies,
		"series":   series,
	})
}

var (
	reValidIMDb = regexp.MustCompile(`(?i)^tt\d+$`)
	reFourDigit = regexp.MustCompile(`^\d{4}$`)
	reDigits    = regexp.MustCompile(`^\d+$`)
)

func (s *Server) handleMediaDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	q := r.URL.Query()
	mediaType := "movie"
	if q.Get("type") == "tv" {
		mediaType = "tv"
	}
	season, _ := strconv.Atoi(q.Get("season"))
	if season <= 0 {
		season = 1
	}
	episode, _ := strconv.Atoi(q.Get("episode"))
	if episode <= 0 {
		episode = 1
	}

	prowlarrURL := r.Header.Get("x-prowlarr-url")
	if prowlarrURL == "" {
		prowlarrURL = os.Getenv("PROWLARR_URL")
	}
	prowlarrKey := r.Header.Get("x-prowlarr-key")
	if prowlarrKey == "" {
		prowlarrKey = os.Getenv("PROWLARR_API_KEY")
	}
	jackettURL := r.Header.Get("x-jackett-url")
	if jackettURL == "" {
		jackettURL = os.Getenv("JACKETT_URL")
	}
	jackettKey := r.Header.Get("x-jackett-key")
	if jackettKey == "" {
		jackettKey = os.Getenv("JACKETT_API_KEY")
	}

	var customAddons []string
	if addonsParam := strings.TrimSpace(q.Get("addons")); addonsParam != "" {
		customAddons = strings.Split(addonsParam, ",")
	}
	addonScraper := scrapers.NewStremioAddonScraper(customAddons)

	var details *types.MediaItem
	var wgInit sync.WaitGroup
	wgInit.Add(2)
	go func() {
		defer wgInit.Done()
		details = s.tmdb.GetDetails(ctx, id, mediaType)
	}()
	go func() {
		defer wgInit.Done()
		_ = streaming.GetTrackers(ctx)
	}()
	wgInit.Wait()

	staticStreams := streaming.GetStreamsForMedia(id)
	if len(staticStreams) == 0 && details != nil && details.TmdbID != nil {
		staticStreams = streaming.GetStreamsForMedia(*details.TmdbID)
	}

	var kitsuID *int
	if details != nil && details.Anime != nil {
		kitsuID = details.Anime.KitsuID
	}
	rawIMDbParam := strings.TrimSpace(q.Get("imdbId"))
	validIMDbParam := ""
	if reValidIMDb.MatchString(rawIMDbParam) {
		validIMDbParam = rawIMDbParam
	}
	imdbID := ""
	if kitsuID == nil || *kitsuID == 0 {
		if validIMDbParam != "" {
			imdbID = validIMDbParam
		} else if details != nil {
			imdbID = details.ImdbID
		}
	}

	mediaTitle := "Media"
	if details != nil && details.Title != "" {
		mediaTitle = details.Title
	}

	var searchQuery string
	if details != nil && details.Anime != nil {
		if mediaType == "tv" {
			searchQuery = fmt.Sprintf("%s %02d", mediaTitle, episode)
		} else {
			searchQuery = mediaTitle
		}
	} else if mediaType == "tv" {
		searchQuery = fmt.Sprintf("%s S%02dE%02d", mediaTitle, season, episode)
	} else {
		year := ""
		if details != nil && details.ReleaseDate != "" {
			year = strings.Split(details.ReleaseDate, "-")[0]
		}
		searchQuery = strings.TrimSpace(mediaTitle + " " + year)
	}

	var (
		stremioStreams  []types.StreamSource
		prowlarrStreams []types.StreamSource
		jackettStreams  []types.StreamSource
		wgScrape        sync.WaitGroup
	)

	if imdbID != "" || (kitsuID != nil && *kitsuID > 0) {
		wgScrape.Add(1)
		go func() {
			defer wgScrape.Done()
			stremioStreams = addonScraper.FetchStreams(ctx, scrapers.StremioFetchParams{
				IMDbID:    imdbID,
				KitsuID:   kitsuID,
				TMDBID:    id,
				MediaType: mediaType,
				Season:    season,
				Episode:   episode,
				Title:     mediaTitle,
			})
		}()
	}
	if prowlarrURL != "" && prowlarrKey != "" {
		wgScrape.Add(1)
		go func() {
			defer wgScrape.Done()
			prowlarrStreams = scrapers.SearchProwlarr(ctx, scrapers.ProwlarrSearchParams{
				HostURL:   prowlarrURL,
				APIKey:    prowlarrKey,
				Query:     searchQuery,
				MediaType: mediaType,
			})
		}()
	}
	if jackettURL != "" && jackettKey != "" {
		wgScrape.Add(1)
		go func() {
			defer wgScrape.Done()
			jackettStreams = scrapers.SearchJackett(ctx, scrapers.JackettSearchParams{
				HostURL: jackettURL,
				APIKey:  jackettKey,
				Query:   searchQuery,
				IMDbID:  imdbID,
			})
		}()
	}
	wgScrape.Wait()

	if details == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Media not found"})
		return
	}

	allCandidates := make([]types.StreamSource, 0, len(staticStreams)+len(stremioStreams)+len(prowlarrStreams)+len(jackettStreams))
	allCandidates = append(allCandidates, staticStreams...)
	allCandidates = append(allCandidates, stremioStreams...)
	allCandidates = append(allCandidates, prowlarrStreams...)
	allCandidates = append(allCandidates, jackettStreams...)

	seen := make(map[string]bool)
	combined := make([]types.StreamSource, 0, len(allCandidates))
	for _, st := range allCandidates {
		key := ""
		if st.InfoHash != "" {
			key = strings.ToLower(st.InfoHash)
		} else if st.Magnet != "" {
			key = strings.ToLower(st.Magnet)
		} else if st.URL != "" {
			key = st.URL
		} else {
			key = st.ID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		combined = append(combined, st)
	}

	out := *details
	out.Streams = combined
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	skip, _ := strconv.Atoi(q.Get("skip"))
	if skip < 0 {
		skip = 0
	}

	if q.Get("type") == "anime" {
		cat := q.Get("catalog")
		if !slices.Contains(metadata.AnimeCatalogs, cat) {
			cat = "trending"
		}
		items, hasMore := metadata.GetAnimeCatalog(ctx, metadata.AnimeCatalogOptions{
			Catalog: cat,
			Genre:   q.Get("genre"),
			Page:    (skip / metadata.AnilistPageSize) + 1,
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"items":   items,
			"hasMore": hasMore,
			"genres":  metadata.AnimeGenres,
		})
		return
	}

	mediaType := "movie"
	if q.Get("type") == "tv" {
		mediaType = "tv"
	}
	catalog := q.Get("catalog")
	if catalog != "top" && catalog != "year" {
		catalog = "top"
	}
	genreParam := q.Get("genre")
	genre := ""
	if catalog == "year" {
		if reFourDigit.MatchString(genreParam) {
			genre = genreParam
		} else {
			genre = strconv.Itoa(time.Now().UTC().Year())
		}
	} else if slices.Contains(metadata.CinemetaGenres, genreParam) {
		genre = genreParam
	}

	items := metadata.GetCinemetaCatalog(ctx, mediaType, metadata.CinemetaCatalogOptions{
		Catalog: catalog,
		Genre:   genre,
		Skip:    skip,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"items":   items,
		"hasMore": float64(len(items)) >= float64(metadata.CinemetaPageSize)*0.8,
		"genres":  metadata.CinemetaGenres,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"results":  []types.MediaItem{},
			"channels": []types.LiveChannel{},
		})
		return
	}

	var (
		catalogResults []types.MediaItem
		animeResults   []types.MediaItem
		allChannels    []types.LiveChannel
		wg             sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		catalogResults = s.tmdb.Search(ctx, q)
	}()
	go func() {
		defer wg.Done()
		animeResults = metadata.SearchAnime(ctx, q, 10)
	}()
	go func() {
		defer wg.Done()
		allChannels = live.GetAllChannels(ctx)
	}()
	wg.Wait()

	seen := make(map[string]bool)
	results := make([]types.MediaItem, 0, len(catalogResults)+len(animeResults))
	maxLen := max(len(catalogResults), len(animeResults))
	for i := 0; i < maxLen; i++ {
		if i < len(catalogResults) {
			item := catalogResults[i]
			key := fmt.Sprintf("%s|%s", item.IDString(), strings.ToLower(strings.TrimSpace(item.Title)))
			if !seen[key] {
				seen[key] = true
				results = append(results, item)
			}
		}
		if i < len(animeResults) {
			item := animeResults[i]
			key := fmt.Sprintf("%s|%s", item.IDString(), strings.ToLower(strings.TrimSpace(item.Title)))
			if !seen[key] {
				seen[key] = true
				results = append(results, item)
			}
		}
	}

	qLower := strings.ToLower(q)
	var matchedChannels []types.LiveChannel
	for _, ch := range allChannels {
		if strings.Contains(strings.ToLower(ch.Name), qLower) ||
			strings.Contains(strings.ToLower(ch.Country), qLower) ||
			strings.Contains(strings.ToLower(ch.Category), qLower) {
			matchedChannels = append(matchedChannels, ch)
			if len(matchedChannels) >= 8 {
				break
			}
		}
	}
	if matchedChannels == nil {
		matchedChannels = []types.LiveChannel{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  results,
		"channels": matchedChannels,
	})
}

func (s *Server) handleTVMazeShow(w http.ResponseWriter, r *http.Request) {
	imdbID := strings.TrimSpace(r.URL.Query().Get("imdbId"))
	if !reValidIMDb.MatchString(imdbID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid imdbId"})
		return
	}
	status := metadata.GetShowStatus(r.Context(), imdbID)
	w.Header().Set("Cache-Control", "public, max-age=1800")
	if status == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	magnet := q.Get("magnet")
	var fileIdx *int
	if fiStr := q.Get("fileIndex"); fiStr != "" {
		if v, err := strconv.Atoi(fiStr); err == nil {
			fileIdx = &v
		}
	}
	s.torrents.ServeStream(w, r, magnet, fileIdx)
}

func (s *Server) handleStreamPrepare(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	magnet := q.Get("magnet")
	if magnet == "" || streaming.ExtractInfoHash(magnet) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing or invalid magnet parameter"})
		return
	}
	var fileIdx *int
	if fiStr := q.Get("fileIndex"); fiStr != "" {
		if v, err := strconv.Atoi(fiStr); err == nil {
			fileIdx = &v
		}
	}
	from, _ := strconv.ParseInt(q.Get("from"), 10, 64)
	if from < 0 {
		from = 0
	}
	status := s.torrents.GetBufferStatus(magnet, fileIdx, from)
	stats := s.torrents.GetStats(magnet)
	if stats != nil && stats.Files != nil {
		status.Files = stats.Files
	} else {
		status.Files = []types.TorrentFileMeta{}
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleStreamRelease(w http.ResponseWriter, r *http.Request) {
	magnet := r.URL.Query().Get("magnet")
	if magnet == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing magnet parameter"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"released": s.torrents.Release(magnet)})
}

func (s *Server) handlePlaybackProbe(w http.ResponseWriter, r *http.Request) {
	input := streaming.ResolvePlaybackInput(r.Context(), r)
	if input == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing or invalid magnet/url"})
		return
	}
	info, err := streaming.ProbeMedia(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	var videoCodec any
	if info.Video != nil {
		videoCodec = info.Video.Codec
	}
	var audioDesc any
	var selectedAudioIndex any
	if info.Audio != nil {
		if info.Audio.Language != "" {
			audioDesc = fmt.Sprintf("%s (%s)", info.Audio.Codec, info.Audio.Language)
		} else {
			audioDesc = info.Audio.Codec
		}
		selectedAudioIndex = info.Audio.Index
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"duration":           info.Duration,
		"directPlayable":     info.DirectPlayable,
		"reason":             info.Reason,
		"video":              videoCodec,
		"audio":              audioDesc,
		"selectedAudioIndex": selectedAudioIndex,
		"audioStreams":       info.AudioStreams,
		"subtitleStreams":    info.SubtitleStreams,
		"reencodesVideo":     info.ReencodesVideo,
	})
}

func (s *Server) handlePlaybackTranscode(w http.ResponseWriter, r *http.Request) {
	input := streaming.ResolvePlaybackInput(r.Context(), r)
	if input == "" {
		http.Error(w, "Missing or invalid magnet/url", http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	start, _ := strconv.ParseFloat(q.Get("start"), 64)
	if start < 0 {
		start = 0
	}
	var audioStreamIdx *int
	if rawAudio := q.Get("audioStream"); reDigits.MatchString(rawAudio) {
		if v, err := strconv.Atoi(rawAudio); err == nil {
			audioStreamIdx = &v
		}
	}

	info, err := streaming.ProbeMedia(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Duration", fmt.Sprintf("%v", info.Duration))
	w.WriteHeader(http.StatusOK)

	_ = streaming.TranscodeToMP4(r.Context(), input, start, info, audioStreamIdx, w)
}

func (s *Server) handleSubtitles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	imdbID := q.Get("imdbId")
	if !reValidIMDb.MatchString(imdbID) {
		writeJSON(w, http.StatusOK, map[string]any{"subtitles": []subtitles.SubtitleOption{}})
		return
	}
	mediaType := "movie"
	if q.Get("type") == "tv" {
		mediaType = "tv"
	}
	season, _ := strconv.Atoi(q.Get("season"))
	if season <= 0 {
		season = 1
	}
	episode, _ := strconv.Atoi(q.Get("episode"))
	if episode <= 0 {
		episode = 1
	}
	subs := subtitles.FindSubtitles(r.Context(), subtitles.FindSubtitlesParams{
		IMDbID:    imdbID,
		MediaType: mediaType,
		Season:    season,
		Episode:   episode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"subtitles": subs})
}

func (s *Server) handleSubtitlesVTT(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	subURL := q.Get("url")
	offset, _ := strconv.ParseFloat(q.Get("offset"), 64)
	if !subtitles.IsAllowedSubtitleURL(subURL) {
		http.Error(w, "Subtitle host not allowed", http.StatusBadRequest)
		return
	}

	resp, err := netguard.GuardedFetch(r.Context(), subURL, netguard.FetchOptions{
		Timeout:      15 * time.Second,
		AllowPrivate: false,
	})
	if err != nil {
		http.Error(w, "Subtitle download failed", http.StatusBadGateway)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		http.Error(w, "Subtitle download failed", http.StatusBadGateway)
		return
	}
	rawSRT, err := netguard.ReadBoundedText(resp, 2*1024*1024)
	if err != nil {
		http.Error(w, "Subtitle download failed", http.StatusBadGateway)
		return
	}
	vtt := subtitles.SRTToVTT(rawSRT, offset)
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(vtt))
}

func (s *Server) handleSubtitlesEmbedded(w http.ResponseWriter, r *http.Request) {
	input := streaming.ResolvePlaybackInput(r.Context(), r)
	if input == "" {
		http.Error(w, "Missing or invalid magnet/url", http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	rawStream := q.Get("stream")
	if !reDigits.MatchString(rawStream) {
		http.Error(w, "Missing or invalid stream index", http.StatusBadRequest)
		return
	}
	streamIdx, _ := strconv.Atoi(rawStream)
	offset, _ := strconv.ParseFloat(q.Get("offset"), 64)

	srt, err := streaming.ExtractEmbeddedSubtitleSRT(r.Context(), input, streamIdx)
	if err != nil {
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("WEBVTT\n\n"))
		return
	}
	vtt := subtitles.SRTToVTT(srt, offset)
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(vtt))
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	chs := live.GetAllChannels(r.Context())
	w.Header().Set("Cache-Control", "public, max-age=600")
	writeJSON(w, http.StatusOK, map[string]any{"channels": chs})
}

func (s *Server) handleLiveProxy(w http.ResponseWriter, r *http.Request) {
	live.ServeLiveProxy(w, r, nil)
}

func (s *Server) handleLiveStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URLs []string `json:"urls"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	statusMap := live.CheckBatchStreamStatus(r.Context(), body.URLs, nil)
	writeJSON(w, http.StatusOK, map[string]any{"status": statusMap})
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	st, err := s.stateStore.GetState(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, state.HouseholdState{
			WatchProgress:      []state.WatchProgressEntry{},
			FavoriteChannelIDs: []string{},
		})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handlePutState(w http.ResponseWriter, r *http.Request) {
	var patch state.StatePatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to save state"})
		return
	}
	st, err := s.stateStore.UpdateState(r.Context(), patch)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
	trs := streaming.GetTrackers(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"tmdbConfigured": os.Getenv("TMDB_API_KEY") != "",
		"trackerCount":   len(trs),
	})
}

func (s *Server) handleIntegrationsTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type    string `json:"type"`
		HostURL string `json:"hostUrl"`
		APIKey  string `json:"apiKey"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if (body.Type != "prowlarr" && body.Type != "jackett") || strings.TrimSpace(body.HostURL) == "" || strings.TrimSpace(body.APIKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "Missing type, hostUrl or apiKey",
		})
		return
	}
	res := scrapers.TestConnection(r.Context(), body.Type, body.HostURL, body.APIKey, nil)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if cleanPath == "" || cleanPath == "." {
		cleanPath = "index.html"
	}
	if f, err := s.spaFS.Open(cleanPath); err == nil {
		stat, statErr := f.Stat()
		_ = f.Close()
		if statErr == nil && !stat.IsDir() {
			http.FileServer(http.FS(s.spaFS)).ServeHTTP(w, r)
			return
		}
	}
	// Fallback to index.html for SPA routes
	indexBytes, err := fs.ReadFile(s.spaFS, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexBytes)
}
