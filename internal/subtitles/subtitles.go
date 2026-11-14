package subtitles

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/metadata"
)

const (
	openSubtitlesAddon = "https://opensubtitles-v3.strem.io"
	maxPerLanguage     = 3
)

// SubtitleOption represents a selectable WebVTT subtitle track.
type SubtitleOption struct {
	ID    string `json:"id"`
	Lang  string `json:"lang"`
	Label string `json:"label"`
	Src   string `json:"src"`
}

var (
	reTiming     = regexp.MustCompile(`^(\d{1,2}):(\d{2}):(\d{2})[.,](\d{1,3})\s*-->\s*(\d{1,2}):(\d{2}):(\d{2})[.,](\d{1,3})(.*)$`)
	reSSATags    = regexp.MustCompile(`\{\\[^}]*\}`)
	reWebVTTHead = regexp.MustCompile(`^WEBVTT[^\n]*\n`)
	reBlankLines = regexp.MustCompile(`\n{2,}`)
)

func toSeconds(h, m, s, ms string) float64 {
	hv, _ := strconv.Atoi(h)
	mv, _ := strconv.Atoi(m)
	sv, _ := strconv.Atoi(s)
	for len(ms) < 3 {
		ms += "0"
	}
	msv, _ := strconv.Atoi(ms)
	return float64(hv*3600+mv*60+sv) + float64(msv)/1000.0
}

func formatVTTTime(secs float64) string {
	total := int64(math.Max(0, math.Round(secs*1000)))
	h := total / 3600000
	m := (total % 3600000) / 60000
	s := (total % 60000) / 1000
	ms := total % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// SRTToVTT converts SRT (or VTT) subtitle text into WebVTT, shifting cue timestamps earlier by offsetSeconds
// (or later when offsetSeconds is negative) and dropping cues whose shifted end time is <= 0.
func SRTToVTT(srt string, offsetSeconds float64) string {
	s := strings.TrimPrefix(srt, "\uFEFF")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = reSSATags.ReplaceAllString(s, "")
	s = reWebVTTHead.ReplaceAllString(s, "")

	blocks := reBlankLines.Split(s, -1)
	var out []string
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		timingIdx := -1
		var m []string
		for idx, line := range lines {
			if match := reTiming.FindStringSubmatch(line); len(match) == 10 {
				timingIdx = idx
				m = match
				break
			}
		}
		if timingIdx == -1 {
			continue
		}
		start := toSeconds(m[1], m[2], m[3], m[4]) - offsetSeconds
		end := toSeconds(m[5], m[6], m[7], m[8]) - offsetSeconds
		if end <= 0 {
			continue
		}
		text := strings.TrimSpace(strings.Join(lines[timingIdx+1:], "\n"))
		if text == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s --> %s%s\n%s", formatVTTTime(start), formatVTTTime(end), m[9], text))
	}
	if len(out) == 0 {
		return "WEBVTT\n\n\n"
	}
	return "WEBVTT\n\n" + strings.Join(out, "\n\n") + "\n"
}

var twoToThree = map[string][]string{
	"en": {"eng"},
	"nl": {"nld", "dut"},
	"de": {"ger", "deu"},
	"fr": {"fre", "fra"},
	"es": {"spa", "spn"},
	"it": {"ita"},
	"pt": {"por", "pob"},
	"ru": {"rus"},
	"pl": {"pol"},
	"sv": {"swe"},
	"da": {"dan"},
	"no": {"nor"},
	"fi": {"fin"},
	"tr": {"tur"},
	"ar": {"ara"},
	"he": {"heb"},
	"el": {"ell", "gre"},
	"cs": {"cze", "ces"},
	"hu": {"hun"},
	"ro": {"ron", "rum"},
	"ja": {"jpn"},
	"ko": {"kor"},
	"zh": {"chi", "zho", "zht"},
}

var threeToTwo = func() map[string]string {
	m := make(map[string]string)
	for two, threes := range twoToThree {
		for _, three := range threes {
			m[three] = two
		}
	}
	return m
}()

var specialLangs = map[string]string{
	"pob": "Portuguese (Brazil)",
	"por": "Portuguese",
	"chi": "Chinese",
	"zht": "Chinese (Traditional)",
	"ze":  "Chinese/English",
	"scc": "Serbian",
	"mne": "Montenegrin",
}

