package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/types"
	_ "modernc.org/sqlite"
)

const (
	MaxProgressItems    = 40
	MaxFavoriteChannels = 200
)

// EpisodeProgress stores resume position and duration for a single episode.
type EpisodeProgress struct {
	Position  int   `json:"position"`
	Duration  int   `json:"duration"`
	UpdatedAt int64 `json:"updatedAt"`
}

// WatchProgressEntry represents playback progress for a movie or TV/anime series.
type WatchProgressEntry struct {
	Media     types.MediaItem            `json:"media"`
	Season    *int                       `json:"season,omitempty"`
	Episode   *int                       `json:"episode,omitempty"`
	Position  int                        `json:"position"`
	Duration  int                        `json:"duration"`
	UpdatedAt int64                      `json:"updatedAt"`
	Episodes  map[string]EpisodeProgress `json:"episodes,omitempty"`
}

// HouseholdState is the wire payload returned by GET/PUT /api/state.
type HouseholdState struct {
	WatchProgress      []WatchProgressEntry `json:"watchProgress"`
	FavoriteChannelIDs []string             `json:"favoriteChannelIds"`
	UpdatedAt          int64                `json:"updatedAt,omitempty"`
}

// StatePatch is the incoming request payload for PUT /api/state.
type StatePatch struct {
	WatchProgress        *[]WatchProgressEntry `json:"watchProgress,omitempty"`
	FavoriteChannelIDs   *[]string             `json:"favoriteChannelIds,omitempty"`
	RemovedMediaIDs      []string              `json:"removedMediaIds,omitempty"`
	RemovedChannelIDs    []string              `json:"removedChannelIds,omitempty"`
	ReplaceWatchProgress bool                  `json:"replaceWatchProgress,omitempty"`
	ReplaceFavorites     bool                  `json:"replaceFavorites,omitempty"`
}

func compactMediaItem(m types.MediaItem) types.MediaItem {
	cp := m
	cp.Seasons = nil
	return cp
}

// GetMediaProgress finds a WatchProgressEntry by media ID or IMDb ID.
func GetMediaProgress(mediaID any, list []WatchProgressEntry) *WatchProgressEntry {
	key := fmt.Sprintf("%v", mediaID)
	for i := range list {
		if list[i].Media.IDString() == key || (list[i].Media.ImdbID != "" && list[i].Media.ImdbID == key) {
			cp := list[i]
			return &cp
		}
	}
	return nil
}

// GetEpisodeProgress returns the saved EpisodeProgress for (mediaID, season, episode).
func GetEpisodeProgress(mediaID any, season, episode int, list []WatchProgressEntry) *EpisodeProgress {
	entry := GetMediaProgress(mediaID, list)
	if entry == nil {
		return nil
	}
	epKey := fmt.Sprintf("%d:%d", season, episode)
	if entry.Episodes != nil {
		if ep, ok := entry.Episodes[epKey]; ok {
			return &ep
		}
	}
	if entry.Season != nil && *entry.Season == season && entry.Episode != nil && *entry.Episode == episode {
		return &EpisodeProgress{
			Position:  entry.Position,
			Duration:  entry.Duration,
			UpdatedAt: entry.UpdatedAt,
		}
	}
	return nil
}

// RemoveWatchProgress removes mediaID from list and returns the sorted, capped slice.
func RemoveWatchProgress(list []WatchProgressEntry, mediaID any) []WatchProgressEntry {
	key := fmt.Sprintf("%v", mediaID)
	var next []WatchProgressEntry
	for _, e := range list {
		if e.Media.IDString() != key && (e.Media.ImdbID == "" || e.Media.ImdbID != key) {
			next = append(next, e)
		}
	}
	return normalizeWatchProgressList(next)
}

func normalizeWatchProgressList(list []WatchProgressEntry) []WatchProgressEntry {
	out := make([]WatchProgressEntry, len(list))
	copy(out, list)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	if len(out) > MaxProgressItems {
		out = out[:MaxProgressItems]
	}
	if out == nil {
		return []WatchProgressEntry{}
	}
	return out
}

