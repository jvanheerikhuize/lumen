package streaming

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
)

// ResolvePlaybackInput resolves the media input ffmpeg/ffprobe should read for a playback request:
//   ?magnet=...&fileIndex=N  -> local loopback range-capable torrent endpoint http://127.0.0.1:<port>/api/stream?...
//   ?url=https://...         -> a direct HTTP(S) source on the public internet
func ResolvePlaybackInput(ctx context.Context, r *http.Request) string {
	return ResolvePlaybackInputWithLookup(ctx, r, nil)
}

// ResolvePlaybackInputWithLookup is like ResolvePlaybackInput but allows injecting a DNS LookupHostFunc for testing.
func ResolvePlaybackInputWithLookup(ctx context.Context, r *http.Request, lookup netguard.LookupHostFunc) string {
	q := r.URL.Query()
	magnet := q.Get("magnet")
	rawURL := q.Get("url")

	if magnet != "" {
		if ExtractInfoHash(magnet) == "" {
			return ""
		}
		fileIndex := q.Get("fileIndex")
		qs := "magnet=" + encodeURIComponent(strings.TrimSpace(magnet))
		if fileIndex != "" {
			qs += "&fileIndex=" + encodeURIComponent(fileIndex)
		}
		localPort := os.Getenv("LUMEN_INTERNAL_PORT")
		if localPort == "" {
			localPort = os.Getenv("SCREENTIME_INTERNAL_PORT")
		}
		if localPort == "" {
			localPort = os.Getenv("PORT")
		}
		if localPort == "" && r.URL.Port() != "" {
			localPort = r.URL.Port()
		}
		if localPort == "" {
			localPort = "3000"
		}
		return fmt.Sprintf("http://127.0.0.1:%s/api/stream?%s", localPort, qs)
	}

	if rawURL != "" {
		if netguard.IsAllowedURLWithLookup(ctx, rawURL, false, lookup) {
			if parsed, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
				return parsed.String()
			}
		}
	}
	return ""
}
