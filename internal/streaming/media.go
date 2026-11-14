package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MediaStreamInfo describes a single video, audio, or subtitle stream in a container.
type MediaStreamInfo struct {
	Index     int    `json:"index"`
	Codec     string `json:"codec"`
	Language  string `json:"language,omitempty"`
	Title     string `json:"title,omitempty"`
	Channels  int    `json:"channels,omitempty"`
	Height    int    `json:"height,omitempty"`
	IsDefault bool   `json:"isDefault"`
}

// MediaInfo holds the result of probing a media stream via ffprobe.
type MediaInfo struct {
	Duration        float64           `json:"duration"`
	Container       string            `json:"container"`
	Video           *MediaStreamInfo  `json:"video,omitempty"`
	Audio           *MediaStreamInfo  `json:"audio,omitempty"`
	AudioTracks     int               `json:"audioTracks"`
	AudioStreams    []MediaStreamInfo `json:"audioStreams"`
	SubtitleStreams []MediaStreamInfo `json:"subtitleStreams"`
	DirectPlayable  bool              `json:"directPlayable"`
	CopyVideo       bool              `json:"copyVideo"`
	CopyAudio       bool              `json:"copyAudio"`
	ReencodesVideo  bool              `json:"reencodesVideo"`
	Reason          string            `json:"reason"`
}

// FfprobeStream represents a stream entry in ffprobe JSON output.
type FfprobeStream struct {
	Index       int    `json:"index"`
	CodecType   string `json:"codec_type"`
	CodecName   string `json:"codec_name"`
	Channels    int    `json:"channels"`
	Height      int    `json:"height"`
	Tags        *struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
	Disposition *struct {
		Default     int `json:"default"`
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

// FfprobeOutput represents the top-level ffprobe JSON output.
type FfprobeOutput struct {
	Streams []FfprobeStream `json:"streams"`
	Format  *struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
}

var (
	browserVideoCodecs = map[string]bool{
		"h264": true,
		"vp8":  true,
		"vp9":  true,
		"av1":  true,
	}
	browserAudioCodecs = map[string]bool{
		"aac":    true,
		"mp3":    true,
		"opus":   true,
		"vorbis": true,
		"flac":   true,
	}
	textSubCodecs = map[string]bool{
		"subrip":   true,
		"srt":      true,
		"ass":      true,
		"ssa":      true,
		"webvtt":   true,
		"mov_text": true,
		"text":     true,
	}
	reMP4Container  = regexp.MustCompile(`mp4|mov`)
	reWebMContainer = regexp.MustCompile(`webm`)

	probeMu       sync.Mutex
	probeCache    = make(map[string]MediaInfo)
	probeOrder    []string

	subExtractMu    sync.Mutex
	subExtractCache = make(map[string]string)
	subExtractOrder []string
)

func pickAudio(streams []MediaStreamInfo) *MediaStreamInfo {
	if len(streams) == 0 {
		return nil
	}
	for i := range streams {
		lang := strings.ToLower(streams[i].Language)
		if lang == "eng" || lang == "en" {
			return &streams[i]
		}
	}
	for i := range streams {
		if streams[i].IsDefault {
			return &streams[i]
		}
	}
	return &streams[0]
}

// AnalyzeProbeOutput classifies ffprobe output into Tier 1 (direct play), Tier 2 (copy video + AAC audio),
// or Tier 3 (transcode video to H.264 <= 1080p + AAC audio).
func AnalyzeProbeOutput(data FfprobeOutput) MediaInfo {
	var (
		videos    []MediaStreamInfo
		audios    []MediaStreamInfo
		subtitles []MediaStreamInfo
	)
	for _, s := range data.Streams {
		codec := strings.ToLower(strings.TrimSpace(s.CodecName))
		if codec == "" {
			codec = "unknown"
		}
		var lang, title string
		if s.Tags != nil {
			lang = s.Tags.Language
			title = s.Tags.Title
		}
		isDef := s.Disposition != nil && s.Disposition.Default == 1
		attachedPic := s.Disposition != nil && s.Disposition.AttachedPic != 0
		info := MediaStreamInfo{
			Index:     s.Index,
			Codec:     codec,
			Language:  lang,
			Title:     title,
			Channels:  s.Channels,
			Height:    s.Height,
			IsDefault: isDef,
		}
		switch s.CodecType {
		case "video":
			if !attachedPic {
				videos = append(videos, info)
			}
		case "audio":
			audios = append(audios, info)
		case "subtitle":
			if textSubCodecs[codec] {
				subtitles = append(subtitles, info)
			}
		}
	}

	var video *MediaStreamInfo
	if len(videos) > 0 {
		video = &videos[0]
	}
	audio := pickAudio(audios)
	if audios == nil {
		audios = []MediaStreamInfo{}
	}
	if subtitles == nil {
		subtitles = []MediaStreamInfo{}
	}

	var container string
	var duration float64
	if data.Format != nil {
		container = data.Format.FormatName
		duration, _ = strconv.ParseFloat(data.Format.Duration, 64)
	}

	videoOk := video == nil || browserVideoCodecs[video.Codec]
	audioOk := audio == nil || browserAudioCodecs[audio.Codec]
	isMp4 := reMP4Container.MatchString(container)
	isWebm := reWebMContainer.MatchString(container) &&
		(video == nil || video.Codec == "vp8" || video.Codec == "vp9" || video.Codec == "av1") &&
		(audio == nil || audio.Codec == "opus" || audio.Codec == "vorbis")

	audioIsFirstChoice := true
	if audio != nil && len(audios) > 1 {
		firstDefaultIdx := 0
		for i := range audios {
			if audios[i].IsDefault {
				firstDefaultIdx = i
				break
			}
		}
		audioIsFirstChoice = audio.Index == audios[firstDefaultIdx].Index
	}

	directPlayable := (isMp4 || isWebm) && videoOk && audioOk && audioIsFirstChoice
	var reason string
	if directPlayable {
		reason = "direct"
	} else {
		var parts []string
		if !(isMp4 || isWebm) {
			cName := strings.Split(container, ",")[0]
			parts = append(parts, "container "+cName)
		}
		if !videoOk && video != nil {
			parts = append(parts, "video "+video.Codec)
		}
		if !audioOk && audio != nil {
			parts = append(parts, "audio "+audio.Codec)
		}
		if !audioIsFirstChoice && audio != nil {
			parts = append(parts, "audio track "+audio.Language)
		}
		reason = strings.Join(parts, ", ")
	}

	copyVideo := videoOk && video != nil && video.Codec == "h264"
	copyAudio := true
	if audio != nil {
		copyAudio = audio.Codec == "aac" || audio.Codec == "mp3"
	}

	return MediaInfo{
		Duration:        duration,
		Container:       container,
		Video:           video,
		Audio:           audio,
		AudioTracks:     len(audios),
		AudioStreams:    audios,
		SubtitleStreams: subtitles,
		DirectPlayable:  directPlayable,
		CopyVideo:       copyVideo,
		CopyAudio:       copyAudio,
		ReencodesVideo:  !directPlayable && !copyVideo,
		Reason:          reason,
	}
}

// ProbeMedia runs ffprobe on input and returns its classified MediaInfo.
func ProbeMedia(ctx context.Context, input string) (MediaInfo, error) {
	probeMu.Lock()
	if hit, ok := probeCache[input]; ok {
		probeMu.Unlock()
		return hit, nil
	}
	probeMu.Unlock()

	probeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-rw_timeout", "30000000",
		input,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errLines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
		lastLine := errLines[len(errLines)-1]
		if lastLine != "" {
			return MediaInfo{}, fmt.Errorf("%s", lastLine)
		}
		return MediaInfo{}, err
	}

	var parsed FfprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return MediaInfo{}, err
	}
	info := AnalyzeProbeOutput(parsed)

	probeMu.Lock()
	if _, exists := probeCache[input]; !exists {
		if len(probeOrder) >= 200 {
			oldest := probeOrder[0]
			probeOrder = probeOrder[1:]
			delete(probeCache, oldest)
		}
		probeOrder = append(probeOrder, input)
	}
	probeCache[input] = info
	probeMu.Unlock()

	return info, nil
}

