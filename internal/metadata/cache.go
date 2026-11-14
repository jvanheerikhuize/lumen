package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	defaultCacheTTL = 30 * time.Minute
	maxCacheEntries = 500
)

type cacheEntry struct {
	expires time.Time
	rawJSON []byte
}

var (
	cacheMu    sync.RWMutex
	cacheStore = make(map[string]cacheEntry)
	cacheOrder []string
	httpClient = &http.Client{Timeout: 15 * time.Second}
)

// CachedJSON fetches JSON from url and caches the raw bytes for 30 minutes.
func CachedJSON[T any](ctx context.Context, url string, timeout time.Duration) (*T, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	cacheMu.RLock()
	if hit, ok := cacheStore[url]; ok && time.Now().Before(hit.expires) {
		raw := hit.rawJSON
		cacheMu.RUnlock()
		var out T
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	cacheMu.RUnlock()

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Lumen/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var out T
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(out)
	if err == nil {
		cacheMu.Lock()
		if _, exists := cacheStore[url]; !exists {
			if len(cacheOrder) >= maxCacheEntries {
				oldest := cacheOrder[0]
				cacheOrder = cacheOrder[1:]
				delete(cacheStore, oldest)
			}
			cacheOrder = append(cacheOrder, url)
		}
		cacheStore[url] = cacheEntry{
			expires: time.Now().Add(defaultCacheTTL),
			rawJSON: raw,
		}
		cacheMu.Unlock()
	}

	return &out, nil
}
