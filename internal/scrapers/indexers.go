package scrapers

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
	"github.com/jvanheerikhuize/lumen/internal/streaming"
	"github.com/jvanheerikhuize/lumen/internal/types"
)

type prowlarrResult struct {
	Title       string `json:"title"`
	Size        int64  `json:"size"`
	Seeders     *int   `json:"seeders"`
	Leechers    *int   `json:"leechers"`
	Indexer     string `json:"indexer"`
	MagnetURL   string `json:"magnetUrl"`
	DownloadURL string `json:"downloadUrl"`
	InfoHash    string `json:"infoHash"`
	GUID        string `json:"guid"`
}

type torznabRSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []torznabItem `xml:"item"`
	} `xml:"channel"`
}

type torznabItem struct {
	Title     string `xml:"title"`
	Link      string `xml:"link"`
	GUID      string `xml:"guid"`
	Size      int64  `xml:"size"`
	Enclosure struct {
		URL string `xml:"url,attr"`
	} `xml:"enclosure"`
	JackettIndexer string        `xml:"jackettindexer"`
	Attrs          []torznabAttr `xml:"attr"`
}

type torznabAttr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const k = 1024.0
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := int(math.Floor(math.Log(float64(bytes)) / math.Log(k)))
	if i < 0 {
		i = 0
	}
	if i >= len(sizes) {
		i = len(sizes) - 1
	}
	val := float64(bytes) / math.Pow(k, float64(i))
	rounded := math.Round(val*10) / 10
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%.0f %s", rounded, sizes[i])
	}
	return fmt.Sprintf("%.1f %s", rounded, sizes[i])
}

func isMagnetStr(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "magnet:?")
}

// ParseProwlarrJSON parses Prowlarr /api/v1/search JSON results into StreamSource candidates.
func ParseProwlarrJSON(rawJSON []byte, fallbackQuery string) ([]types.StreamSource, error) {
	var list []prowlarrResult
	if err := json.Unmarshal(rawJSON, &list); err != nil {
		return nil, err
	}
	var results []types.StreamSource
	for idx, item := range list {
		title := item.Title
		if title == "" {
			title = fallbackQuery
		}
		quality := ParseQuality(title)
		codec := ParseCodec(title)
		sizeStr := ""
		if item.Size > 0 {
			sizeStr = formatBytes(item.Size)
		}
		indexer := item.Indexer
		if indexer == "" {
			indexer = "Prowlarr Indexer"
		}

		candidateMagnet := ""
		for _, v := range []string{item.MagnetURL, item.DownloadURL, item.GUID} {
			if isMagnetStr(v) {
				candidateMagnet = strings.TrimSpace(v)
				break
			}
		}
		infoHash := item.InfoHash
		if infoHash == "" && candidateMagnet != "" {
			infoHash = streaming.ExtractInfoHash(candidateMagnet)
		}
		magnet := candidateMagnet
		if magnet == "" && infoHash != "" {
			magnet = streaming.BuildMagnet(infoHash, title)
		}
		if magnet == "" {
			continue
		}
		idKey := infoHash
		if idKey == "" {
			idKey = item.GUID
		}
		if idKey == "" {
			idKey = strconv.Itoa(idx)
		}
		results = append(results, types.StreamSource{
			ID:       fmt.Sprintf("prowlarr-%s-%d", idKey, idx),
			Name:     title,
			Title:    title,
			Magnet:   magnet,
			InfoHash: infoHash,
			Quality:  quality,
			Codec:    codec,
			Size:     sizeStr,
			Seeds:    item.Seeders,
			Peers:    item.Leechers,
			Provider: "Prowlarr: " + indexer,
			Type:     "torrent",
		})
	}
	return results, nil
}

