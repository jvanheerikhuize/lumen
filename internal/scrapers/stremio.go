package scrapers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
	"github.com/jvanheerikhuize/lumen/internal/streaming"
	"github.com/jvanheerikhuize/lumen/internal/types"
)

var (
	reSeeds     = regexp.MustCompile(`(?i)(?:👤|seeds?[:\s]|s[:\s])\s*(\d+)`)
	reSize      = regexp.MustCompile(`(?i)(?:💾\s*)?(\d+(?:\.\d+)?\s*(?:GB|MB|TB|GiB|MiB))`)
	reCodecHEVC = regexp.MustCompile(`(?i)x265|hevc`)
	reCodecH264 = regexp.MustCompile(`(?i)x264|avc|h\.?264`)
	reCodecAV1  = regexp.MustCompile(`(?i)av1`)
	reCodecHDR  = regexp.MustCompile(`(?i)hdr10\+|hdr`)
	reCodecDV   = regexp.MustCompile(`(?i)dolby\s*vision|\bdv\b`)
)

// ParseQuality extracts canonical stream quality from description text.
func ParseQuality(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "4k") || strings.Contains(lower, "2160p") || strings.Contains(lower, "uhd") {
		return "4K"
	}
	if strings.Contains(lower, "1080p") || strings.Contains(lower, "fhd") {
		return "1080p"
	}
	if strings.Contains(lower, "720p") || strings.Contains(lower, "hd") {
		return "720p"
	}
	if strings.Contains(lower, "480p") || strings.Contains(lower, "sd") {
		return "480p"
	}
	return "Auto"
}

