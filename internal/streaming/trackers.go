package streaming

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
)

var trackerListURLs = []string{
	"https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_best.txt",
	"https://cdn.jsdelivr.net/gh/ngosang/trackerslist@master/trackers_best.txt",
}

const (
	trackerRefreshTTL = 24 * time.Hour
	maxTrackers       = 30
)

// FallbackTrackers is the built-in snapshot of public BitTorrent trackers.
var FallbackTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.stealth.si:80/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"udp://open.demonii.com:1337/announce",
	"udp://open.tracker.cl:1337/announce",
	"udp://tracker.qu.ax:6969/announce",
	"udp://tracker.ducks.party:1984/announce",
	"udp://tracker-udp.gbitt.info:80/announce",
	"udp://tracker.tryhackx.org:6969/announce",
	"udp://tracker.theoks.net:6969/announce",
	"udp://tracker.gmi.gd:6969/announce",
	"udp://explodie.org:6969/announce",
	"http://tracker.renfei.net:8080/announce",
	"http://tracker.dler.org:6969/announce",
}

var (
	reSupportedTracker = regexp.MustCompile(`(?i)^(udp|https?)://[^\s]+$`)
	reMagnetPrefix     = regexp.MustCompile(`(?i)^magnet:\?`)
	reBtihParam        = regexp.MustCompile(`(?i)[?&]xt=urn:btih:([0-9a-fA-F]{40}|[a-zA-Z2-7]{32})(?:&|$)`)

	trackerMu   sync.RWMutex
	cachedList  []string
	cachedAt    time.Time
)

func downloadTrackers(ctx context.Context) ([]string, error) {
	for _, u := range trackerListURLs {
		resp, err := netguard.GuardedFetch(ctx, u, netguard.FetchOptions{
			Timeout:      5 * time.Second,
			AllowPrivate: false,
		})
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}
		text, err := netguard.ReadBoundedText(resp, 512*1024)
		if err != nil {
			continue
		}
		fields := strings.Fields(text)
		var list []string
		for _, f := range fields {
			f = strings.TrimSpace(f)
			if reSupportedTracker.MatchString(f) {
				list = append(list, f)
			}
		}
		if len(list) >= 5 {
			return list, nil
		}
	}
	return nil, fmt.Errorf("tracker list unavailable")
}

// GetTrackers returns the current public trackers (cached for 24h; never fails).
func GetTrackers(ctx context.Context) []string {
	trackerMu.RLock()
	if len(cachedList) > 0 && time.Since(cachedAt) < trackerRefreshTTL {
		out := append([]string(nil), cachedList...)
		trackerMu.RUnlock()
		return out
	}
	trackerMu.RUnlock()

	list, err := downloadTrackers(ctx)
	trackerMu.Lock()
	defer trackerMu.Unlock()
	if err == nil {
		deduped := Dedupe(list)
		if len(deduped) > maxTrackers {
			deduped = deduped[:maxTrackers]
		}
		cachedList = deduped
		cachedAt = time.Now()
		return append([]string(nil), cachedList...)
	}
	if len(cachedList) == 0 {
		cachedList = append([]string(nil), FallbackTrackers...)
	}
	cachedAt = time.Now().Add(-trackerRefreshTTL + time.Hour)
	return append([]string(nil), cachedList...)
}

// KnownTrackers returns the synchronous best-known tracker list.
func KnownTrackers() []string {
	trackerMu.RLock()
	defer trackerMu.RUnlock()
	if len(cachedList) > 0 {
		return append([]string(nil), cachedList...)
	}
	return append([]string(nil), FallbackTrackers...)
}

// Dedupe deduplicates trackers ignoring case and trailing slashes.
func Dedupe(list []string) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, t := range list {
		key := strings.ToLower(strings.TrimRight(t, "/"))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

const base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

func base32ToHex(b32 string) string {
	var bits strings.Builder
	bits.Grow(len(b32) * 5)
	for _, ch := range strings.ToUpper(b32) {
		val := strings.IndexRune(base32Alphabet, ch)
		if val == -1 {
			return strings.ToLower(b32)
		}
		bits.WriteString(fmt.Sprintf("%05b", val))
	}
	bitStr := bits.String()
	var hex strings.Builder
	hex.Grow(40)
	for i := 0; i+4 <= len(bitStr); i += 4 {
		v, _ := strconv.ParseUint(bitStr[i:i+4], 2, 8)
		hex.WriteString(strconv.FormatUint(v, 16))
	}
	return hex.String()
}

// ExtractInfoHash extracts the canonical 40-char lowercase hex infoHash from a magnet URI, or "" if invalid.
func ExtractInfoHash(magnet string) string {
	trimmed := strings.TrimSpace(magnet)
	if !reMagnetPrefix.MatchString(trimmed) {
		return ""
	}
	m := reBtihParam.FindStringSubmatch(trimmed)
	if len(m) < 2 {
		return ""
	}
	raw := m[1]
	if len(raw) == 32 {
		return base32ToHex(raw)
	}
	return strings.ToLower(raw)
}

// MagnetTrackers extracts supported UDP/HTTP(S) trackers embedded in a magnet URI (`tr=` params).
func MagnetTrackers(magnet string) []string {
	q := strings.IndexByte(magnet, '?')
	if q == -1 {
		return []string{}
	}
	vals, err := url.ParseQuery(magnet[q+1:])
	if err != nil {
		return []string{}
	}
	trs := vals["tr"]
	out := make([]string, 0, len(trs))
	for _, t := range trs {
		if reSupportedTracker.MatchString(t) {
			out = append(out, t)
		}
	}
	return out
}

func encodeURIComponent(s string) string {
	escaped := url.QueryEscape(s)
	return strings.ReplaceAll(escaped, "+", "%20")
}

// BuildMagnet builds a magnet link carrying the lowercase infoHash, encoded title, and current tracker list.
func BuildMagnet(infoHash string, title string) string {
	trs := KnownTrackers()
	parts := make([]string, 0, len(trs))
	for _, t := range trs {
		parts = append(parts, "tr="+encodeURIComponent(t))
	}
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s&%s",
		strings.ToLower(infoHash),
		encodeURIComponent(title),
		strings.Join(parts, "&"),
	)
}
