package state

import (
	"context"
	"sync"
	"testing"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

func makeMedia(id, title, mediaType string) types.MediaItem {
	return types.MediaItem{
		ID:          id,
		ImdbID:      id,
		Title:       title,
		Overview:    title + " overview",
		PosterPath:  nil,
		VoteAverage: 8.0,
		MediaType:   mediaType,
		Genres:      []string{"Sci-Fi"},
	}
}

func TestSaveWatchProgressMovieCompletionAndSeriesMemory(t *testing.T) {
	movie := makeMedia("tt100", "Interstellar", "movie")
	var list []WatchProgressEntry

	list = SaveWatchProgress(list, movie, 1200, 7200, nil, nil)
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if got := GetMediaProgress("tt100", list); got == nil || got.Position != 1200 {
		t.Fatalf("expected position 1200, got %+v", got)
	}

	// Finishing the movie (>= 94%) removes it from Continue Watching
	list = SaveWatchProgress(list, movie, 7000, 7200, nil, nil)
	if len(list) != 0 || GetMediaProgress("tt100", list) != nil {
		t.Fatalf("expected finished movie to be removed, got %+v", list)
	}

	// Series per-episode resume memory
	show := makeMedia("tt200", "Severance", "tv")
	s1, e2, e3 := 1, 2, 3
	list = SaveWatchProgress(list, show, 1400, 2800, &s1, &e2)
	list = SaveWatchProgress(list, show, 600, 2900, &s1, &e3)

	entry := GetMediaProgress("tt200", list)
	if entry == nil || entry.Season == nil || *entry.Season != 1 || entry.Episode == nil || *entry.Episode != 3 {
		t.Fatalf("unexpected series entry: %+v", entry)
	}
	if ep2 := GetEpisodeProgress("tt200", 1, 2, list); ep2 == nil || ep2.Position != 1400 {
		t.Fatalf("expected episode 2 position 1400, got %+v", ep2)
	}
	if ep3 := GetEpisodeProgress("tt200", 1, 3, list); ep3 == nil || ep3.Position != 600 {
		t.Fatalf("expected episode 3 position 600, got %+v", ep3)
	}

	list = RemoveWatchProgress(list, "tt200")
	if len(list) != 0 {
		t.Fatalf("expected empty list after RemoveWatchProgress")
	}
}

func TestSQLiteStoreConcurrentHouseholdSync(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Device A has Movie 1 at t=200 (pos=1800)
	// Device B has stale Movie 1 at t=100 (pos=500) AND new Movie 2 at t=250 (pos=900)
	// Concurrent updates must preserve Movie 1 at pos=1800 (t=200) AND Movie 2 at pos=900 (t=250)!
	deviceAPatch := []WatchProgressEntry{
		{
			Media:     makeMedia("tt001", "Dune", "movie"),
			Position:  1800,
			Duration:  7200,
			UpdatedAt: 200,
		},
	}
	deviceBPatch := []WatchProgressEntry{
		{
			Media:     makeMedia("tt001", "Dune", "movie"),
			Position:  500,
			Duration:  7200,
			UpdatedAt: 100,
		},
		{
			Media:     makeMedia("tt002", "Arrival", "movie"),
			Position:  900,
			Duration:  6800,
			UpdatedAt: 250,
		},
	}
	favA := []string{"ch-nasa"}
	favB := []string{"ch-dw"}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = store.UpdateState(context.Background(), StatePatch{
			WatchProgress:      &deviceAPatch,
			FavoriteChannelIDs: &favA,
		})
	}()
	go func() {
		defer wg.Done()
		_, _ = store.UpdateState(context.Background(), StatePatch{
			WatchProgress:      &deviceBPatch,
			FavoriteChannelIDs: &favB,
		})
	}()
	wg.Wait()

	state, err := store.GetState(context.Background())
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if len(state.WatchProgress) != 2 {
		t.Fatalf("expected 2 merged watchProgress entries, got %d: %+v", len(state.WatchProgress), state.WatchProgress)
	}
	m1 := GetMediaProgress("tt001", state.WatchProgress)
	if m1 == nil || m1.Position != 1800 || m1.UpdatedAt != 200 {
		t.Fatalf("expected newer timestamp (pos=1800, t=200) to win for tt001, got %+v", m1)
	}
	m2 := GetMediaProgress("tt002", state.WatchProgress)
	if m2 == nil || m2.Position != 900 || m2.UpdatedAt != 250 {
		t.Fatalf("expected tt002 (pos=900, t=250) to be preserved, got %+v", m2)
	}
	if len(state.FavoriteChannelIDs) != 2 {
		t.Fatalf("expected both favorite channels to be preserved, got %v", state.FavoriteChannelIDs)
	}
}
