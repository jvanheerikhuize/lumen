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
	"strings"
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

// HouseholdMember represents a member profile in the household.
type HouseholdMember struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Nickname string `json:"nickname,omitempty"`
	Color    string `json:"color"`
	IsKids   bool   `json:"isKids,omitempty"`
}

// DefaultHouseholdMembers are the initial household member profiles.
var DefaultHouseholdMembers = []HouseholdMember{
	{ID: "default", Name: "Member 1", Color: "rose"},
	{ID: "member-2", Name: "Member 2", Color: "sky"},
	{ID: "kids", Name: "Kids", Color: "emerald", IsKids: true},
}

// HouseholdState is the wire payload returned by GET/PUT /api/state.
type HouseholdState struct {
	ProfileID          string               `json:"profileId,omitempty"`
	Profiles           []HouseholdMember    `json:"profiles,omitempty"`
	WatchProgress      []WatchProgressEntry `json:"watchProgress"`
	FavoriteChannelIDs []string             `json:"favoriteChannelIds"`
	UpdatedAt          int64                `json:"updatedAt,omitempty"`
}

// StatePatch is the incoming request payload for PUT /api/state.
type StatePatch struct {
	ProfileID            string                `json:"profileId,omitempty"`
	Profiles             *[]HouseholdMember    `json:"profiles,omitempty"`
	WatchProgress        *[]WatchProgressEntry `json:"watchProgress,omitempty"`
	FavoriteChannelIDs   *[]string             `json:"favoriteChannelIds,omitempty"`
	RemovedMediaIDs      []string              `json:"removedMediaIds,omitempty"`
	RemovedChannelIDs    []string              `json:"removedChannelIds,omitempty"`
	ReplaceWatchProgress bool                  `json:"replaceWatchProgress,omitempty"`
	ReplaceFavorites     bool                  `json:"replaceFavorites,omitempty"`
}

// MovieCompletionRatio is the playback threshold (97%) where a movie is treated as finished and removed from Continue Watching.
const MovieCompletionRatio = 0.97

func normalizeProfileID(profileID string) string {
	if profileID == "" {
		return "default"
	}
	return profileID
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
// - Removes movies reaching >= 97% completion
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

	if media.MediaType == "movie" && ratio >= MovieCompletionRatio {
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

// Store provides concurrency-safe per-member household state persistence backed by embedded SQLite.
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
	CREATE TABLE IF NOT EXISTS profile_watch_progress (
		profile_id TEXT NOT NULL DEFAULT 'default',
		media_id TEXT NOT NULL,
		updated_at INTEGER NOT NULL,
		payload_json TEXT NOT NULL,
		PRIMARY KEY (profile_id, media_id)
	);
	CREATE TABLE IF NOT EXISTS profile_favorite_channels (
		profile_id TEXT NOT NULL DEFAULT 'default',
		channel_id TEXT NOT NULL,
		position_order INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		PRIMARY KEY (profile_id, channel_id)
	);
	CREATE TABLE IF NOT EXISTS household_profiles (
		profile_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		nickname TEXT NOT NULL DEFAULT '',
		color TEXT NOT NULL,
		is_kids INTEGER NOT NULL DEFAULT 0,
		position_order INTEGER NOT NULL
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
	s.ensureProfileColumns()
	s.ensureDefaultProfiles()
	s.migrateLegacyTablesAndJSONIfEmpty()
	return s, nil
}

// Close closes the underlying SQLite database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) ensureProfileColumns() {
	rows, err := s.db.Query(`PRAGMA table_info(household_profiles)`)
	if err != nil {
		return
	}
	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			cols[name] = true
		}
	}
	_ = rows.Close()

	if !cols["nickname"] {
		_, _ = s.db.Exec(`ALTER TABLE household_profiles ADD COLUMN nickname TEXT NOT NULL DEFAULT ''`)
	}
	if !cols["is_kids"] {
		if _, err := s.db.Exec(`ALTER TABLE household_profiles ADD COLUMN is_kids INTEGER NOT NULL DEFAULT 0`); err == nil {
			_, _ = s.db.Exec(`UPDATE household_profiles SET is_kids = 1 WHERE profile_id = 'kids'`)
		}
	}
}

func (s *Store) ensureDefaultProfiles() {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM household_profiles`).Scan(&count); err != nil || count > 0 {
		return
	}
	for idx, m := range DefaultHouseholdMembers {
		isKidsInt := 0
		if m.IsKids {
			isKidsInt = 1
		}
		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO household_profiles (profile_id, name, nickname, color, is_kids, position_order) VALUES (?, ?, ?, ?, ?, ?)`,
			m.ID, m.Name, m.Nickname, m.Color, isKidsInt, idx,
		)
	}
}

