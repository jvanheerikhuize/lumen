package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sampleFreeTVM3U = `#EXTM3U
#EXTINF:-1 tvg-logo="https://logo/dw.png" group-title="Germany",DW Ⓖ
https://example.com/dw/master.m3u8
#EXTINF:-1 group-title="Germany",Duplicate URL
https://example.com/dw/master.m3u8
#EXTINF:-1 group-title="News",Global News Ⓢ
https://example.com/news.m3u8?token=1
#EXTINF:-1 group-title="Netherlands",YouTube channel Ⓨ
https://www.youtube.com/watch?v=abc
#EXTINF:-1 group-title="VOD Movies",Some film
https://example.com/film.mp4
#EXTINF:-1 group-title="United Kingdom",Not a stream
https://example.com/page.html
`

func TestParseFreeTVPlaylist(t *testing.T) {
	channels := ParseFreeTVPlaylist(sampleFreeTVM3U)
	var names []string
	for _, c := range channels {
		names = append(names, c.Name)
	}
	wantNames := []string{"DW", "Global News"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("expected %v, got %v", wantNames, names)
	}

	dw, news := channels[0], channels[1]
	if dw.CountryCode != "DE" || dw.Logo != "https://logo/dw.png" || !dw.GeoBlocked || dw.PartTime || dw.Source != "freetv" {
		t.Fatalf("unexpected DW channel fields: %+v", dw)
	}
	if news.Category != "News" || news.Country != "International" || news.CountryCode != "" || !news.PartTime {
		t.Fatalf("unexpected Global News channel fields: %+v", news)
	}
}

func TestRewritePlaylist(t *testing.T) {
	master := strings.Join([]string{
		"#EXTM3U",
		`#EXT-X-KEY:METHOD=AES-128,URI="keys/k1.key"`,
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aud",URI="audio/en.m3u8"`,
		"#EXT-X-STREAM-INF:BANDWIDTH=800000",
		"low/index.m3u8",
		"",
		"https://cdn.example.org/seg1.ts",
	}, "\n")

	enc := func(s string) string {
		return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
	}
	out := RewritePlaylist(master, "https://live.example.com/ch/master.m3u8", func(u string) string {
		return "/relay?u=" + enc(u)
	})
	lines := strings.Split(out, "\n")
	if lines[0] != "#EXTM3U" {
		t.Fatalf("unexpected line 0: %s", lines[0])
	}
	wantKey := `#EXT-X-KEY:METHOD=AES-128,URI="/relay?u=` + enc("https://live.example.com/ch/keys/k1.key") + `"`
	if lines[1] != wantKey {
		t.Fatalf("expected %s, got %s", wantKey, lines[1])
	}
	if !strings.Contains(lines[2], enc("https://live.example.com/ch/audio/en.m3u8")) {
		t.Fatalf("expected audio URI to be rewritten: %s", lines[2])
	}
	if lines[3] != "#EXT-X-STREAM-INF:BANDWIDTH=800000" {
		t.Fatalf("unexpected line 3: %s", lines[3])
	}
	if lines[4] != "/relay?u="+enc("https://live.example.com/ch/low/index.m3u8") {
		t.Fatalf("unexpected line 4: %s", lines[4])
	}
	if lines[5] != "" {
		t.Fatalf("expected empty line 5, got %s", lines[5])
	}
	if lines[6] != "/relay?u="+enc("https://cdn.example.org/seg1.ts") {
		t.Fatalf("unexpected line 6: %s", lines[6])
	}
}