// SaveWatchProgress records playback progress for a movie or episode into list:
// - Ignores positions < 5s
// - Removes movies reaching >= 94% completion
// - Stores per-episode progress map keyed by "${season}:${episode}" for series
func SaveWatchProgress(
	list []WatchProgressEntry,
	media types.MediaItem,
	position float64,
	duration float64,
	season *int,
	episode *int,
) []WatchProgressEntry {
	if math.IsNaN(position) || math.IsInf(position, 0) || position < 5 {
		return normalizeWatchProgressList(list)
	}
	safeDuration := 0.0
	if !math.IsNaN(duration) && !math.IsInf(duration, 0) && duration > 0 {
		safeDuration = duration
	}
	ratio := 0.0
	if safeDuration > 0 {
		ratio = position / safeDuration
	}

	key := media.IDString()
	var existing *WatchProgressEntry
	for i := range list {
		if list[i].Media.IDString() == key {
			existing = &list[i]
			break
		}
	}

	if media.MediaType == "movie" && ratio >= 0.94 {
		return RemoveWatchProgress(list, media.ID)
	}

	now := time.Now().UnixMilli()
	episodesMap := make(map[string]EpisodeProgress)
	if existing != nil && existing.Episodes != nil {
		for k, v := range existing.Episodes {
			episodesMap[k] = v
		}
	}
	if season != nil && episode != nil {
		epKey := fmt.Sprintf("%d:%d", *season, *episode)
		episodesMap[epKey] = EpisodeProgress{
			Position:  int(math.Floor(position)),
			Duration:  int(math.Floor(safeDuration)),
			UpdatedAt: now,
		}
	}

	var epField map[string]EpisodeProgress
	if len(episodesMap) > 0 {
		epField = episodesMap
	}

	updated := WatchProgressEntry{
		Media:     compactMediaItem(media),
		Season:    season,
		Episode:   episode,
		Position:  int(math.Floor(position)),
		Duration:  int(math.Floor(safeDuration)),
		UpdatedAt: now,
		Episodes:  epField,
	}

	next := []WatchProgressEntry{updated}
	for _, e := range list {
		if e.Media.IDString() != key {
			next = append(next, e)
		}
	}
	return normalizeWatchProgressList(next)
}

// MergeWatchProgressEntry merges two WatchProgressEntry records for the same media item by updatedAt timestamp,
// preserving per-episode progress across both records.
func MergeWatchProgressEntry(existing, incoming WatchProgressEntry) WatchProgressEntry {
	mergedEpisodes := make(map[string]EpisodeProgress)
	for k, v := range existing.Episodes {
		mergedEpisodes[k] = v
	}
	for k, v := range incoming.Episodes {
		if prev, ok := mergedEpisodes[k]; !ok || v.UpdatedAt >= prev.UpdatedAt {
			mergedEpisodes[k] = v
		}
	}
	winner := existing
	if incoming.UpdatedAt >= existing.UpdatedAt {
		winner = incoming
	}
	winner.Media = compactMediaItem(winner.Media)
	if len(mergedEpisodes) > 0 {
		winner.Episodes = mergedEpisodes
	}
	return winner
}

// Store provides concurrency-safe household state persistence backed by embedded SQLite (`modernc.org/sqlite`).
type Store struct {
	mu      sync.Mutex
	db      *sql.DB
	dataDir string
}

// ResolveDataDir returns the configured LUMEN_DATA_DIR or SCREENTIME_DATA_DIR, or os.TempDir()/lumen-data.
func ResolveDataDir() string {
	dir := os.Getenv("LUMEN_DATA_DIR")
	if dir == "" {
		dir = os.Getenv("SCREENTIME_DATA_DIR")
	}
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "lumen-data")
	}
	return dir
}

