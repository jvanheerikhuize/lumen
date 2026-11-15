package live

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/netguard"
	"github.com/jvanheerikhuize/lumen/internal/types"
)

var freeTVPlaylistURLs = []string{
	"https://raw.githubusercontent.com/Free-TV/IPTV/master/playlist.m3u8",
	"https://cdn.jsdelivr.net/gh/Free-TV/IPTV@master/playlist.m3u8",
}

const freeTVRefreshInterval = 6 * time.Hour

var (
	reHTTPURL      = regexp.MustCompile(`(?i)^https?://`)
	reStreamExt    = regexp.MustCompile(`(?i)\.(m3u8|mp4)(\?|$)`)
	reExcludedHost = regexp.MustCompile(`(?i)youtube\.com|youtu\.be|twitch\.tv`)
	reVODGroup     = regexp.MustCompile(`(?i)^VOD\b`)
	reMarkers      = regexp.MustCompile(`\s*[ⒼⓈⓉⓎ]`)
	reSlugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
	reNewsWord     = regexp.MustCompile(`(?i)\bnews\b`)
	reISO2         = regexp.MustCompile(`^[A-Z]{2}$`)

	reThemeNews  = regexp.MustCompile(`(?i)news|business|weather`)
	reThemeDoc   = regexp.MustCompile(`(?i)documentar`)
	reThemeMusic = regexp.MustCompile(`(?i)music`)
	reThemeSport = regexp.MustCompile(`(?i)sport`)
	reThemeKids  = regexp.MustCompile(`(?i)kids|children|cartoon|junior|youth`)
	reKidsName   = regexp.MustCompile(`(?i)\b(kids|cbeebies|cbbc|kika|cartoonito|cartoon network|boomerang|gulli|nick jr\.?|nicktoons|nickelodeon|disney channel|disney junior|disney xd|babytv|ducktv|rai yoyo|rai gulp|ketnet|npo zapp|zappelin|toggo|super rtl|tiji|canal j|tiny pop)\b|\bboing\b|\.kids\b`)

	freeTVMu    sync.RWMutex
	freeTVCache []types.LiveChannel
	freeTVAt    time.Time
)

var countryCodeByName = map[string]string{
	"afghanistan": "AF", "albania": "AL", "algeria": "DZ", "andorra": "AD", "angola": "AO",
	"argentina": "AR", "armenia": "AM", "australia": "AU", "austria": "AT", "azerbaijan": "AZ",
	"bahamas": "BS", "bahrain": "BH", "bangladesh": "BD", "barbados": "BB", "belarus": "BY",
	"belgium": "BE", "belize": "BZ", "bolivia": "BO", "bosnia and herzegovina": "BA", "bosnia": "BA",
	"brazil": "BR", "brunei": "BN", "bulgaria": "BG", "cambodia": "KH", "cameroon": "CM",
	"canada": "CA", "chile": "CL", "china": "CN", "colombia": "CO", "costa rica": "CR",
	"croatia": "HR", "cuba": "CU", "cyprus": "CY", "czechia": "CZ", "czech republic": "CZ",
	"denmark": "DK", "dominican republic": "DO", "ecuador": "EC", "egypt": "EG", "el salvador": "SV",
	"estonia": "EE", "ethiopia": "ET", "faroe islands": "FO", "finland": "FI", "france": "FR",
	"georgia": "GE", "germany": "DE", "ghana": "GH", "greece": "GR", "guatemala": "GT",
	"haiti": "HT", "honduras": "HN", "hong kong": "HK", "hungary": "HU", "iceland": "IS",
	"india": "IN", "indonesia": "ID", "iran": "IR", "iraq": "IQ", "ireland": "IE",
	"israel": "IL", "italy": "IT", "ivory coast": "CI", "jamaica": "JM", "japan": "JP",
	"jordan": "JO", "kazakhstan": "KZ", "kenya": "KE", "korea": "KR", "south korea": "KR",
	"north korea": "KP", "kosovo": "XK", "kuwait": "KW", "kyrgyzstan": "KG", "laos": "LA",
	"latvia": "LV", "lebanon": "LB", "libya": "LY", "liechtenstein": "LI", "lithuania": "LT",
	"luxembourg": "LU", "macau": "MO", "malaysia": "MY", "maldives": "MV", "malta": "MT",
	"mexico": "MX", "moldova": "MD", "monaco": "MC", "mongolia": "MN", "montenegro": "ME",
	"morocco": "MA", "mozambique": "MZ", "myanmar": "MM", "nepal": "NP", "netherlands": "NL",
	"new zealand": "NZ", "nicaragua": "NI", "nigeria": "NG", "north macedonia": "MK", "norway": "NO",
	"oman": "OM", "pakistan": "PK", "palestine": "PS", "panama": "PA", "paraguay": "PY",
	"peru": "PE", "philippines": "PH", "poland": "PL", "portugal": "PT", "puerto rico": "PR",
	"qatar": "QA", "romania": "RO", "russia": "RU", "rwanda": "RW", "san marino": "SM",
	"saudi arabia": "SA", "senegal": "SN", "serbia": "RS", "singapore": "SG", "slovakia": "SK",
	"slovenia": "SI", "somalia": "SO", "south africa": "ZA", "spain": "ES", "sri lanka": "LK",
	"sudan": "SD", "suriname": "SR", "sweden": "SE", "switzerland": "CH", "syria": "SY",
	"taiwan": "TW", "tajikistan": "TJ", "tanzania": "TZ", "thailand": "TH", "trinidad and tobago": "TT",
	"tunisia": "TN", "turkey": "TR", "turkmenistan": "TM", "uganda": "UG", "uk": "GB",
	"united kingdom": "GB", "ukraine": "UA", "united arab emirates": "AE", "uae": "AE",
	"usa": "US", "united states": "US", "uruguay": "UY", "uzbekistan": "UZ", "vatican city": "VA",
	"venezuela": "VE", "vietnam": "VN", "yemen": "YE", "zambia": "ZM", "zimbabwe": "ZW",
}