func TestREQ0010_ServeLiveProxyRewritesManifestAndSetsCORS(t *testing.T) {
	// Verify relayPlaylist rewrites relative segment and key URIs to /api/live/proxy?url=...
	manifest := "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\nseg01.ts\n"
	rewritten := relayPlaylist(manifest, "https://stream.example.com/live/index.m3u8")
	if !strings.Contains(rewritten, `/api/live/proxy?url=https%3A%2F%2Fstream.example.com%2Flive%2Fkey.bin`) {
		t.Fatalf("expected key URI rewritten to /api/live/proxy?url=..., got:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, `/api/live/proxy?url=https%3A%2F%2Fstream.example.com%2Flive%2Fseg01.ts`) {
		t.Fatalf("expected segment URI rewritten to /api/live/proxy?url=..., got:\n%s", rewritten)
	}

	// Verify ServeLiveProxy rejects unallowed hosts and sets Access-Control-Allow-Origin: *
	req := httptest.NewRequest(http.MethodGet, "/api/live/proxy?url=https://unlisted.example.org/live.m3u8", nil)
	rec := httptest.NewRecorder()
	ServeLiveProxy(rec, req, func(ctx context.Context, host string) ([]string, error) {
		return []string{"93.184.216.34"}, nil
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unlisted host, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin: *")
	}
}

func TestCuratedLiveChannelsHaveCountryAttribution(t *testing.T) {
	if len(LiveChannels) == 0 {
		t.Fatalf("expected curated LiveChannels to be non-empty")
	}
	wantCode := map[string]string{
		"dw-news-en":   "DE",
		"nasa-tv":      "US",
		"france24-en":  "FR",
		"nhk-world":    "JP",
		"trt-world":    "TR",
		"redbull-tv":   "",
		"cgtv-doc":     "",
	}
	for _, ch := range LiveChannels {
		if ch.Country == "" {
			t.Fatalf("channel %s is missing Country", ch.ID)
		}
		if want, ok := wantCode[ch.ID]; ok && ch.CountryCode != want {
			t.Fatalf("channel %s expected CountryCode=%q, got %q", ch.ID, want, ch.CountryCode)
		}
		if ch.ID == "redbull-tv" && strings.Contains(ch.Logo, "7c2K3ad") {
			t.Fatalf("expected broken Red Bull Imgur logo to be removed, got %q", ch.Logo)
		}
	}
}

func TestParseFreeTVPlaylistKidsCategory(t *testing.T) {
	m3u := strings.Join([]string{
		"#EXTM3U",
		`#EXTINF:-1 group-title="United Kingdom",CBBC`,
		"https://example.com/cbbc.m3u8",
		`#EXTINF:-1 group-title="Kids",Duck TV`,
		"https://example.com/ducktv.m3u8",
		`#EXTINF:-1 group-title="Germany",KiKA`,
		"https://example.com/kika.m3u8",
		`#EXTINF:-1 group-title="Netherlands",NOS Journaal`,
		"https://example.com/nos.m3u8",
	}, "\n")

	channels := ParseFreeTVPlaylist(m3u)
	if len(channels) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(channels))
	}
	if channels[0].Category != "Kids" || channels[0].CountryCode != "GB" {
		t.Fatalf("expected CBBC to be Kids/GB, got %+v", channels[0])
	}
	if channels[1].Category != "Kids" || channels[1].Country != "International" {
		t.Fatalf("expected Duck TV to be Kids/International, got %+v", channels[1])
	}
	if channels[2].Category != "Kids" || channels[2].CountryCode != "DE" {
		t.Fatalf("expected KiKA to be Kids/DE, got %+v", channels[2])
	}
	if channels[3].Category != "General" || channels[3].CountryCode != "NL" {
		t.Fatalf("expected NOS Journaal to remain General/NL, got %+v", channels[3])
	}
}

func TestGetAllChannelsReturnsFreeTVDirectly(t *testing.T) {
	freeTVMu.Lock()
	prevCache := freeTVCache
	prevAt := freeTVAt
	freeTVCache = ParseFreeTVPlaylist(sampleFreeTVM3U)
	freeTVAt = time.Now()
	freeTVMu.Unlock()
	defer func() {
		freeTVMu.Lock()
		freeTVCache = prevCache
		freeTVAt = prevAt
		freeTVMu.Unlock()
	}()

	channels := GetAllChannels(context.Background())
	if len(channels) != 2 {
		t.Fatalf("expected 2 Free-TV channels directly without prepended curated channels, got %d (%+v)", len(channels), channels)
	}
	if channels[0].Source != "freetv" || channels[0].Name != "DW" {
		t.Fatalf("expected first channel to be Free-TV DW, got %+v", channels[0])
	}

	// Clearing cache falls back to curated LiveChannels
	freeTVMu.Lock()
	freeTVCache = nil
	freeTVAt = time.Time{}
	freeTVMu.Unlock()
	origURLs := freeTVPlaylistURLs
	freeTVPlaylistURLs = nil
	defer func() { freeTVPlaylistURLs = origURLs }()

	fallback := GetAllChannels(context.Background())
	if len(fallback) != len(LiveChannels) || fallback[0].Source != "curated" {
		t.Fatalf("expected curated fallback when Free-TV is unavailable, got %d channels", len(fallback))
	}
}