// ParseTorznabXML parses Jackett Torznab XML results into StreamSource candidates.
func ParseTorznabXML(rawXML []byte, fallbackQuery string) ([]types.StreamSource, error) {
	var rss torznabRSS
	if err := xml.Unmarshal(rawXML, &rss); err != nil {
		return nil, err
	}
	var results []types.StreamSource
	for idx, item := range rss.Channel.Items {
		title := item.Title
		if title == "" {
			title = fallbackQuery
		}
		quality := ParseQuality(title)
		codec := ParseCodec(title)

		var (
			size       int64
			seeds      *int
			peers      *int
			infoHash   string
			attrMagnet string
		)
		for _, a := range item.Attrs {
			switch a.Name {
			case "size":
				if v, err := strconv.ParseInt(a.Value, 10, 64); err == nil {
					size = v
				}
			case "seeders":
				if v, err := strconv.Atoi(a.Value); err == nil {
					seeds = &v
				}
			case "peers":
				if v, err := strconv.Atoi(a.Value); err == nil {
					peers = &v
				}
			case "infohash":
				infoHash = a.Value
			case "magneturl":
				if isMagnetStr(a.Value) {
					attrMagnet = strings.TrimSpace(a.Value)
				}
			}
		}

		enclosureURL := strings.TrimSpace(item.Enclosure.URL)
		if enclosureURL == "" {
			enclosureURL = strings.TrimSpace(item.Link)
		}
		if enclosureURL == "" {
			enclosureURL = strings.TrimSpace(item.GUID)
		}
		magnet := attrMagnet
		if magnet == "" && isMagnetStr(enclosureURL) {
			magnet = enclosureURL
		}
		if infoHash == "" && magnet != "" {
			infoHash = streaming.ExtractInfoHash(magnet)
		}
		if magnet == "" && infoHash != "" {
			magnet = streaming.BuildMagnet(infoHash, title)
		}
		if magnet == "" {
			continue
		}

		if size == 0 && item.Size > 0 {
			size = item.Size
		}
		sizeStr := ""
		if size > 0 {
			sizeStr = formatBytes(size)
		}
		indexer := strings.TrimSpace(item.JackettIndexer)
		if indexer == "" {
			indexer = "Jackett Indexer"
		}
		if seeds == nil {
			zero := 0
			seeds = &zero
		}
		idKey := infoHash
		if idKey == "" {
			idKey = strconv.Itoa(idx)
		}

		results = append(results, types.StreamSource{
			ID:       fmt.Sprintf("jackett-%s-%d", idKey, idx),
			Name:     title,
			Title:    title,
			Magnet:   magnet,
			InfoHash: infoHash,
			Quality:  quality,
			Codec:    codec,
			Size:     sizeStr,
			Seeds:    seeds,
			Peers:    peers,
			Provider: "Jackett: " + indexer,
			Type:     "torrent",
		})
	}
	return results, nil
}

// ProwlarrSearchParams configures SearchProwlarr.
type ProwlarrSearchParams struct {
	HostURL   string
	APIKey    string
	Query     string
	MediaType string
	Lookup    netguard.LookupHostFunc
}

// SearchProwlarr searches a self-hosted Prowlarr instance via its JSON v1 API.
func SearchProwlarr(ctx context.Context, params ProwlarrSearchParams) []types.StreamSource {
	if params.HostURL == "" || params.APIKey == "" || strings.TrimSpace(params.Query) == "" {
		return []types.StreamSource{}
	}
	cleanHost := netguard.CheckServiceURL(params.HostURL, true)
	if cleanHost == "" {
		return []types.StreamSource{}
	}
	searchType := "search"
	if params.MediaType == "tv" {
		searchType = "tvsearch"
	}
	targetURL := fmt.Sprintf("%s/api/v1/search?query=%s&type=%s&categories=2000&categories=5000",
		cleanHost, url.QueryEscape(params.Query), searchType)

	resp, err := netguard.GuardedFetch(ctx, targetURL, netguard.FetchOptions{
		Timeout:      8 * time.Second,
		AllowPrivate: true,
		Headers: map[string]string{
			"X-Api-Key": params.APIKey,
			"Accept":    "application/json",
		},
		Lookup: params.Lookup,
	})
	if err != nil {
		return []types.StreamSource{}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return []types.StreamSource{}
	}
	body, err := netguard.ReadBoundedText(resp, 4*1024*1024)
	if err != nil {
		return []types.StreamSource{}
	}
	results, err := ParseProwlarrJSON([]byte(body), params.Query)
	if err != nil {
		return []types.StreamSource{}
	}
	return results
}

// JackettSearchParams configures SearchJackett.
type JackettSearchParams struct {
	HostURL string
	APIKey  string
	Query   string
	IMDbID  string
	Lookup  netguard.LookupHostFunc
}