func (s *Store) migrateLegacyTablesAndJSONIfEmpty() {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM profile_watch_progress`).Scan(&count); err != nil || count > 0 {
		return
	}
	var favCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM profile_favorite_channels`).Scan(&favCount); err != nil || favCount > 0 {
		return
	}

	// Migrate from older single-profile tables if they exist
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO profile_watch_progress (profile_id, media_id, updated_at, payload_json) SELECT 'default', media_id, updated_at, payload_json FROM watch_progress`)
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO profile_favorite_channels (profile_id, channel_id, position_order, updated_at) SELECT 'default', channel_id, position_order, updated_at FROM favorite_channels`)

	_ = s.db.QueryRow(`SELECT COUNT(*) FROM profile_watch_progress`).Scan(&count)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM profile_favorite_channels`).Scan(&favCount)
	if count > 0 || favCount > 0 {
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
		ProfileID:          "default",
		WatchProgress:      &legacy.WatchProgress,
		FavoriteChannelIDs: &legacy.FavoriteChannelIDs,
	})
}

// GetState reads the current HouseholdState for the default profile.
func (s *Store) GetState(ctx context.Context) (HouseholdState, error) {
	return s.GetStateForProfile(ctx, "default")
}

// GetStateForProfile reads the HouseholdState for a specific household member profile.
func (s *Store) GetStateForProfile(ctx context.Context, profileID string) (HouseholdState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readStateLocked(ctx, normalizeProfileID(profileID))
}

func (s *Store) readProfilesLocked(ctx context.Context) []HouseholdMember {
	rows, err := s.db.QueryContext(ctx, `SELECT profile_id, name, nickname, color, is_kids FROM household_profiles ORDER BY position_order ASC`)
	if err != nil {
		return DefaultHouseholdMembers
	}
	defer func() { _ = rows.Close() }()
	var list []HouseholdMember
	for rows.Next() {
		var m HouseholdMember
		var isKidsInt int
		if err := rows.Scan(&m.ID, &m.Name, &m.Nickname, &m.Color, &isKidsInt); err == nil && m.ID != "" {
			m.IsKids = isKidsInt != 0
			list = append(list, m)
		}
	}
	if len(list) == 0 {
		return DefaultHouseholdMembers
	}
	return list
}

func (s *Store) readStateLocked(ctx context.Context, profileID string) (HouseholdState, error) {
	profileID = normalizeProfileID(profileID)
	profiles := s.readProfilesLocked(ctx)

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT payload_json FROM profile_watch_progress WHERE profile_id = ? ORDER BY updated_at DESC LIMIT ?`,
		profileID, MaxProgressItems,
	)
	if err != nil {
		return HouseholdState{ProfileID: profileID, Profiles: profiles, WatchProgress: []WatchProgressEntry{}, FavoriteChannelIDs: []string{}}, err
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

	favRows, err := s.db.QueryContext(
		ctx,
		`SELECT channel_id FROM profile_favorite_channels WHERE profile_id = ? ORDER BY position_order ASC, updated_at DESC LIMIT ?`,
		profileID, MaxFavoriteChannels,
	)
	if err != nil {
		return HouseholdState{ProfileID: profileID, Profiles: profiles, WatchProgress: wp, FavoriteChannelIDs: []string{}}, err
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
		ProfileID:          profileID,
		Profiles:           profiles,
		WatchProgress:      wp,
		FavoriteChannelIDs: favs,
	}, nil
}