// NewStore opens or initializes the SQLite state database in dataDir.
func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		dataDir = ResolveDataDir()
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "lumen.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	schema := `
	CREATE TABLE IF NOT EXISTS watch_progress (
		media_id TEXT PRIMARY KEY,
		updated_at INTEGER NOT NULL,
		payload_json TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS favorite_channels (
		channel_id TEXT PRIMARY KEY,
		position_order INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &Store{
		db:      db,
		dataDir: dataDir,
	}
	s.migrateLegacyJSONIfEmpty()
	return s, nil
}

// Close closes the underlying SQLite database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) migrateLegacyJSONIfEmpty() {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM watch_progress`).Scan(&count); err != nil || count > 0 {
		return
	}
	var favCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM favorite_channels`).Scan(&favCount); err != nil || favCount > 0 {
		return
	}

	legacyPath := filepath.Join(s.dataDir, "state.json")
	raw, err := os.ReadFile(legacyPath)
	if err != nil {
		return
	}
	var legacy HouseholdState
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return
	}
	if len(legacy.WatchProgress) == 0 && len(legacy.FavoriteChannelIDs) == 0 {
		return
	}
	_, _ = s.UpdateState(context.Background(), StatePatch{
		WatchProgress:      &legacy.WatchProgress,
		FavoriteChannelIDs: &legacy.FavoriteChannelIDs,
	})
}

// GetState reads the current HouseholdState from SQLite.
func (s *Store) GetState(ctx context.Context) (HouseholdState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readStateLocked(ctx)
}

func (s *Store) readStateLocked(ctx context.Context) (HouseholdState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload_json FROM watch_progress ORDER BY updated_at DESC LIMIT ?`, MaxProgressItems)
	if err != nil {
		return HouseholdState{WatchProgress: []WatchProgressEntry{}, FavoriteChannelIDs: []string{}}, err
	}
	defer func() { _ = rows.Close() }()

	var wp []WatchProgressEntry
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var entry WatchProgressEntry
		if err := json.Unmarshal([]byte(raw), &entry); err == nil && entry.Media.IDString() != "" {
			wp = append(wp, entry)
		}
	}
	if wp == nil {
		wp = []WatchProgressEntry{}
	}

	favRows, err := s.db.QueryContext(ctx, `SELECT channel_id FROM favorite_channels ORDER BY position_order ASC, updated_at DESC LIMIT ?`, MaxFavoriteChannels)
	if err != nil {
		return HouseholdState{WatchProgress: wp, FavoriteChannelIDs: []string{}}, err
	}
	defer func() { _ = favRows.Close() }()

	var favs []string
	for favRows.Next() {
		var chID string
		if err := favRows.Scan(&chID); err == nil && chID != "" {
			favs = append(favs, chID)
		}
	}
	if favs == nil {
		favs = []string{}
	}

	return HouseholdState{
		WatchProgress:      wp,
		FavoriteChannelIDs: favs,
	}, nil
}