var isoDisplayNames = map[string]string{
	"en": "English", "eng": "English",
	"nl": "Dutch", "nld": "Dutch", "dut": "Dutch",
	"de": "German", "ger": "German", "deu": "German",
	"fr": "French", "fre": "French", "fra": "French",
	"es": "Spanish", "spa": "Spanish", "spn": "Spanish",
	"it": "Italian", "ita": "Italian",
	"pt": "Portuguese",
	"ru": "Russian", "rus": "Russian",
	"pl": "Polish", "pol": "Polish",
	"sv": "Swedish", "swe": "Swedish",
	"da": "Danish", "dan": "Danish",
	"no": "Norwegian", "nor": "Norwegian",
	"fi": "Finnish", "fin": "Finnish",
	"tr": "Turkish", "tur": "Turkish",
	"ar": "Arabic", "ara": "Arabic",
	"he": "Hebrew", "heb": "Hebrew",
	"el": "Greek", "ell": "Greek", "gre": "Greek",
	"cs": "Czech", "cze": "Czech", "ces": "Czech",
	"hu": "Hungarian", "hun": "Hungarian",
	"ro": "Romanian", "ron": "Romanian", "rum": "Romanian",
	"ja": "Japanese", "jpn": "Japanese",
	"ko": "Korean", "kor": "Korean",
	"zh": "Chinese", "zho": "Chinese",
	"hi": "Hindi", "hin": "Hindi",
	"id": "Indonesian", "ind": "Indonesian",
	"vi": "Vietnamese", "vie": "Vietnamese",
	"th": "Thai", "tha": "Thai",
	"uk": "Ukrainian", "ukr": "Ukrainian",
	"bg": "Bulgarian", "bul": "Bulgarian",
	"hr": "Croatian", "hrv": "Croatian",
	"sr": "Serbian", "srp": "Serbian",
	"sk": "Slovak", "slk": "Slovak", "slo": "Slovak",
	"sl": "Slovenian", "slv": "Slovenian",
}

// NormalizeSubtitleLang normalizes 3-letter OpenSubtitles codes to 2-letter settings codes.
func NormalizeSubtitleLang(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	if lower == "" {
		return "en"
	}
	if lower == "off" {
		return "off"
	}
	if two, ok := threeToTwo[lower]; ok {
		return two
	}
	return lower
}

// MatchesLanguage reports whether an OpenSubtitles language code matches a preferred language (2- or 3-letter).
func MatchesLanguage(subLang, preferred string) bool {
	pref := strings.ToLower(strings.TrimSpace(preferred))
	if pref == "" || pref == "off" {
		return false
	}
	sub := strings.ToLower(strings.TrimSpace(subLang))
	if sub == pref {
		return true
	}
	return slices.Contains(twoToThree[pref], sub)
}

// LanguageLabel returns the human-readable display label for a subtitle language code.
func LanguageLabel(code string) string {
	lower := strings.ToLower(strings.TrimSpace(code))
	if label, ok := specialLangs[lower]; ok {
		return label
	}
	if label, ok := isoDisplayNames[lower]; ok {
		return label
	}
	return strings.ToUpper(code)
}

// IsAllowedSubtitleURL allows only HTTPS URLs under strem.io or *.strem.io.
func IsAllowedSubtitleURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && (host == "strem.io" || strings.HasSuffix(host, ".strem.io"))
}

// FindSubtitlesParams configures FindSubtitles.
type FindSubtitlesParams struct {
	IMDbID    string
	MediaType string // "movie" | "tv"
	Season    int
	Episode   int
}

// FindSubtitles queries the OpenSubtitles v3 Stremio addon and returns up to 3 tracks per language.
func FindSubtitles(ctx context.Context, params FindSubtitlesParams) []SubtitleOption {
	season := params.Season
	if season <= 0 {
		season = 1
	}
	episode := params.Episode
	if episode <= 0 {
		episode = 1
	}
	id := params.IMDbID
	sType := "movie"
	if params.MediaType == "tv" {
		id = fmt.Sprintf("%s:%d:%d", params.IMDbID, season, episode)
		sType = "series"
	}

	targetURL := fmt.Sprintf("%s/subtitles/%s/%s.json", openSubtitlesAddon, sType, url.PathEscape(id))
	data, err := metadata.CachedJSON[struct {
		Subtitles []struct {
			ID   any    `json:"id"`
			URL  string `json:"url"`
			Lang string `json:"lang"`
		} `json:"subtitles"`
	}](ctx, targetURL, 12*time.Second)
	if err != nil || data == nil {
		return []SubtitleOption{}
	}

	perLang := make(map[string]int)
	var options []SubtitleOption
	for _, s := range data.Subtitles {
		if s.URL == "" || s.Lang == "" {
			continue
		}
		count := perLang[s.Lang] + 1
		if count > maxPerLanguage {
			continue
		}
		perLang[s.Lang] = count

		base := LanguageLabel(s.Lang)
		label := base
		if count > 1 {
			label = fmt.Sprintf("%s %d", base, count)
		}
		subID := fmt.Sprintf("%s-%d", s.Lang, count)
		if s.ID != nil && fmt.Sprintf("%v", s.ID) != "" {
			subID = fmt.Sprintf("%v", s.ID)
		}
		qURL := strings.ReplaceAll(url.QueryEscape(s.URL), "+", "%20")
		options = append(options, SubtitleOption{
			ID:    subID,
			Lang:  s.Lang,
			Label: label,
			Src:   "/api/subtitles/vtt?url=" + qURL,
		})
	}

	slices.SortFunc(options, func(a, b SubtitleOption) int {
		return cmp.Compare(a.Label, b.Label)
	})
	if options == nil {
		return []SubtitleOption{}
	}
	return options
}