// UpdateState atomically merges or updates watchProgress and favoriteChannelIds for a household member profile in SQLite,
// preserving newer per-item timestamps across concurrent household devices.
func (s *Store) UpdateState(ctx context.Context, patch StatePatch) (HouseholdState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	profileID := normalizeProfileID(patch.ProfileID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return HouseholdState{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UnixMilli()

	if patch.Profiles != nil && len(*patch.Profiles) > 0 {
		seenProfiles := make(map[string]bool)
		var validProfiles []HouseholdMember
		for _, p := range *patch.Profiles {
			id := strings.TrimSpace(p.ID)
			name := strings.TrimSpace(p.Name)
			if id == "" || name == "" || seenProfiles[id] {
				continue
			}
			seenProfiles[id] = true
			color := strings.TrimSpace(p.Color)
			if color == "" {
				color = "rose"
			}
			validProfiles = append(validProfiles, HouseholdMember{
				ID:       id,
				Name:     name,
				Nickname: strings.TrimSpace(p.Nickname),
				Color:    color,
				IsKids:   p.IsKids,
			})
		}
		if len(validProfiles) > 0 {
			existingRows, err := tx.QueryContext(ctx, `SELECT profile_id FROM household_profiles`)
			if err == nil {
				var orphanIDs []string
				for existingRows.Next() {
					var existingID string
					if err := existingRows.Scan(&existingID); err == nil && existingID != "" && !seenProfiles[existingID] {
						orphanIDs = append(orphanIDs, existingID)
					}
				}
				_ = existingRows.Close()
				for _, orphanID := range orphanIDs {
					_, _ = tx.ExecContext(ctx, `DELETE FROM profile_watch_progress WHERE profile_id = ?`, orphanID)
					_, _ = tx.ExecContext(ctx, `DELETE FROM profile_favorite_channels WHERE profile_id = ?`, orphanID)
				}
			}

			_, _ = tx.ExecContext(ctx, `DELETE FROM household_profiles`)
			for idx, p := range validProfiles {
				isKidsInt := 0
				if p.IsKids {
					isKidsInt = 1
				}
				_, _ = tx.ExecContext(
					ctx,
					`INSERT INTO household_profiles (profile_id, name, nickname, color, is_kids, position_order) VALUES (?, ?, ?, ?, ?, ?)`,
					p.ID, p.Name, p.Nickname, p.Color, isKidsInt, idx,
				)
			}
		}
	}

	for _, rmID := range patch.RemovedMediaIDs {
		if rmID != "" {
			_, _ = tx.ExecContext(ctx, `DELETE FROM profile_watch_progress WHERE profile_id = ? AND media_id = ?`, profileID, rmID)
		}
	}

	if patch.WatchProgress != nil {
		if patch.ReplaceWatchProgress {
			_, _ = tx.ExecContext(ctx, `DELETE FROM profile_watch_progress WHERE profile_id = ?`, profileID)
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
			// Auto-remove finished movies (>= 97% watched)
			if item.Media.MediaType == "movie" && item.Duration > 0 && float64(item.Position)/float64(item.Duration) >= MovieCompletionRatio {
				_, _ = tx.ExecContext(ctx, `DELETE FROM profile_watch_progress WHERE profile_id = ? AND media_id = ?`, profileID, mediaID)
				continue
			}
			if item.UpdatedAt <= 0 {
				item.UpdatedAt = now
			}

			var existingRaw string
			err := tx.QueryRowContext(
				ctx,
				`SELECT payload_json FROM profile_watch_progress WHERE profile_id = ? AND media_id = ?`,
				profileID, mediaID,
			).Scan(&existingRaw)
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
				INSERT INTO profile_watch_progress (profile_id, media_id, updated_at, payload_json)
				VALUES (?, ?, ?, ?)
				ON CONFLICT(profile_id, media_id) DO UPDATE SET
					updated_at = excluded.updated_at,
					payload_json = excluded.payload_json
			`, profileID, mediaID, merged.UpdatedAt, string(payloadBytes))
			if err != nil {
				return HouseholdState{}, err
			}
		}

		// Cap table for this profile at MaxProgressItems (40)
		_, _ = tx.ExecContext(ctx, `
			DELETE FROM profile_watch_progress
			WHERE profile_id = ? AND media_id NOT IN (
				SELECT media_id FROM profile_watch_progress WHERE profile_id = ? ORDER BY updated_at DESC LIMIT ?
			)
		`, profileID, profileID, MaxProgressItems)
	}

	for _, rmCh := range patch.RemovedChannelIDs {
		if rmCh != "" {
			_, _ = tx.ExecContext(ctx, `DELETE FROM profile_favorite_channels WHERE profile_id = ? AND channel_id = ?`, profileID, rmCh)
		}
	}

	if patch.FavoriteChannelIDs != nil {
		if patch.ReplaceFavorites {
			_, _ = tx.ExecContext(ctx, `DELETE FROM profile_favorite_channels WHERE profile_id = ?`, profileID)
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

		// Also preserve existing favorite channels for this profile unless explicitly removed or ReplaceFavorites is set
		if !patch.ReplaceFavorites {
			rows, err := tx.QueryContext(
				ctx,
				`SELECT channel_id FROM profile_favorite_channels WHERE profile_id = ? ORDER BY position_order ASC`,
				profileID,
			)
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

		_, _ = tx.ExecContext(ctx, `DELETE FROM profile_favorite_channels WHERE profile_id = ?`, profileID)
		for idx, chID := range cleanFavs {
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO profile_favorite_channels (profile_id, channel_id, position_order, updated_at)
				VALUES (?, ?, ?, ?)
			`, profileID, chID, idx, now)
		}
	}

	if err := tx.Commit(); err != nil {
		return HouseholdState{}, err
	}

	state, err := s.readStateLocked(ctx, profileID)
	if err != nil {
		return HouseholdState{}, err
	}
	state.UpdatedAt = now

	if profileID == "default" {
		if rawMirror, err := json.Marshal(state); err == nil {
			_ = os.WriteFile(filepath.Join(s.dataDir, "state.json"), rawMirror, 0o644)
		}
	}

	return state, nil
}