// UpdateState atomically merges or updates watchProgress and favoriteChannelIds in SQLite,
// preserving newer per-item timestamps across concurrent household devices.
func (s *Store) UpdateState(ctx context.Context, patch StatePatch) (HouseholdState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return HouseholdState{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UnixMilli()

	for _, rmID := range patch.RemovedMediaIDs {
		if rmID != "" {
			_, _ = tx.ExecContext(ctx, `DELETE FROM watch_progress WHERE media_id = ?`, rmID)
		}
	}

	if patch.WatchProgress != nil {
		if patch.ReplaceWatchProgress {
			_, _ = tx.ExecContext(ctx, `DELETE FROM watch_progress`)
		}
		incomingList := *patch.WatchProgress
		if len(incomingList) > MaxProgressItems {
			incomingList = incomingList[:MaxProgressItems]
		}
		for _, item := range incomingList {
			mediaID := item.Media.IDString()
			if mediaID == "" {
				continue
			}
			// Auto-remove finished movies (>= 94% watched)
			if item.Media.MediaType == "movie" && item.Duration > 0 && float64(item.Position)/float64(item.Duration) >= 0.94 {
				_, _ = tx.ExecContext(ctx, `DELETE FROM watch_progress WHERE media_id = ?`, mediaID)
				continue
			}
			if item.UpdatedAt <= 0 {
				item.UpdatedAt = now
			}

			var existingRaw string
			err := tx.QueryRowContext(ctx, `SELECT payload_json FROM watch_progress WHERE media_id = ?`, mediaID).Scan(&existingRaw)
			merged := item
			merged.Media = compactMediaItem(merged.Media)
			if err == nil && existingRaw != "" {
				var existing WatchProgressEntry
				if jsonErr := json.Unmarshal([]byte(existingRaw), &existing); jsonErr == nil {
					merged = MergeWatchProgressEntry(existing, item)
				}
			}

			payloadBytes, err := json.Marshal(merged)
			if err != nil {
				continue
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO watch_progress (media_id, updated_at, payload_json)
				VALUES (?, ?, ?)
				ON CONFLICT(media_id) DO UPDATE SET
					updated_at = excluded.updated_at,
					payload_json = excluded.payload_json
			`, mediaID, merged.UpdatedAt, string(payloadBytes))
			if err != nil {
				return HouseholdState{}, err
			}
		}

		// Cap table at MaxProgressItems (40)
		_, _ = tx.ExecContext(ctx, `
			DELETE FROM watch_progress
			WHERE media_id NOT IN (
				SELECT media_id FROM watch_progress ORDER BY updated_at DESC LIMIT ?
			)
		`, MaxProgressItems)
	}

	for _, rmCh := range patch.RemovedChannelIDs {
		if rmCh != "" {
			_, _ = tx.ExecContext(ctx, `DELETE FROM favorite_channels WHERE channel_id = ?`, rmCh)
		}
	}

	if patch.FavoriteChannelIDs != nil {
		if patch.ReplaceFavorites {
			_, _ = tx.ExecContext(ctx, `DELETE FROM favorite_channels`)
		}
		favList := *patch.FavoriteChannelIDs
		seen := make(map[string]bool)
		var cleanFavs []string
		for _, ch := range favList {
			if ch != "" && !seen[ch] {
				seen[ch] = true
				cleanFavs = append(cleanFavs, ch)
			}
		}
		if len(cleanFavs) > MaxFavoriteChannels {
			cleanFavs = cleanFavs[:MaxFavoriteChannels]
		}

		// Also preserve existing favorite channels unless explicitly removed or ReplaceFavorites is set
		if !patch.ReplaceFavorites {
			rows, err := tx.QueryContext(ctx, `SELECT channel_id FROM favorite_channels ORDER BY position_order ASC`)
			if err == nil {
				var existingFavs []string
				for rows.Next() {
					var c string
					if err := rows.Scan(&c); err == nil && c != "" && !seen[c] && !slices.Contains(patch.RemovedChannelIDs, c) {
						seen[c] = true
						existingFavs = append(existingFavs, c)
					}
				}
				_ = rows.Close()
				cleanFavs = append(cleanFavs, existingFavs...)
				if len(cleanFavs) > MaxFavoriteChannels {
					cleanFavs = cleanFavs[:MaxFavoriteChannels]
				}
			}
		}

		_, _ = tx.ExecContext(ctx, `DELETE FROM favorite_channels`)
		for idx, chID := range cleanFavs {
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO favorite_channels (channel_id, position_order, updated_at)
				VALUES (?, ?, ?)
			`, chID, idx, now)
		}
	}

	if err := tx.Commit(); err != nil {
		return HouseholdState{}, err
	}

	state, err := s.readStateLocked(ctx)
	if err != nil {
		return HouseholdState{}, err
	}
	state.UpdatedAt = now

	// Write JSON mirror file for compatibility/inspection
	if rawMirror, err := json.Marshal(state); err == nil {
		_ = os.WriteFile(filepath.Join(s.dataDir, "state.json"), rawMirror, 0o644)
	}

	return state, nil
}