// SearchJackett searches a self-hosted Jackett instance via its Torznab XML API.
func SearchJackett(ctx context.Context, params JackettSearchParams) []types.StreamSource {
	if params.HostURL == "" || params.APIKey == "" || (strings.TrimSpace(params.Query) == "" && params.IMDbID == "") {
		return []types.StreamSource{}
	}
	cleanHost := netguard.CheckServiceURL(params.HostURL, true)
	if cleanHost == "" {
		return []types.StreamSource{}
	}
	targetURL := fmt.Sprintf("%s/api/v2.0/indexers/all/results/torznab/api?apikey=%s&t=search&q=%s&cat=2000,5000",
		cleanHost, url.QueryEscape(params.APIKey), url.QueryEscape(params.Query))
	if params.IMDbID != "" {
		targetURL += "&imdbid=" + url.QueryEscape(params.IMDbID)
	}

	resp, err := netguard.GuardedFetch(ctx, targetURL, netguard.FetchOptions{
		Timeout:      8 * time.Second,
		AllowPrivate: true,
		Lookup:       params.Lookup,
	})
	if err != nil {
		return []types.StreamSource{}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return []types.StreamSource{}
	}
	body, err := netguard.ReadBoundedText(resp, 4*1024*1024)
	if err != nil {
		return []types.StreamSource{}
	}
	results, err := ParseTorznabXML([]byte(body), params.Query)
	if err != nil {
		return []types.StreamSource{}
	}
	return results
}

// ConnectionTestResult represents the response of /api/integrations/test.
type ConnectionTestResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LatencyMs int64  `json:"latencyMs"`
}

// TestConnection tests connectivity to a Prowlarr or Jackett instance, enforcing CheckServiceURL and GuardedFetch.
func TestConnection(ctx context.Context, indexerType, hostURL, apiKey string, lookup netguard.LookupHostFunc) ConnectionTestResult {
	cleanHost := netguard.CheckServiceURL(hostURL, true)
	if cleanHost == "" {
		return ConnectionTestResult{
			Success: false,
			Message: "Enter an http(s) address, e.g. http://192.168.1.10:9696",
		}
	}
	start := time.Now()
	if indexerType == "prowlarr" {
		resp, err := netguard.GuardedFetch(ctx, cleanHost+"/api/v1/system/status", netguard.FetchOptions{
			Timeout:      8 * time.Second,
			AllowPrivate: true,
			Headers:      map[string]string{"X-Api-Key": apiKey},
			Lookup:       lookup,
		})
		latency := time.Since(start).Milliseconds()
		if err != nil {
			return ConnectionTestResult{
				Success:   false,
				Message:   "Could not connect. Check the address and that the service is running.",
				LatencyMs: latency,
			}
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusOK {
			var status struct {
				Version string `json:"version"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&status)
			v := status.Version
			if v == "" {
				v = "1.x"
			}
			return ConnectionTestResult{
				Success:   true,
				Message:   "Connected to Prowlarr v" + v,
				LatencyMs: latency,
			}
		}
		return ConnectionTestResult{
			Success:   false,
			Message:   fmt.Sprintf("Prowlarr error: HTTP %d", resp.StatusCode),
			LatencyMs: latency,
		}
	}

	targetURL := fmt.Sprintf("%s/api/v2.0/indexers/all/results/torznab/api?apikey=%s&t=caps", cleanHost, url.QueryEscape(apiKey))
	resp, err := netguard.GuardedFetch(ctx, targetURL, netguard.FetchOptions{
		Timeout:      8 * time.Second,
		AllowPrivate: true,
		Lookup:       lookup,
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return ConnectionTestResult{
			Success:   false,
			Message:   "Could not connect. Check the address and that the service is running.",
			LatencyMs: latency,
		}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return ConnectionTestResult{
			Success:   true,
			Message:   "Connected to Jackett / Torznab API successfully!",
			LatencyMs: latency,
		}
	}
	return ConnectionTestResult{
		Success:   false,
		Message:   fmt.Sprintf("Jackett error: HTTP %d", resp.StatusCode),
		LatencyMs: latency,
	}
}
