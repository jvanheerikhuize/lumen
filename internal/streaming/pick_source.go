package streaming

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

var (
	reSizeUnit = regexp.MustCompile(`(?i)([\d.]+)\s*(TB|GB|GiB|MB|MiB)`)
	reHEVC     = regexp.MustCompile(`x265|hevc|h\.?265`)
	reHDR      = regexp.MustCompile(`\bhdr|dolby\s*vision|\bdv\b`)
	reRemux    = regexp.MustCompile(`remux|bdremux`)
	reCam      = regexp.MustCompile(`\b(cam|hdcam|ts|telesync|hdts)\b`)
	reH264     = regexp.MustCompile(`x264|h\.?264|avc`)
	reMP4Web   = regexp.MustCompile(`\.mp4\b|\bmp4\b|web-?dl|webrip`)
)

func sizeInGB(size string) (float64, bool) {
	if size == "" {
		return 0, false
	}
	m := reSizeUnit.FindStringSubmatch(size)
	if len(m) < 3 {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	unit := strings.ToUpper(m[2])
	if strings.HasPrefix(unit, "T") {
		return n * 1024, true
	}
	if strings.HasPrefix(unit, "G") {
		return n, true
	}
	return n / 1024, true
}

// StreamabilityScore scores how well a source will stream in a browser.
func StreamabilityScore(source types.StreamSource) float64 {
	text := strings.ToLower(fmt.Sprintf("%s %s %s", source.Name, source.Title, source.Codec))
	score := 0.0

	if source.Type != "torrent" {
		score += 1000
	}

	switch source.Quality {
	case "1080p":
		score += 300
	case "720p":
		score += 260
	case "480p":
		score += 120
	case "Auto":
		score += 100
	case "4K":
		score += 40
	default:
		score += 100
	}

	if reHEVC.MatchString(text) {
		score -= 250
	}
	if reHDR.MatchString(text) {
		score -= 150
	}
	if reRemux.MatchString(text) {
		score -= 200
	}
	if reCam.MatchString(text) {
		score -= 300
	}
	if reH264.MatchString(text) {
		score += 40
	}
	if reMP4Web.MatchString(text) {
		score += 30
	}

	if source.Type == "torrent" {
		seeds := 0
		if source.Seeds != nil {
			seeds = *source.Seeds
		}
		if seeds == 0 {
			score -= 400
		} else {
			score += math.Min(250, math.Log2(float64(seeds+1))*45)
		}
	}

	if gb, ok := sizeInGB(source.Size); ok {
		if gb > 15 {
			score -= 250
		} else if gb > 6 {
			score -= 100
		} else if gb < 0.3 {
			score -= 80
		}
	}

	return score
}

// RankSources returns a copy of sources sorted in descending order of StreamabilityScore.
func RankSources(sources []types.StreamSource) []types.StreamSource {
	out := make([]types.StreamSource, len(sources))
	copy(out, sources)
	sort.SliceStable(out, func(i, j int) bool {
		return StreamabilityScore(out[i]) > StreamabilityScore(out[j])
	})
	return out
}

// StallFailoverThresholdMs is the 25-second timeout before failing over a dead torrent source (REQ-0005).
const StallFailoverThresholdMs = 25000

// ShouldFailoverSource reports whether a torrent source has stalled (0 peers and 0 headBytes for >= 25s)
// and should be released in favor of the next ranked candidate.
func ShouldFailoverSource(elapsedMs int64, peers int, headBytes int64) bool {
	return elapsedMs >= StallFailoverThresholdMs && peers == 0 && headBytes == 0
}
