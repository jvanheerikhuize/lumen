package scrapers

import (
	"net/url"
	"slices"
	"strings"
)

// DefaultAddonURLs is the built-in list of public Stremio stream addons.
var DefaultAddonURLs = []string{
	"https://torrentio.strem.fun",
	"https://mediafusion.elfhosted.com",
}

var retiredAddonHosts = []string{
	"knightcrawler.elfhosted.com",
}

// CleanAddonURLs keeps valid http(s) addon URLs (max 10, without trailing slashes or /manifest.json)
// and drops retired public instances; falls back to DefaultAddonURLs if nothing is left.
func CleanAddonURLs(urls []string) []string {
	var kept []string
	for _, raw := range urls {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		u, err := url.Parse(trimmed)
		if err != nil {
			continue
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue
		}
		if slices.Contains(retiredAddonHosts, strings.ToLower(u.Hostname())) {
			continue
		}
		clean := u.String()
		clean = strings.TrimSuffix(clean, "/manifest.json")
		clean = strings.TrimRight(clean, "/")
		if !slices.Contains(kept, clean) {
			kept = append(kept, clean)
		}
	}
	if len(kept) == 0 {
		out := make([]string, len(DefaultAddonURLs))
		copy(out, DefaultAddonURLs)
		return out
	}
	if len(kept) > 10 {
		kept = kept[:10]
	}
	return kept
}

// IsDefaultAddonList reports whether urls matches DefaultAddonURLs exactly.
func IsDefaultAddonList(urls []string) bool {
	if len(urls) != len(DefaultAddonURLs) {
		return false
	}
	for i := range urls {
		if urls[i] != DefaultAddonURLs[i] {
			return false
		}
	}
	return true
}
