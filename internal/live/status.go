package live

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
)

const (
	StatusCacheTTL    = 15 * time.Minute // 15m TTL per REQ-0010
	MaxStatusURLs     = 24               // Max 24 URLs per batch per REQ-0010
	StatusConcurrency = 8                // Concurrency 8 per REQ-0010
	StatusTimeout     = 6 * time.Second  // 6s timeout per REQ-0010
)

var reMP4Ext = regexp.MustCompile(`(?i)\.mp4(\?|$)`)

type statusCacheEntry struct {
	status string // "ok" | "offline"
	at     time.Time
}

var (
	statusMu       sync.Mutex
	statusCache    = make(map[string]statusCacheEntry)
	knownStreamSet = make(map[string]bool)
)

// RegisterKnownStreamURL registers a stream URL as known for /api/live/status probing (used in tests).
func RegisterKnownStreamURL(u string) {
	statusMu.Lock()
	defer statusMu.Unlock()
	knownStreamSet[u] = true
}

func playlistURIs(playlist string) []string {
	raw := strings.ReplaceAll(playlist, "\r\n", "\n")
	var out []string
	for _, l := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			out = append(out, trimmed)
		}
	}
	return out
}

func fetchPlaylistForStatus(ctx context.Context, targetURL string, lookup netguard.LookupHostFunc) (string, string, bool) {
	resp, err := netguard.GuardedFetch(ctx, targetURL, netguard.FetchOptions{
		Timeout:      StatusTimeout,
		AllowPrivate: false,
		Headers:      map[string]string{"User-Agent": "Mozilla/5.0 Screentime/1.0"},
		Lookup:       lookup,
	})
	if err != nil {
		return "", "", false
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return "", "", false
	}
	finalURL := targetURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	text, err := netguard.ReadBoundedText(resp, 2*1024*1024)
	if err != nil {
		return "", "", false
	}
	if !strings.HasPrefix(strings.TrimLeft(text, " \t\r\n\uFEFF"), "#EXTM3U") {
		return "", "", false
	}
	return text, finalURL, true
}

func segmentAnswers(ctx context.Context, segURL string, lookup netguard.LookupHostFunc) bool {
	resp, err := netguard.GuardedFetch(ctx, segURL, netguard.FetchOptions{
		Timeout:      StatusTimeout,
		AllowPrivate: false,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 Screentime/1.0",
			"Range":      "bytes=0-1023",
		},
		Lookup: lookup,
	})
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func resolveRef(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := b.Parse(ref)
	if err != nil {
		return ref
	}
	return r.String()
}

// CheckStreamStatus probes a single stream URL with 15m caching.
func CheckStreamStatus(ctx context.Context, streamURL string, lookup netguard.LookupHostFunc) string {
	statusMu.Lock()
	if hit, ok := statusCache[streamURL]; ok && time.Since(hit.at) < StatusCacheTTL {
		st := hit.status
		statusMu.Unlock()
		return st
	}
	statusMu.Unlock()

	status := "offline"
	if reMP4Ext.MatchString(streamURL) {
		resp, err := netguard.GuardedFetch(ctx, streamURL, netguard.FetchOptions{
			Method:       http.MethodHead,
			Timeout:      StatusTimeout,
			AllowPrivate: false,
			Lookup:       lookup,
		})
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				status = "ok"
			}
		}
	} else {
		masterText, masterURL, ok := fetchPlaylistForStatus(ctx, streamURL, lookup)
		if ok {
			mediaText, mediaURL := masterText, masterURL
			if strings.Contains(masterText, "#EXT-X-STREAM-INF") {
				refs := playlistURIs(masterText)
				if len(refs) > 0 {
					var vOk bool
					mediaText, mediaURL, vOk = fetchPlaylistForStatus(ctx, resolveRef(masterURL, refs[0]), lookup)
					if !vOk {
						mediaText = ""
					}
				}
			}
			if mediaText != "" {
				segs := playlistURIs(mediaText)
				if len(segs) == 0 {
					status = "ok"
				} else {
					newestSeg := resolveRef(mediaURL, segs[len(segs)-1])
					if segmentAnswers(ctx, newestSeg, lookup) {
						status = "ok"
					}
				}
			}
		}
	}

	statusMu.Lock()
	if len(statusCache) > 5000 {
		statusCache = make(map[string]statusCacheEntry)
	}
	statusCache[streamURL] = statusCacheEntry{status: status, at: time.Now()}
	statusMu.Unlock()
	return status
}

// CheckBatchStreamStatus checks up to MaxStatusURLs (24) known channel URLs with concurrency 8.
func CheckBatchStreamStatus(ctx context.Context, requestedURLs []string, lookup netguard.LookupHostFunc) map[string]string {
	all := GetAllChannels(ctx)
	known := make(map[string]bool, len(all)+len(LiveChannels))
	for _, c := range all {
		known[c.StreamURL] = true
	}
	for _, c := range LiveChannels {
		known[c.StreamURL] = true
	}
	statusMu.Lock()
	for k := range knownStreamSet {
		known[k] = true
	}
	statusMu.Unlock()

	var filtered []string
	for _, u := range requestedURLs {
		if known[u] {
			filtered = append(filtered, u)
			if len(filtered) >= MaxStatusURLs {
				break
			}
		}
	}

	result := make(map[string]string, len(filtered))
	if len(filtered) == 0 {
		return result
	}

	var (
		mu   sync.Mutex
		next int
		wg   sync.WaitGroup
	)
	workers := min(StatusConcurrency, len(filtered))
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if next >= len(filtered) {
					mu.Unlock()
					return
				}
				u := filtered[next]
				next++
				mu.Unlock()

				st := CheckStreamStatus(ctx, u, lookup)
				mu.Lock()
				result[u] = st
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return result
}