// ExtractEmbeddedSubtitleSRT extracts an embedded text subtitle stream as raw SRT text.
func ExtractEmbeddedSubtitleSRT(ctx context.Context, input string, streamIndex int) (string, error) {
	key := fmt.Sprintf("%s#%d", input, streamIndex)
	subExtractMu.Lock()
	if hit, ok := subExtractCache[key]; ok {
		subExtractMu.Unlock()
		return hit, nil
	}
	subExtractMu.Unlock()

	subCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(subCtx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-i", input,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-f", "srt", "pipe:1",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	outStr := stdout.String()
	if len(outStr) > 2*1024*1024 {
		outStr = outStr[:2*1024*1024]
	}
	if err != nil && strings.TrimSpace(outStr) == "" {
		errLines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
		lastLine := errLines[len(errLines)-1]
		if lastLine != "" {
			return "", fmt.Errorf("%s", lastLine)
		}
		return "", err
	}

	subExtractMu.Lock()
	if _, exists := subExtractCache[key]; !exists {
		if len(subExtractOrder) >= 30 {
			oldest := subExtractOrder[0]
			subExtractOrder = subExtractOrder[1:]
			delete(subExtractCache, oldest)
		}
		subExtractOrder = append(subExtractOrder, key)
	}
	subExtractCache[key] = outStr
	subExtractMu.Unlock()

	return outStr, nil
}

// GetConfiguredHwAccel returns the configured hardware acceleration mode.
func GetConfiguredHwAccel() string {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("LUMEN_HWACCEL")))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(os.Getenv("SCREENTIME_HWACCEL")))
	}
	switch raw {
	case "vaapi", "nvenc", "qsv", "videotoolbox":
		return raw
	default:
		return "none"
	}
}

