package personalization

import (
	"math"
	"math/rand/v2"
	"slices"
	"sort"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

// TasteProfile stores cumulative genre affinity, pinned genres, liked titles, and recent hero IDs.
type TasteProfile struct {
	GenreScores    map[string]float64 `json:"genreScores"`
	FavoriteGenres []string           `json:"favoriteGenres"`
	LikedItems     []types.MediaItem  `json:"likedItems"`
	RecentHeroIDs  []string           `json:"recentHeroIds"`
}

// NewTasteProfile returns an initialized empty TasteProfile.
func NewTasteProfile() TasteProfile {
	return TasteProfile{
		GenreScores:    make(map[string]float64),
		FavoriteGenres: []string{},
		LikedItems:     []types.MediaItem{},
		RecentHeroIDs:  []string{},
	}
}

// IsMediaLiked reports whether mediaID is in profile.LikedItems.
func IsMediaLiked(id any, profile TasteProfile) bool {
	key := (types.MediaItem{ID: id}).IDString()
	for _, m := range profile.LikedItems {
		if m.IDString() == key || (m.ImdbID != "" && m.ImdbID == key) {
			return true
		}
	}
	return false
}

// RecordMediaInteraction boosts affinity for item's genres by weight (+1 view, +3 play, +5 like).
func RecordMediaInteraction(profile TasteProfile, item types.MediaItem, weight float64) TasteProfile {
	nextScores := make(map[string]float64, len(profile.GenreScores))
	for k, v := range profile.GenreScores {
		nextScores[k] = v
	}
	for _, g := range item.Genres {
		if g == "" {
			continue
		}
		nextScores[g] += weight
	}
	profile.GenreScores = nextScores
	return profile
}

// ToggleLikedMedia toggles item in profile.LikedItems (max 60) and adjusts genre scores (+5 / -4).
func ToggleLikedMedia(profile TasteProfile, item types.MediaItem) (bool, TasteProfile) {
	key := item.IDString()
	exists := false
	for _, m := range profile.LikedItems {
		if m.IDString() == key {
			exists = true
			break
		}
	}
	nextScores := make(map[string]float64, len(profile.GenreScores))
	for k, v := range profile.GenreScores {
		nextScores[k] = v
	}
	delta := 5.0
	if exists {
		delta = -4.0
	}
	for _, g := range item.Genres {
		if g == "" {
			continue
		}
		nextScores[g] = math.Max(0, nextScores[g]+delta)
	}
	var nextLiked []types.MediaItem
	if !exists {
		nextLiked = append(nextLiked, item)
	}
	for _, m := range profile.LikedItems {
		if m.IDString() != key {
			nextLiked = append(nextLiked, m)
		}
	}
	if len(nextLiked) > 60 {
		nextLiked = nextLiked[:60]
	}
	profile.GenreScores = nextScores
	profile.LikedItems = nextLiked
	return !exists, profile
}

// ToggleFavoriteGenre pins or unpins genre on the taste bar (+8 / -6).
func ToggleFavoriteGenre(profile TasteProfile, genre string) TasteProfile {
	has := slices.Contains(profile.FavoriteGenres, genre)
	var favs []string
	if has {
		for _, g := range profile.FavoriteGenres {
			if g != genre {
				favs = append(favs, g)
			}
		}
	} else {
		favs = append(append([]string(nil), profile.FavoriteGenres...), genre)
	}
	nextScores := make(map[string]float64, len(profile.GenreScores))
	for k, v := range profile.GenreScores {
		nextScores[k] = v
	}
	delta := 8.0
	if has {
		delta = -6.0
	}
	nextScores[genre] = math.Max(0, nextScores[genre]+delta)
	profile.FavoriteGenres = favs
	profile.GenreScores = nextScores
	return profile
}

// RememberHeroID prepends id to profile.RecentHeroIDs (capped at 12).
func RememberHeroID(profile TasteProfile, id any) TasteProfile {
	key := (types.MediaItem{ID: id}).IDString()
	next := []string{key}
	for _, x := range profile.RecentHeroIDs {
		if x != key {
			next = append(next, x)
		}
	}
	if len(next) > 12 {
		next = next[:12]
	}
	profile.RecentHeroIDs = next
	return profile
}

// ScoreMediaForUser calculates a personalization score for a MediaItem based on the user's TasteProfile.
func ScoreMediaForUser(item types.MediaItem, profile TasteProfile) float64 {
	vote := item.VoteAverage
	if vote == 0 {
		vote = 6.5
	}
	score := vote * 0.6
	for _, g := range item.Genres {
		if slices.Contains(profile.FavoriteGenres, g) {
			score += 10
		}
		if gs, ok := profile.GenreScores[g]; ok && gs > 0 {
			score += math.Min(15, gs*1.5)
		}
	}
	if IsMediaLiked(item.ID, profile) {
		score += 6
	}
	return score
}

// WeightedShuffle shuffles items biased toward higher scoreFn values with random jitter [0, 8).
func WeightedShuffle[T any](items []T, scoreFn func(T) float64) []T {
	type scored struct {
		item    T
		sortKey float64
	}
	arr := make([]scored, len(items))
	for i, it := range items {
		arr[i] = scored{
			item:    it,
			sortKey: scoreFn(it) + rand.Float64()*8.0,
		}
	}
	sort.SliceStable(arr, func(i, j int) bool {
		return arr[i].sortKey > arr[j].sortKey
	})
	out := make([]T, len(items))
	for i, s := range arr {
		out[i] = s.item
	}
	return out
}

// PickRandomHero picks a high-quality Hero title tailored to the user's taste, avoiding recentHeroIds and excludeID.
func PickRandomHero(rows []types.CatalogRow, profile TasteProfile, excludeID string) *types.MediaItem {
	seen := make(map[string]bool)
	var candidates []types.MediaItem
	for _, row := range rows {
		for _, item := range row.Items {
			id := item.IDString()
			if seen[id] {
				continue
			}
			seen[id] = true
			if item.BackdropPath == nil || *item.BackdropPath == "" || item.Overview == "" {
				continue
			}
			candidates = append(candidates, item)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	recentSet := make(map[string]bool, len(profile.RecentHeroIDs))
	for _, r := range profile.RecentHeroIDs {
		recentSet[r] = true
	}

	var pool []types.MediaItem
	for _, c := range candidates {
		id := c.IDString()
		if id != excludeID && !recentSet[id] {
			pool = append(pool, c)
		}
	}
	if len(pool) == 0 {
		for _, c := range candidates {
			if c.IDString() != excludeID {
				pool = append(pool, c)
			}
		}
	}
	if len(pool) == 0 {
		pool = candidates
	}

	if len(profile.FavoriteGenres) > 0 {
		var favMatches []types.MediaItem
		for _, c := range pool {
			for _, g := range c.Genres {
				if slices.Contains(profile.FavoriteGenres, g) {
					favMatches = append(favMatches, c)
					break
				}
			}
		}
		if len(favMatches) >= 3 {
			pool = favMatches
		}
	}

	ranked := WeightedShuffle(pool, func(it types.MediaItem) float64 {
		return ScoreMediaForUser(it, profile)
	})
	limit := min(8, len(ranked))
	topSlice := ranked[:limit]
	chosen := topSlice[rand.IntN(len(topSlice))]
	return &chosen
}

// PersonalizeRows builds personalized & freshly shuffled Home rows.
func PersonalizeRows(rawRows []types.CatalogRow, profile TasteProfile) []types.CatalogRow {
	if len(rawRows) == 0 {
		return rawRows
	}
	hasTaste := len(profile.FavoriteGenres) > 0 || len(profile.GenreScores) > 0 || len(profile.LikedItems) > 0

	seen := make(map[string]bool)
	var allItems []types.MediaItem
	for _, r := range rawRows {
		for _, item := range r.Items {
			id := item.IDString()
			if seen[id] {
				continue
			}
			seen[id] = true
			allItems = append(allItems, item)
		}
	}

	var forYouPool []types.MediaItem
	if len(profile.FavoriteGenres) > 0 {
		for _, it := range allItems {
			for _, g := range it.Genres {
				if slices.Contains(profile.FavoriteGenres, g) {
					forYouPool = append(forYouPool, it)
					break
				}
			}
		}
	} else {
		forYouPool = allItems
	}
	if len(forYouPool) < 8 {
		forYouPool = allItems
	}

	forYouItems := WeightedShuffle(forYouPool, func(it types.MediaItem) float64 {
		return ScoreMediaForUser(it, profile)
	})
	if len(forYouItems) > 30 {
		forYouItems = forYouItems[:30]
	}

	activeGenresLabel := "Freshly shuffled top picks across movies & series"
	if len(profile.FavoriteGenres) > 0 {
		topFavs := profile.FavoriteGenres
		if len(topFavs) > 3 {
			topFavs = topFavs[:3]
		}
		label := topFavs[0]
		for i := 1; i < len(topFavs); i++ {
			label += ", " + topFavs[i]
		}
		activeGenresLabel = "Based on your " + label + " picks"
	} else if hasTaste {
		activeGenresLabel = "Tailored to what you watch and like — shuffled every visit"
	}

	shuffledRows := make([]types.CatalogRow, len(rawRows))
	for i, row := range rawRows {
		if row.Key == "airing-today" {
			shuffledRows[i] = row
			continue
		}
		cp := row
		cp.Items = WeightedShuffle(row.Items, func(it types.MediaItem) float64 {
			return ScoreMediaForUser(it, profile)
		})
		shuffledRows[i] = cp
	}

	var coreRows, genreRows []types.CatalogRow
	for _, r := range shuffledRows {
		if len(r.Key) >= 6 && r.Key[:6] == "genre-" {
			genreRows = append(genreRows, r)
		} else {
			coreRows = append(coreRows, r)
		}
	}

	scoreRow := func(r types.CatalogRow) float64 {
		var sampleGenres []string
		if len(r.Items) > 0 {
			sampleGenres = r.Items[0].Genres
		}
		s := rand.Float64() * 2.0
		for _, g := range sampleGenres {
			if slices.Contains(profile.FavoriteGenres, g) {
				s += 20
			}
			s += profile.GenreScores[g]
		}
		return s
	}
	type scoredRow struct {
		row   types.CatalogRow
		score float64
	}
	sRows := make([]scoredRow, len(genreRows))
	for i, gr := range genreRows {
		sRows[i] = scoredRow{row: gr, score: scoreRow(gr)}
	}
	sort.SliceStable(sRows, func(i, j int) bool {
		return sRows[i].score > sRows[j].score
	})
	for i, sr := range sRows {
		genreRows[i] = sr.row
	}

	var out []types.CatalogRow
	if len(forYouItems) > 0 {
		out = append(out, types.CatalogRow{
			Key:      "for-you",
			Title:    "Picked for You",
			Subtitle: activeGenresLabel,
			Items:    forYouItems,
		})
	}
	if len(profile.LikedItems) > 0 {
		out = append(out, types.CatalogRow{
			Key:      "my-likes",
			Title:    "Your Liked Titles",
			Subtitle: "Movies & series you hearted",
			Items:    profile.LikedItems,
		})
	}
	out = append(out, coreRows...)
	out = append(out, genreRows...)
	return out
}
