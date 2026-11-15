package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/jvanheerikhuize/lumen/internal/state"
	"github.com/jvanheerikhuize/lumen/internal/types"
)

func TestServerHealthStateAndSPA(t *testing.T) {
	store, err := state.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	spaFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body>Lumen SPA</body></html>")},
	}

	srv, err := New(Config{
		SPAFS:      spaFS,
		StateStore: store,
	})
	if err != nil {
		t.Fatalf("New server failed: %v", err)
	}

	// 1. GET /api/health
	reqHealth := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recHealth := httptest.NewRecorder()
	srv.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/health, got %d", recHealth.Code)
	}
	var healthBody struct {
		Status string `json:"status"`
		Ffmpeg bool   `json:"ffmpeg"`
	}
	if err := json.Unmarshal(recHealth.Body.Bytes(), &healthBody); err != nil || healthBody.Status != "ok" {
		t.Fatalf("unexpected /api/health response: %s", recHealth.Body.String())
	}

	// 2. PUT & GET /api/state (including profile nicknames and isKids)
	putPayload := []byte(`{"profiles":[{"id":"default","name":"Member 1","nickname":"Sam","color":"amber","isKids":false},{"id":"kids","name":"Kids","nickname":"Kiddo","color":"emerald","isKids":true}],"watchProgress":[{"media":{"id":"tt15239678","title":"Dune: Part Two","mediaType":"movie","voteAverage":8.8,"genres":["Sci-Fi"]},"position":600,"duration":9960,"updatedAt":1700000000000}],"favoriteChannelIds":["dw-news-en"]}`)
	reqPut := httptest.NewRequest(http.MethodPut, "/api/state", bytes.NewReader(putPayload))
	recPut := httptest.NewRecorder()
	srv.ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT /api/state, got %d: %s", recPut.Code, recPut.Body.String())
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	recGet := httptest.NewRecorder()
	srv.ServeHTTP(recGet, reqGet)
	var gotState state.HouseholdState
	if err := json.Unmarshal(recGet.Body.Bytes(), &gotState); err != nil {
		t.Fatalf("failed to decode GET /api/state: %v", err)
	}
	if len(gotState.WatchProgress) != 1 || gotState.WatchProgress[0].Position != 600 || len(gotState.FavoriteChannelIDs) != 1 {
		t.Fatalf("unexpected state: %+v", gotState)
	}
	if len(gotState.Profiles) != 2 || gotState.Profiles[0].Nickname != "Sam" || !gotState.Profiles[1].IsKids || gotState.Profiles[1].Nickname != "Kiddo" {
		t.Fatalf("unexpected profiles: %+v", gotState.Profiles)
	}

	// 3. Kids catalog rejects mature genre requests immediately
	reqKidsHorror := httptest.NewRequest(http.MethodGet, "/api/catalog?type=movie&catalog=top&genre=Horror&kids=1", nil)
	recKidsHorror := httptest.NewRecorder()
	srv.ServeHTTP(recKidsHorror, reqKidsHorror)
	if recKidsHorror.Code != http.StatusOK {
		t.Fatalf("expected 200 on kids Horror catalog, got %d", recKidsHorror.Code)
	}
	var kidsCatalog struct {
		Items  []types.MediaItem `json:"items"`
		Genres []string          `json:"genres"`
	}
	if err := json.Unmarshal(recKidsHorror.Body.Bytes(), &kidsCatalog); err != nil {
		t.Fatalf("failed to decode kids catalog: %v", err)
	}
	if len(kidsCatalog.Items) != 0 {
		t.Fatalf("expected 0 items for Horror in kids mode, got %d", len(kidsCatalog.Items))
	}
	for _, g := range kidsCatalog.Genres {
		if g == "Horror" || g == "Crime" || g == "Thriller" {
			t.Fatalf("unexpected mature genre %q in kids catalog genres: %v", g, kidsCatalog.Genres)
		}
	}

	// 4. GET / serves embedded SPA index.html
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	srv.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusOK || !bytes.Contains(recRoot.Body.Bytes(), []byte("Lumen SPA")) {
		t.Fatalf("expected SPA index.html on /, got %d: %s", recRoot.Code, recRoot.Body.String())
	}
}

func TestIsKidsSafeMedia(t *testing.T) {
	safeMovie := types.MediaItem{
		ID:     "tt0245429",
		Title:  "Spirited Away",
		Genres: []string{"Animation", "Adventure", "Family", "Fantasy"},
	}
	if !IsKidsSafeMedia(safeMovie) {
		t.Fatalf("expected %s to be safe for kids", safeMovie.Title)
	}

	safeAnime := types.MediaItem{
		ID:     "anilist-20958",
		Title:  "Haikyuu!!",
		Genres: []string{"Comedy", "Sports"},
		Anime:  &types.AnimeRef{AnilistID: 20583},
	}
	if !IsKidsSafeMedia(safeAnime) {
		t.Fatalf("expected %s to be safe for kids anime", safeAnime.Title)
	}

	unsafeHorror := types.MediaItem{
		ID:     "tt0387564",
		Title:  "Saw",
		Genres: []string{"Horror", "Mystery", "Thriller"},
	}
	if IsKidsSafeMedia(unsafeHorror) {
		t.Fatalf("expected %s to be blocked for kids", unsafeHorror.Title)
	}

	unsafeFamilyThriller := types.MediaItem{
		ID:     "tt9999",
		Title:  "Family Thriller",
		Genres: []string{"Family", "Thriller"},
	}
	if IsKidsSafeMedia(unsafeFamilyThriller) {
		t.Fatalf("expected mixed Family+Thriller item to be blocked")
	}

	adultDrama := types.MediaItem{
		ID:     "tt15398776",
		Title:  "Oppenheimer",
		Genres: []string{"Drama", "History", "Biography"},
	}
	if IsKidsSafeMedia(adultDrama) {
		t.Fatalf("expected adult historical drama without Family/Animation to be blocked")
	}

	adultDoc := types.MediaItem{
		ID:     "tt14720536",
		Title:  "Bo Burnham: Inside",
		Genres: []string{"Documentary", "Comedy", "Drama"},
	}
	if IsKidsSafeMedia(adultDoc) {
		t.Fatalf("expected adult documentary special without Family/Kids to be blocked")
	}

	actionAnimation := types.MediaItem{
		ID:     "tt2560140",
		Title:  "Attack on Titan",
		Genres: []string{"Animation", "Action", "Adventure", "Drama", "Fantasy"},
	}
	if IsKidsSafeMedia(actionAnimation) {
		t.Fatalf("expected action/drama animation without Family/Kids to be blocked")
	}

	adultAnimatedComedy := types.MediaItem{
		ID:     "tt2861424",
		Title:  "Rick and Morty",
		Genres: []string{"Animation", "Adventure", "Comedy", "Sci-Fi"},
	}
	if IsKidsSafeMedia(adultAnimatedComedy) {
		t.Fatalf("expected adult animated comedy without Family/Kids to be blocked")
	}
}