// BuildTranscodeArgs constructs the exact ffmpeg command-line arguments for Tier 2 (copy video) or Tier 3 (transcode video) fragmented MP4 streaming.
func BuildTranscodeArgs(input string, start float64, info MediaInfo, audioStreamIndex *int, hwAccel string, vaapiDevice string) []string {
	var chosenAudio *MediaStreamInfo
	if audioStreamIndex != nil {
		for i := range info.AudioStreams {
			if info.AudioStreams[i].Index == *audioStreamIndex {
				chosenAudio = &info.AudioStreams[i]
				break
			}
		}
	}
	if chosenAudio == nil {
		chosenAudio = info.Audio
	}
	copyAudio := true
	if chosenAudio != nil {
		copyAudio = chosenAudio.Codec == "aac" || chosenAudio.Codec == "mp3"
	}
	downscale := info.Video != nil && info.Video.Height > 1080
	if vaapiDevice == "" {
		vaapiDevice = "/dev/dri/renderD128"
	}

	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if !info.CopyVideo && hwAccel == "vaapi" {
		args = append(args, "-vaapi_device", vaapiDevice)
	}
	if start > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", start))
	}
	args = append(args, "-i", input)
	if info.Video != nil {
		args = append(args, "-map", fmt.Sprintf("0:%d", info.Video.Index))
	}
	if chosenAudio != nil {
		args = append(args, "-map", fmt.Sprintf("0:%d", chosenAudio.Index))
	}

	if info.CopyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		switch hwAccel {
		case "vaapi":
			vf := "format=nv12,hwupload"
			if downscale {
				vf = "format=nv12,hwupload,scale_vaapi=w=-2:h=1080"
			}
			args = append(args, "-vf", vf, "-c:v", "h264_vaapi", "-qp", "24")
		case "nvenc":
			args = append(args, "-c:v", "h264_nvenc", "-preset", "p4", "-cq", "23", "-pix_fmt", "yuv420p")
			if downscale {
				args = append(args, "-vf", "scale=-2:1080")
			}
		case "qsv":
			args = append(args, "-c:v", "h264_qsv", "-global_quality", "23")
			if downscale {
				args = append(args, "-vf", "scale=-2:1080")
			}
		case "videotoolbox":
			args = append(args, "-c:v", "h264_videotoolbox", "-q:v", "65", "-pix_fmt", "yuv420p")
			if downscale {
				args = append(args, "-vf", "scale=-2:1080")
			}
		default:
			args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "22", "-pix_fmt", "yuv420p")
			if downscale {
				args = append(args, "-vf", "scale=-2:1080")
			}
		}
	}

	if chosenAudio != nil {
		if copyAudio {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-b:a", "192k", "-ac", "2")
		}
	}

	args = append(args, "-sn", "-dn", "-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof", "pipe:1")
	return args
}

const MaxConcurrentTranscodes = 2

var (
	transcodeMu     sync.Mutex
	activeTranscode = make(map[string]context.CancelFunc)
	transcodeOrder  []string
)

func removeKeyFromSlice(s []string, key string) []string {
	for i, k := range s {
		if k == key {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// TranscodeToMP4 streams `input` from `start` seconds as fragmented MP4 into `w`.
// Enforces MaxConcurrentTranscodes (2) and immediately terminates any previous transcode for the same `input`.
func TranscodeToMP4(ctx context.Context, input string, start float64, info MediaInfo, audioStreamIndex *int, w io.Writer) error {
	transcodeCtx, cancel := context.WithCancel(ctx)

	transcodeMu.Lock()
	if existingCancel, ok := activeTranscode[input]; ok {
		existingCancel()
		delete(activeTranscode, input)
		transcodeOrder = removeKeyFromSlice(transcodeOrder, input)
	}
	for len(activeTranscode) >= MaxConcurrentTranscodes && len(transcodeOrder) > 0 {
		oldest := transcodeOrder[0]
		transcodeOrder = transcodeOrder[1:]
		if fn, ok := activeTranscode[oldest]; ok {
			fn()
			delete(activeTranscode, oldest)
		}
	}
	activeTranscode[input] = cancel
	transcodeOrder = append(transcodeOrder, input)
	transcodeMu.Unlock()

	defer func() {
		cancel()
		transcodeMu.Lock()
		delete(activeTranscode, input)
		transcodeOrder = removeKeyFromSlice(transcodeOrder, input)
		transcodeMu.Unlock()
	}()

	args := BuildTranscodeArgs(input, start, info, audioStreamIndex, GetConfiguredHwAccel(), os.Getenv("LUMEN_VAAPI_DEVICE"))
	cmd := exec.CommandContext(transcodeCtx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	buf := make([]byte, 64*1024)
	_, copyErr := io.CopyBuffer(w, stdout, buf)
	_ = cmd.Wait()
	return copyErr
}
