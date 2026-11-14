package scrapers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCleanAddonURLs(t *testing.T) {
	got := CleanAddonURLs([]string{
		"https://torrentio.strem.fun/manifest.json",
		"https://torrentio.strem.fun/",
		"https://knightcrawler.elfhosted.com",
		"ftp://example.com",
		"nonsense",
		"https://addon.example.com/config/",
	})
	want := []string{"https://torrentio.strem.fun", "https://addon.example.com/config"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	if !reflect.DeepEqual(CleanAddonURLs(nil), DefaultAddonURLs) {
		t.Fatalf("expected fallback to DefaultAddonURLs on nil")
	}
	if !reflect.DeepEqual(CleanAddonURLs([]string{"https://knightcrawler.elfhosted.com"}), DefaultAddonURLs) {
		t.Fatalf("expected fallback to DefaultAddonURLs when only retired host provided")
	}

	var many []string
	for i := 0; i < 15; i++ {
		many = append(many, fmt.Sprintf("https://addon%d.example.com", i))
	}
	if len(CleanAddonURLs(many)) != 10 {
		t.Fatalf("expected cap at 10 addons")
	}

	if !IsDefaultAddonList(DefaultAddonURLs) {
		t.Fatalf("expected IsDefaultAddonList(DefaultAddonURLs) == true")
	}
	if IsDefaultAddonList([]string{DefaultAddonURLs[0]}) {
		t.Fatalf("expected IsDefaultAddonList(single) == false")
	}
}

func TestREQ0012_ProwlarrAndJackettSearchAndHealthCheck(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/system/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.18.0"}`))
	})
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"title": "Dune.Part.Two.2024.1080p.WEB-DL.x264",
				"size": 2576980377,
				"seeders": 95,
				"leechers": 12,
				"indexer": "1337x",
				"infoHash": "0123456789abcdef0123456789abcdef01234567"
			}
		]`))
	})
	mux.HandleFunc("/api/v2.0/indexers/all/results/torznab/api", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") == "caps" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<caps/>`))
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel>
    <item>
      <title>Dune.Part.Two.2024.720p.WEB-DL.x264</title>
      <jackettindexer>RARBG</jackettindexer>
      <torznab:attr name="size" value="1503238553" />
      <torznab:attr name="seeders" value="60" />
      <torznab:attr name="peers" value="8" />
      <torznab:attr name="infohash" value="abcdef0123456789abcdef0123456789abcdef01" />
    </item>
  </channel>
</rss>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Health checks
	resProwlarr := TestConnection(context.Background(), "prowlarr", srv.URL, "secret-key", nil)
	if !resProwlarr.Success || resProwlarr.Message != "Connected to Prowlarr v1.18.0" {
		t.Fatalf("unexpected prowlarr test result: %+v", resProwlarr)
	}
	resJackett := TestConnection(context.Background(), "jackett", srv.URL, "secret-key", nil)
	if !resJackett.Success {
		t.Fatalf("unexpected jackett test result: %+v", resJackett)
	}

	// Blocked link-local metadata address must be rejected by TestConnection
	resBlocked := TestConnection(context.Background(), "prowlarr", "http://169.254.169.254", "key", nil)
	if resBlocked.Success {
		t.Fatalf("expected link-local address to be rejected")
	}

	// Search Prowlarr & Jackett
	pSources := SearchProwlarr(context.Background(), ProwlarrSearchParams{
		HostURL:   srv.URL,
		APIKey:    "secret-key",
		Query:     "Dune Part Two",
		MediaType: "movie",
	})
	if len(pSources) != 1 || pSources[0].Quality != "1080p" || pSources[0].Codec != "x264" || pSources[0].Seeds == nil || *pSources[0].Seeds != 95 {
		t.Fatalf("unexpected prowlarr sources: %+v", pSources)
	}

	jSources := SearchJackett(context.Background(), JackettSearchParams{
		HostURL: srv.URL,
		APIKey:  "secret-key",
		Query:   "Dune Part Two",
		IMDbID:  "tt15239678",
	})
	if len(jSources) != 1 || jSources[0].Quality != "720p" || jSources[0].Seeds == nil || *jSources[0].Seeds != 60 {
		t.Fatalf("unexpected jackett sources: %+v", jSources)
	}
}