var countryNameByCode = func() map[string]string {
	m := make(map[string]string, len(countryCodeByName))
	for name, code := range countryCodeByName {
		if name == "uk" || name == "usa" || name == "uae" || name == "korea" || name == "bosnia" {
			continue
		}
		words := strings.Fields(name)
		for i, w := range words {
			if w != "and" {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		m[code] = strings.Join(words, " ")
	}
	return m
}()

func extractAttr(line, name string) string {
	key := name + `="`
	idx := strings.Index(line, key)
	if idx == -1 {
		return ""
	}
	rest := line[idx+len(key):]
	end := strings.IndexByte(rest, '"')
	if end == -1 {
		return ""
	}
	return rest[:end]
}

func themedCategory(group string) (string, bool) {
	switch {
	case reThemeKids.MatchString(group):
		return "Kids", true
	case reThemeNews.MatchString(group):
		return "News", true
	case reThemeDoc.MatchString(group):
		return "Documentary", true
	case reThemeMusic.MatchString(group):
		return "Music & Culture", true
	case reThemeSport.MatchString(group):
		return "Sports", true
	default:
		return "", false
	}
}

func resolveCountryName(code, group string) string {
	if code != "" {
		if name, ok := countryNameByCode[strings.ToUpper(code)]; ok {
			return name
		}
	}
	if idx := strings.LastIndex(group, "/"); idx != -1 {
		return strings.TrimSpace(group[idx+1:])
	}
	return strings.TrimSpace(group)
}

// ParseFreeTVPlaylist parses an M3U playlist into LiveChannel entries, skipping VOD, YouTube/Twitch, duplicates, and non-streams.
func ParseFreeTVPlaylist(m3u string) []types.LiveChannel {
	rawLines := strings.Split(strings.ReplaceAll(m3u, "\r\n", "\n"), "\n")
	var channels []types.LiveChannel
	seen := make(map[string]bool)

	for i := 0; i < len(rawLines); i++ {
		info := rawLines[i]
		if !strings.HasPrefix(info, "#EXTINF") {
			continue
		}
		var streamURL string
		if i+1 < len(rawLines) {
			streamURL = strings.TrimSpace(rawLines[i+1])
		}
		if !reHTTPURL.MatchString(streamURL) || !reStreamExt.MatchString(streamURL) || reExcludedHost.MatchString(streamURL) {
			continue
		}
		if seen[streamURL] {
			continue
		}

		group := extractAttr(info, "group-title")
		if reVODGroup.MatchString(group) {
			continue
		}
		commaIdx := strings.LastIndexByte(info, ',')
		rawName := ""
		if commaIdx != -1 {
			rawName = strings.TrimSpace(info[commaIdx+1:])
		}
		name := strings.TrimSpace(reMarkers.ReplaceAllString(rawName, ""))
		if name == "" {
			continue
		}
		seen[streamURL] = true

		groupName := group
		if slashIdx := strings.LastIndex(group, "/"); slashIdx != -1 {
			groupName = group[slashIdx+1:]
		}
		groupName = strings.TrimSpace(groupName)

		themeCat, isThemed := themedCategory(group)
		tvgCountry := extractAttr(info, "tvg-country")
		code := ""
		if tvgCountry != "" {
			parts := strings.FieldsFunc(tvgCountry, func(r rune) bool { return r == ';' || r == ',' })
			if len(parts) > 0 {
				code = strings.ToUpper(strings.TrimSpace(parts[0]))
			}
		}
		if code == "" && !isThemed {
			code = countryCodeByName[strings.ToLower(groupName)]
		}

		category := "General"
		if isThemed {
			category = themeCat
		} else if reKidsName.MatchString(name) {
			category = "Kids"
		} else if reNewsWord.MatchString(name) {
			category = "News"
		}

		country := resolveCountryName(code, group)
		if isThemed && code == "" {
			country = "International"
		}

		countryCode := ""
		if reISO2.MatchString(code) {
			countryCode = code
		}

		slug := reSlugNonAlnum.ReplaceAllString(strings.ToLower(name), "-")
		if len(slug) > 40 {
			slug = slug[:40]
		}

		channels = append(channels, types.LiveChannel{
			ID:          fmt.Sprintf("freetv-%d-%s", len(channels), slug),
			Name:        name,
			Category:    category,
			Country:     country,
			CountryCode: countryCode,
			Language:    "",
			Logo:        extractAttr(info, "tvg-logo"),
			StreamURL:   streamURL,
			Description: "",
			IsLive:      true,
			Source:      "freetv",
			GeoBlocked:  strings.Contains(rawName, "Ⓖ"),
			PartTime:    strings.Contains(rawName, "Ⓢ"),
		})
	}
	if channels == nil {
		return []types.LiveChannel{}
	}
	return channels
}

func downloadFreeTV(ctx context.Context) ([]types.LiveChannel, error) {
	for _, u := range freeTVPlaylistURLs {
		resp, err := netguard.GuardedFetch(ctx, u, netguard.FetchOptions{
			Timeout:      10 * time.Second,
			AllowPrivate: false,
		})
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}
		text, err := netguard.ReadBoundedText(resp, 4*1024*1024)
		if err != nil {
			continue
		}
		chs := ParseFreeTVPlaylist(text)
		if len(chs) > 50 {
			return chs, nil
		}
	}
	return nil, fmt.Errorf("Free-TV playlist unavailable")
}

// GetFreeTVChannels returns cached Free-TV channels (refreshed every 6 hours).
func GetFreeTVChannels(ctx context.Context) []types.LiveChannel {
	freeTVMu.RLock()
	if freeTVCache != nil && time.Since(freeTVAt) < freeTVRefreshInterval {
		out := make([]types.LiveChannel, len(freeTVCache))
		copy(out, freeTVCache)
		freeTVMu.RUnlock()
		return out
	}
	freeTVMu.RUnlock()

	chs, err := downloadFreeTV(ctx)
	freeTVMu.Lock()
	defer freeTVMu.Unlock()
	if err == nil {
		freeTVCache = chs
		freeTVAt = time.Now()
	} else {
		if freeTVCache == nil {
			freeTVCache = []types.LiveChannel{}
		}
		freeTVAt = time.Now().Add(-freeTVRefreshInterval + 15*time.Minute)
	}
	out := make([]types.LiveChannel, len(freeTVCache))
	copy(out, freeTVCache)
	return out
}
