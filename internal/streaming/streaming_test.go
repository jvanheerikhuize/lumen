package streaming

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

func TestMagnetTrackersAndDedupe(t *testing.T) {
	magnet := "magnet:?xt=urn:btih:abc&tr=" +
		url.QueryEscape("udp://tracker.opentrackr.org:1337/announce") +
		"&tr=" +
		url.QueryEscape("wss://tracker.webtorrent.dev") +
		"&tr=" +
		url.QueryEscape("http://tracker.example.org/announce")

	got := MagnetTrackers(magnet)
	want := []string{"udp://tracker.opentrackr.org:1337/announce", "http://tracker.example.org/announce"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	if len(MagnetTrackers("magnet:no-query")) != 0 {
		t.Fatalf("expected empty slice for magnet without query")
	}

	deduped := Dedupe([]string{"udp://A.org:1/announce", "udp://a.org:1/announce/", "udp://b.org:2"})
	wantDeduped := []string{"udp://A.org:1/announce", "udp://b.org:2"}
	if !reflect.DeepEqual(deduped, wantDeduped) {
		t.Fatalf("expected %v, got %v", wantDeduped, deduped)
	}
}

func TestBuildMagnetAndExtractInfoHash(t *testing.T) {
	magnet := BuildMagnet("ABCDEF0123", "Big Buck Bunny")
	if !strings.HasPrefix(magnet, "magnet:?xt=urn:btih:abcdef0123&dn=Big%20Buck%20Bunny&tr=") {
		t.Fatalf("unexpected magnet prefix: %s", magnet)
	}
	if len(MagnetTrackers(magnet)) != len(FallbackTrackers) {
		t.Fatalf("expected %d trackers, got %d", len(FallbackTrackers), len(MagnetTrackers(magnet)))
	}

	hexHash := "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c"
	if got := ExtractInfoHash("magnet:?xt=urn:btih:" + strings.ToUpper(hexHash) + "&dn=Big+Buck+Bunny"); got != hexHash {
		t.Fatalf("expected %s, got %s", hexHash, got)
	}
	if got := ExtractInfoHash("magnet:?dn=Test&xt=urn:btih:3WBFL3G4PSUV7MF37AJSHWDQMLNRY3I4&tr=udp://tracker.opentrackr.org:1337"); got != "dd8255ecdc7ca95fb0bbf81323d87062db1c6d1c" {
		t.Fatalf("unexpected base32 hash conversion: %s", got)
	}
	if got := ExtractInfoHash("https://example.com/file.torrent"); got != "" {
		t.Fatalf("expected empty hash for http url, got %s", got)
	}
	if got := ExtractInfoHash("magnet:?xt=urn:btih:not-a-valid-hash"); got != "" {
		t.Fatalf("expected empty hash for invalid btih, got %s", got)
	}
}

func TestStreamabilityScoreAndRankSources(t *testing.T) {
	directMp4 := types.StreamSource{
		ID:       "direct-1",
		Name:     "Direct CDN MP4",
		Title:    "Movie.2024.1080p.WEB-DL.x264.mp4",
		URL:      "https://cdn.example.com/movie.mp4",
		Quality:  "1080p",
		Provider: "CDN",
		Type:     "direct",
	}
	goodTorrent := types.StreamSource{
		ID:       "tor-1080",
		Name:     "1080p WEB-DL x264",
		Title:    "Movie.2024.1080p.WEB-DL.x264",
		Magnet:   "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567",
		Quality:  "1080p",
		Codec:    "x264",
		Size:     "2.4 GB",
		Seeds:    types.IntPtr(120),
		Provider: "Torrentio",
		Type:     "torrent",
	}
	heavy4kRemux := types.StreamSource{
		ID:       "tor-4k",
		Name:     "4K BluRay REMUX HEVC HDR DV",
		Title:    "Movie.2024.2160p.BluRay.REMUX.HEVC.DV.HDR",
		Magnet:   "magnet:?xt=urn:btih:abcdef0123456789abcdef0123456789abcdef01",
		Quality:  "4K",
		Codec:    "HEVC/x265 • HDR • DV",
		Size:     "48.5 GB",
		Seeds:    types.IntPtr(80),
		Provider: "Torrentio",
		Type:     "torrent",
	}
	deadCam := types.StreamSource{
		ID:       "tor-cam",
		Name:     "HDCAM",
		Title:    "Movie.2024.HDCAM.x264",
		Magnet:   "magnet:?xt=urn:btih:1111111111111111111111111111111111111111",
		Quality:  "480p",
		Size:     "150 MB",
		Seeds:    types.IntPtr(0),
		Provider: "Torrentio",
		Type:     "torrent",
	}

	if !(StreamabilityScore(directMp4) > StreamabilityScore(goodTorrent)) {
		t.Fatalf("expected directMp4 > goodTorrent")
	}
	if !(StreamabilityScore(goodTorrent) > StreamabilityScore(heavy4kRemux)) {
		t.Fatalf("expected goodTorrent > heavy4kRemux")
	}
	if !(StreamabilityScore(heavy4kRemux) > StreamabilityScore(deadCam)) {
		t.Fatalf("expected heavy4kRemux > deadCam")
	}

	ranked := RankSources([]types.StreamSource{deadCam, heavy4kRemux, goodTorrent, directMp4})
	ids := []string{ranked[0].ID, ranked[1].ID, ranked[2].ID, ranked[3].ID}
	wantIDs := []string{"direct-1", "tor-1080", "tor-4k", "tor-cam"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("expected %v, got %v", wantIDs, ids)
	}

	if !ShouldFailoverSource(25000, 0, 0) {
		t.Fatalf("expected failover after 25s with 0 peers and 0 headBytes")
	}
	if ShouldFailoverSource(24000, 0, 0) {
		t.Fatalf("should not failover before 25s")
	}
}

func TestResolvePlaybackInput(t *testing.T) {
	validMagnet := "magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c&dn=Big+Buck+Bunny"
	prevPort := os.Getenv("PORT")
	prevInternal := os.Getenv("SCREENTIME_INTERNAL_PORT")
	prevLumen := os.Getenv("LUMEN_INTERNAL_PORT")
	defer func() {
		_ = os.Setenv("PORT", prevPort)
		_ = os.Setenv("SCREENTIME_INTERNAL_PORT", prevInternal)
		_ = os.Setenv("LUMEN_INTERNAL_PORT", prevLumen)
	}()
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("SCREENTIME_INTERNAL_PORT")
	_ = os.Unsetenv("LUMEN_INTERNAL_PORT")

	req, _ := http.NewRequest(http.MethodGet, "https://screentime.example.com/api/playback/probe?magnet="+encodeURIComponent(validMagnet)+"&fileIndex=2", nil)
	resolved := ResolvePlaybackInput(context.Background(), req)
	want := "http://127.0.0.1:3000/api/stream?magnet=" + encodeURIComponent(validMagnet) + "&fileIndex=2"
	if resolved != want {
		t.Fatalf("expected %s, got %s", want, resolved)
	}

	_ = os.Setenv("SCREENTIME_INTERNAL_PORT", "4100")
	customResolved := ResolvePlaybackInput(context.Background(), req)
	wantCustom := "http://127.0.0.1:4100/api/stream?magnet=" + encodeURIComponent(validMagnet) + "&fileIndex=2"
	if customResolved != wantCustom {
		t.Fatalf("expected %s, got %s", wantCustom, customResolved)
	}

	badMagnetReq, _ := http.NewRequest(http.MethodGet, "http://localhost:3000/api/playback/probe?magnet=not-a-magnet", nil)
	if got := ResolvePlaybackInput(context.Background(), badMagnetReq); got != "" {
		t.Fatalf("expected empty string for invalid magnet, got %s", got)
	}

	lanURLReq, _ := http.NewRequest(http.MethodGet, "http://localhost:3000/api/playback/probe?url=http://192.168.1.50:8080/video.mp4", nil)
	if got := ResolvePlaybackInput(context.Background(), lanURLReq); got != "" {
		t.Fatalf("expected empty string for LAN direct URL, got %s", got)
	}
}

func TestREQ0003_HeadAndTailPiecePrioritizationAndIdleEviction(t *testing.T) {
	// 2 GB (2,147,483,648 bytes) .mp4 video file with 1 MB piece length
	const fileSize int64 = 2 * 1024 * 1024 * 1024
	const pieceLen int64 = 1024 * 1024

	first, headEnd, tailStart, last, headTarget := CalculatePriorityPieceRanges(0, fileSize, pieceLen)
	if headTarget != 42949673 {
		t.Fatalf("expected headTarget 42949673 (~40 MB), got %d", headTarget)
	}
	if first != 0 || headEnd != 40 {
		t.Fatalf("expected head pieces [0..40], got [%d..%d]", first, headEnd)
	}
	if tailStart != 2044 || last != 2047 {
		t.Fatalf("expected tail pieces [2044..2047] (last 4 MB), got [%d..%d]", tailStart, last)
	}

	// Not ready when only half of head is downloaded
	statusBuffering := ComputeBufferMetrics(0, fileSize, pieceLen, 0, 12, 5*1024*1024, "Movie.mp4", func(p int) bool {
		return p < 20
	})
	if statusBuffering.State != "buffering" {
		t.Fatalf("expected buffering, got %s", statusBuffering.State)
	}

	// Ready when headBytes >= headTarget AND tailReady == true
	statusReady := ComputeBufferMetrics(0, fileSize, pieceLen, 0, 12, 5*1024*1024, "Movie.mp4", func(p int) bool {
		return p <= 41 || p >= tailStart
	})
	if statusReady.State != "ready" || !statusReady.TailReady || statusReady.HeadBytes < headTarget {
		t.Fatalf("expected ready with tailReady, got %+v", statusReady)
	}

	// Idle swarm eviction after 20 minutes
	tmpDir := t.TempDir()
	mgr := NewTorrentEngineManager(tmpDir)
	defer close(mgr.stopCh)

	swarmCache := filepath.Join(tmpDir, "swarm-deadbeef")
	_ = os.MkdirAll(swarmCache, 0o755)
	_ = os.WriteFile(filepath.Join(swarmCache, "video.part"), []byte("data"), 0o644)

	mgr.RegisterMockSwarm("0123456789abcdef0123456789abcdef01234567", time.Now().Add(-21*time.Minute), swarmCache)
	evicted := mgr.EvictIdleSwarms(time.Now(), MaxIdleTime)
	if len(evicted) != 1 {
		t.Fatalf("expected 1 evicted swarm, got %v", evicted)
	}
	if _, err := os.Stat(swarmCache); !os.IsNotExist(err) {
		t.Fatalf("expected temporary cache directory %s to be removed on eviction", swarmCache)
	}
}

func TestREQ0004_MediaProbeAndTranscodeClassification(t *testing.T) {
	// Scenario: 1080p H.264 video + 5.1 EAC3 audio in Matroska (.mkv) container
	probeOut := FfprobeOutput{
		Format: &struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		}{
			FormatName: "matroska,webm",
			Duration:   "5400.0",
		},
		Streams: []FfprobeStream{
			{
				Index:     0,
				CodecType: "video",
				CodecName: "h264",
				Height:    1080,
			},
			{
				Index:     1,
				CodecType: "audio",
				CodecName: "eac3",
				Channels:  6,
				Tags: &struct {
					Language string `json:"language"`
					Title    string `json:"title"`
				}{Language: "eng", Title: "5.1 Surround"},
				Disposition: &struct {
					Default     int `json:"default"`
					AttachedPic int `json:"attached_pic"`
				}{Default: 1},
			},
		},
	}

	info := AnalyzeProbeOutput(probeOut)
	if info.DirectPlayable {
		t.Fatalf("expected directPlayable: false for MKV + EAC3")
	}
	if !info.CopyVideo {
		t.Fatalf("expected copyVideo: true for H.264 video")
	}
	if info.CopyAudio {
		t.Fatalf("expected copyAudio: false for EAC3 audio")
	}
	if info.ReencodesVideo {
		t.Fatalf("expected reencodesVideo: false for Tier 2 remux")
	}

	// Seeking to 2700s (45m) in a transcoded stream
	args := BuildTranscodeArgs("http://127.0.0.1:3000/api/stream?magnet=test", 2700, info, nil, "none", "")
	if !slices.Contains(args, "-ss") || !slices.Contains(args, "2700.00") {
		t.Fatalf("expected -ss 2700.00 in ffmpeg args: %v", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-c:v copy") {
		t.Fatalf("expected -c:v copy in Tier 2 args: %s", joined)
	}
	if !strings.Contains(joined, "-c:a aac -b:a 192k -ac 2") {
		t.Fatalf("expected stereo AAC 192k audio transcoding in args: %s", joined)
	}
	if !strings.Contains(joined, "-f mp4 -movflags frag_keyframe+empty_moov+default_base_moof") {
		t.Fatalf("expected fragmented MP4 flags in args: %s", joined)
	}
}