// ParseSeeds extracts seeder count from Stremio stream descriptions.
func ParseSeeds(text string) *int {
	m := reSeeds.FindStringSubmatch(text)
	if len(m) < 2 {
		return nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	return &n
}

// ParseSize extracts human-readable file size from Stremio stream descriptions.
func ParseSize(text string) string {
	m := reSize.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// ParseCodec extracts codec and HDR/DV tags from stream description text.
func ParseCodec(text string) string {
	var matches []string
	if reCodecHEVC.MatchString(text) {
		matches = append(matches, "HEVC/x265")
	} else if reCodecH264.MatchString(text) {
		matches = append(matches, "x264")
	}
	if reCodecAV1.MatchString(text) {
		matches = append(matches, "AV1")
	}
	if reCodecHDR.MatchString(text) {
		matches = append(matches, "HDR")
	}
	if reCodecDV.MatchString(text) {
		matches = append(matches, "DV")
	}
	return strings.Join(matches, " • ")
}

// StremioFetchParams configures StremioAddonScraper.FetchStreams.
type StremioFetchParams struct {
	IMDbID    string
	KitsuID   *int
	TMDBID    any
	MediaType string // "movie" | "tv"
	Season    int
	Episode   int
	Title     string
	Lookup    netguard.LookupHostFunc
}

type stremioStreamItem struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	InfoHash string `json:"infoHash"`
	FileIdx  *int   `json:"fileIdx"`
	URL      string `json:"url"`
}

// StremioAddonScraper queries Stremio-compatible stream addons concurrently.
type StremioAddonScraper struct {
	addonURLs []string
}

// NewStremioAddonScraper constructs a StremioAddonScraper with cleaned addon URLs.
func NewStremioAddonScraper(customAddons []string) *StremioAddonScraper {
	return &StremioAddonScraper{
		addonURLs: CleanAddonURLs(customAddons),
	}
}

// FetchStreams queries all configured Stremio addons in parallel, deduplicates by lowercase infoHash, and sorts results.
func (s *StremioAddonScraper) FetchStreams(ctx context.Context, params StremioFetchParams) []types.StreamSource {
	season := params.Season
	if season <= 0 {
		season = 1
	}
	episode := params.Episode
	if episode <= 0 {
		episode = 1
	}
	title := params.Title
	if title == "" {
		title = "Media"
	}

	if params.IMDbID == "" && (params.KitsuID == nil || *params.KitsuID == 0) {
		return []types.StreamSource{}
	}

	var streamID string
	if params.KitsuID != nil && *params.KitsuID > 0 {
		if params.MediaType == "tv" {
			streamID = fmt.Sprintf("kitsu:%d:%d", *params.KitsuID, episode)
		} else {
			streamID = fmt.Sprintf("kitsu:%d", *params.KitsuID)
		}
	} else {
		if params.MediaType == "tv" {
			streamID = fmt.Sprintf("%s:%d:%d", params.IMDbID, season, episode)
		} else {
			streamID = params.IMDbID
		}
	}

	typePath := "movie"
	if params.MediaType == "tv" {
		typePath = "series"
	}

	var (
		mu      sync.Mutex
		directs []types.StreamSource
		byHash  = make(map[string]types.StreamSource)
		wg      sync.WaitGroup
	)

	allowPrivate := netguard.AllowPrivateAddons()

	for addonIdx, baseURL := range s.addonURLs {
		wg.Add(1)
		go func(aIdx int, base string) {
			defer wg.Done()
			cleanBase := strings.TrimRight(base, "/")
			targetURL := fmt.Sprintf("%s/stream/%s/%s.json", cleanBase, typePath, url.PathEscape(streamID))

			resp, err := netguard.GuardedFetch(ctx, targetURL, netguard.FetchOptions{
				Timeout:      7 * time.Second,
				AllowPrivate: allowPrivate,
				Headers: map[string]string{
					"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Screentime/1.0",
				},
				Lookup: params.Lookup,
			})
			if err != nil {
				return
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				return
			}

			var payload struct {
				Streams []stremioStreamItem `json:"streams"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for i, st := range payload.Streams {
				rawTitle := st.Title
				if rawTitle == "" {
					rawTitle = st.Name
				}
				if rawTitle == "" {
					rawTitle = title
				}
				fullDesc := st.Name + "\n" + rawTitle
				quality := ParseQuality(fullDesc)
				seeds := ParseSeeds(fullDesc)
				size := ParseSize(fullDesc)
				codec := ParseCodec(fullDesc)
				firstLine := strings.TrimSpace(strings.Split(rawTitle, "\n")[0])

				magnet := ""
				if st.InfoHash != "" {
					dn := firstLine
					if dn == "" {
						dn = title
					}
					magnet = streaming.BuildMagnet(st.InfoHash, dn)
				}

				name := firstLine
				if name == "" {
					name = fmt.Sprintf("%s (%s)", title, quality)
				}
				var peers *int
				if seeds != nil && *seeds > 0 {
					p := int(math.Round(float64(*seeds) * 0.3))
					peers = &p
				}
				provider := strings.TrimSpace(strings.Split(st.Name, "\n")[0])
				if provider == "" {
					provider = "Torrent Swarm"
				}
				sType := "torrent"
				if st.URL != "" {
					sType = "direct"
				}
				idKey := st.InfoHash
				if idKey == "" {
					idKey = strconv.Itoa(i)
				}

				candidate := types.StreamSource{
					ID:       fmt.Sprintf("addon-%s-%d-%d", idKey, aIdx, i),
					Name:     name,
					Title:    rawTitle,
					Magnet:   magnet,
					URL:      st.URL,
					InfoHash: st.InfoHash,
					FileIdx:  st.FileIdx,
					Quality:  quality,
					Codec:    codec,
					Size:     size,
					Seeds:    seeds,
					Peers:    peers,
					Provider: provider,
					Type:     sType,
				}

				if st.InfoHash != "" {
					hashKey := strings.ToLower(st.InfoHash)
					prev, exists := byHash[hashKey]
					candSeeds := -1
					if candidate.Seeds != nil {
						candSeeds = *candidate.Seeds
					}
					prevSeeds := -1
					if exists && prev.Seeds != nil {
						prevSeeds = *prev.Seeds
					}
					if !exists || candSeeds > prevSeeds {
						byHash[hashKey] = candidate
					}
				} else if st.URL != "" {
					directs = append(directs, candidate)
				}
			}
		}(addonIdx, baseURL)
	}
	wg.Wait()

	results := make([]types.StreamSource, 0, len(directs)+len(byHash))
	results = append(results, directs...)
	for _, v := range byHash {
		results = append(results, v)
	}

	qualityWeight := map[string]int{
		"4K":    4,
		"1080p": 3,
		"720p":  2,
		"480p":  1,
		"Auto":  0,
	}
	sort.SliceStable(results, func(i, j int) bool {
		qA := qualityWeight[results[i].Quality]
		qB := qualityWeight[results[j].Quality]
		if qB != qA {
			return qB < qA
		}
		sA := 0
		if results[i].Seeds != nil {
			sA = *results[i].Seeds
		}
		sB := 0
		if results[j].Seeds != nil {
			sB = *results[j].Seeds
		}
		return sB < sA
	})

	return results
}
