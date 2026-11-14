package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/jvanheerikhuize/lumen/internal/state"
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

	// 2. PUT & GET /api/state
	putPayload := []byte(`{"watchProgress":[{"media":{"id":"tt15239678","title":"Dune: Part Two","mediaType":"movie","voteAverage":8.8,"genres":["Sci-Fi"]},"position":600,"duration":9960,"updatedAt":1700000000000}],"favoriteChannelIds":["dw-news-en"]}`)
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

	// 3. GET / serves embedded SPA index.html
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	srv.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusOK || !bytes.Contains(recRoot.Body.Bytes(), []byte("Lumen SPA")) {
		t.Fatalf("expected SPA index.html on /, got %d: %s", recRoot.Code, recRoot.Body.String())
	}
}
