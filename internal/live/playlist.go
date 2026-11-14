package live

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
)

var (
	reURIAttribute = regexp.MustCompile(`URI="([^"]+)"`)
	reM3U8Path     = regexp.MustCompile(`(?i)\.m3u8?(\?|$)`)
	reMpegURL      = regexp.MustCompile(`(?i)mpegurl`)
)

// RewritePlaylist rewrites every reference in an HLS playlist (variant playlists, segments, and URI="..." attributes)
// to absolute URLs passed through mapFn.
func RewritePlaylist(text string, base string, mapFn func(absoluteURL string) string) string {
	baseURL, err := url.Parse(base)
	abs := func(ref string) string {
		if err != nil || baseURL == nil {
			return mapFn(ref)
		}
		resolved, parseErr := baseURL.Parse(ref)
		if parseErr != nil {
			return mapFn(ref)
		}
		return mapFn(resolved.String())
	}

	raw := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out[i] = line
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			out[i] = reURIAttribute.ReplaceAllStringFunc(line, func(m string) string {
				sub := reURIAttribute.FindStringSubmatch(m)
				if len(sub) < 2 {
					return m
				}
				return `URI="` + abs(sub[1]) + `"`
			})
			continue
		}
		out[i] = abs(trimmed)
	}
	return strings.Join(out, "\n")
}

// RelayProxyPath is the canonical proxy path prefix for rewritten HLS playlist references (REQ-0010).
const RelayProxyPath = "/api/live/proxy?url="

var (
	hostAllowMu  sync.Mutex
	learnedHosts = make(map[string]bool)
	channelHosts map[string]bool
	channelAt    time.Time
)

// RegisterAllowedHost registers a host as permitted for the live HLS relay (used in tests and dynamic discovery).
func RegisterAllowedHost(hostname string) {
	hostAllowMu.Lock()
	defer hostAllowMu.Unlock()
	if len(learnedHosts) > 2000 {
		learnedHosts = make(map[string]bool)
	}
	learnedHosts[strings.ToLower(hostname)] = true
}

func isHostAllowedForProxy(ctx context.Context, hostname string) bool {
	h := strings.ToLower(hostname)
	hostAllowMu.Lock()
	if learnedHosts[h] {
		hostAllowMu.Unlock()
		return true
	}
	if channelHosts != nil && time.Since(channelAt) < time.Hour {
		ok := channelHosts[h]
		hostAllowMu.Unlock()
		return ok
	}
	hostAllowMu.Unlock()

	all := GetAllChannels(ctx)
	hosts := make(map[string]bool, len(all))
	for _, c := range all {
		if u, err := url.Parse(c.StreamURL); err == nil && u.Hostname() != "" {
			hosts[strings.ToLower(u.Hostname())] = true
		}
	}

	hostAllowMu.Lock()
	channelHosts = hosts
	channelAt = time.Now()
	ok := channelHosts[h] || learnedHosts[h]
	hostAllowMu.Unlock()
	return ok
}

func relayPlaylist(text, base string) string {
	return RewritePlaylist(text, base, func(absURL string) string {
		if u, err := url.Parse(absURL); err == nil && u.Hostname() != "" {
			RegisterAllowedHost(u.Hostname())
		}
		escaped := strings.ReplaceAll(url.QueryEscape(absURL), "+", "%20")
		return RelayProxyPath + escaped
	})
}

// ServeLiveProxy handles GET /api/live/proxy?url=<url> (and ?u=<url> for backwards compatibility),
// relaying HLS manifests (rewriting relative/attribute URIs) and segments with Access-Control-Allow-Origin: *.
func ServeLiveProxy(w http.ResponseWriter, r *http.Request, lookup netguard.LookupHostFunc) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	q := r.URL.Query()
	target := strings.TrimSpace(q.Get("url"))
	if target == "" {
		target = strings.TrimSpace(q.Get("u"))
	}
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		http.Error(w, "Bad url", http.StatusBadRequest)
		return
	}
	if !isHostAllowedForProxy(r.Context(), parsed.Hostname()) {
		http.Error(w, "Host not allowed", http.StatusForbidden)
		return
	}

	upstream, err := netguard.GuardedFetch(r.Context(), parsed.String(), netguard.FetchOptions{
		Timeout:      15 * time.Second,
		AllowPrivate: false,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) Screentime/1.0",
		},
		Lookup: lookup,
	})
	if err != nil {
		http.Error(w, "Upstream unreachable", http.StatusBadGateway)
		return
	}
	defer func() { _ = upstream.Body.Close() }()

	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		code := http.StatusBadGateway
		if upstream.StatusCode == http.StatusNotFound {
			code = http.StatusNotFound
		}
		http.Error(w, "Upstream error", code)
		return
	}

	cType := upstream.Header.Get("Content-Type")
	finalURL := parsed.String()
	if upstream.Request != nil && upstream.Request.URL != nil {
		finalURL = upstream.Request.URL.String()
	}

	looksLikePlaylist := reMpegURL.MatchString(cType) || reM3U8Path.MatchString(finalURL)
	if looksLikePlaylist {
		text, err := netguard.ReadBoundedText(upstream, 2*1024*1024)
		if err != nil {
			http.Error(w, "Upstream playlist too large or unreadable", http.StatusBadGateway)
			return
		}
		if strings.HasPrefix(strings.TrimLeft(text, " \t\r\n\uFEFF"), "#EXTM3U") {
			rewritten := relayPlaylist(text, finalURL)
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, rewritten)
			return
		}
		if cType == "" {
			cType = "text/plain"
		}
		w.Header().Set("Content-Type", cType)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, text)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	if cType != "" {
		w.Header().Set("Content-Type", cType)
	}
	if cl := upstream.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, upstream.Body)
}
