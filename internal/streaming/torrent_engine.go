package streaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/types/infohash"
	"github.com/jvanheerikhuize/lumen/internal/types"
)

const (
	HeadMinBytes = 16 * 1024 * 1024 // 16 MB
	HeadMaxBytes = 64 * 1024 * 1024 // 64 MB
	TailBytes    = 4 * 1024 * 1024  // 4 MB
	MaxIdleTime  = 20 * time.Minute // 20 minutes
)

// BufferStatus represents the pre-buffer readiness state returned by /api/stream/prepare.
type BufferStatus struct {
	State         string                  `json:"state"` // "connecting" | "buffering" | "ready" | "error"
	Error         string                  `json:"error,omitempty"`
	FileName      string                  `json:"fileName,omitempty"`
	FileSize      int64                   `json:"fileSize"`
	HeadBytes     int64                   `json:"headBytes"`
	HeadTarget    int64                   `json:"headTarget"`
	TailReady     bool                    `json:"tailReady"`
	AheadBytes    int64                   `json:"aheadBytes"`
	Complete      bool                    `json:"complete"`
	Peers         int                     `json:"peers"`
	DownloadSpeed float64                 `json:"downloadSpeed"`
	Progress      float64                 `json:"progress"`
	Files         []types.TorrentFileMeta `json:"files,omitempty"`
}

// PieceRange represents an inclusive range of BitTorrent piece indices [From, To].
type PieceRange struct {
	From int
	To   int
}

// HeadTargetFor returns the target head pre-buffer size (~2% of fileSize, clamped between 16 MB and 64 MB, capped at fileSize).
func HeadTargetFor(fileSize int64) int64 {
	if fileSize <= 0 {
		return 0
	}
	twoPct := int64(math.Round(float64(fileSize) * 0.02))
	clamped := max(int64(HeadMinBytes), min(int64(HeadMaxBytes), twoPct))
	return min(fileSize, clamped)
}

// CalculatePriorityPieceRanges computes the file's piece boundaries and high-priority head and tail piece ranges.
func CalculatePriorityPieceRanges(offset, fileLength, pieceLength int64) (first, headEnd, tailStart, last int, headTarget int64) {
	if fileLength <= 0 || pieceLength <= 0 {
		return 0, 0, 0, 0, 0
	}
	headTarget = HeadTargetFor(fileLength)
	first = int(offset / pieceLength)
	last = int((offset + fileLength - 1) / pieceLength)
	headEnd = min(last, int((offset+headTarget-1)/pieceLength))
	tailStartByte := offset + fileLength - TailBytes
	if tailStartByte < offset {
		tailStartByte = offset
	}
	tailStart = max(first, int(tailStartByte/pieceLength))
	return first, headEnd, tailStart, last, headTarget
}

// ComputeBufferMetrics calculates BufferStatus for a file given a piece completion predicate.
func ComputeBufferMetrics(
	offset, fileLength, pieceLength, fromByte int64,
	peers int,
	downloadSpeed float64,
	fileName string,
	pieceDone func(pieceIdx int) bool,
) BufferStatus {
	if fileLength <= 0 || pieceLength <= 0 {
		return BufferStatus{State: "connecting", Peers: peers}
	}
	first, _, tailStart, last, headTarget := CalculatePriorityPieceRanges(offset, fileLength, pieceLength)

	piece := first
	for piece <= last && pieceDone(piece) {
		piece++
	}
	headBytes := max(int64(0), min(fileLength, int64(piece)*pieceLength-offset))

	tailReady := true
	for p := tailStart; p <= last; p++ {
		if !pieceDone(p) {
			tailReady = false
			break
		}
	}

	completed := 0
	totalPieces := last - first + 1
	for p := first; p <= last; p++ {
		if pieceDone(p) {
			completed++
		}
	}

	aheadBytes := headBytes
	from := max(int64(0), min(fromByte, fileLength-1))
	if from > 0 {
		startPiece := int((offset + from) / pieceLength)
		p := startPiece
		for p <= last && pieceDone(p) {
			p++
		}
		if p == startPiece {
			aheadBytes = 0
		} else {
			aheadBytes = max(int64(0), min(fileLength, int64(p)*pieceLength-offset)-from)
		}
	}

	complete := completed == totalPieces
	ready := (headBytes >= headTarget && tailReady) ||
		headBytes >= int64(float64(headTarget)*1.5) ||
		complete

	state := "buffering"
	if ready {
		state = "ready"
	}

	return BufferStatus{
		State:         state,
		FileName:      fileName,
		FileSize:      fileLength,
		HeadBytes:     headBytes,
		HeadTarget:    headTarget,
		TailReady:     tailReady,
		AheadBytes:    aheadBytes,
		Complete:      complete,
		Peers:         peers,
		DownloadSpeed: downloadSpeed,
		Progress:      float64(completed) / float64(totalPieces),
	}
}

var videoExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".webm": true,
	".avi":  true,
	".mov":  true,
	".m4v":  true,
	".wmv":  true,
	".flv":  true,
	".ts":   true,
}

var mimeByExt = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".mov":  "video/mp4",
	".webm": "video/webm",
	".ogv":  "video/ogg",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".ts":   "video/mp2t",
}

// SelectTargetVideoFileIndex selects either explicitIdx (if in bounds) or the largest video file (falling back to largest file overall).
func SelectTargetVideoFileIndex(files []types.TorrentFileMeta, explicitIdx *int) int {
	if len(files) == 0 {
		return -1
	}
	if explicitIdx != nil && *explicitIdx >= 0 && *explicitIdx < len(files) {
		return *explicitIdx
	}
	bestVideoIdx := -1
	var bestVideoLen int64 = -1
	bestAnyIdx := 0
	var bestAnyLen int64 = -1

	for i, f := range files {
		if f.Length > bestAnyLen {
			bestAnyLen = f.Length
			bestAnyIdx = i
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if videoExtensions[ext] && f.Length > bestVideoLen {
			bestVideoLen = f.Length
			bestVideoIdx = i
		}
	}
	if bestVideoIdx >= 0 {
		return bestVideoIdx
	}
	return bestAnyIdx
}

type managedSwarm struct {
	infoHash         string
	magnet           string
	t                *torrent.Torrent
	selectedIdx      int
	lastAccessed     time.Time
	ready            bool
	errMsg           string
	readyCh          chan struct{}
	lastSampleTime   time.Time
	lastDownloadByte int64
	downloadSpeed    float64
	cacheSubdir      string
}

// TorrentEngineManager manages active BitTorrent swarms, piece prioritization, and idle cleanup.
type TorrentEngineManager struct {
	mu       sync.Mutex
	client   *torrent.Client
	swarms   map[string]*managedSwarm
	cacheDir string
	stopCh   chan struct{}
}

// NewTorrentEngineManager creates a new TorrentEngineManager using cacheDir (defaults to os.TempDir()/lumen-torrents).
func NewTorrentEngineManager(cacheDir string) *TorrentEngineManager {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "lumen-torrents")
	}
	_ = os.RemoveAll(cacheDir)
	_ = os.MkdirAll(cacheDir, 0o755)
	mgr := &TorrentEngineManager{
		swarms:   make(map[string]*managedSwarm),
		cacheDir: cacheDir,
		stopCh:   make(chan struct{}),
	}
	go mgr.periodicCleanupLoop()
	return mgr
}

var DefaultTorrentManager = NewTorrentEngineManager("")

func (m *TorrentEngineManager) periodicCleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.EvictIdleSwarms(time.Now(), MaxIdleTime)
		case <-m.stopCh:
			return
		}
	}
}

func (m *TorrentEngineManager) ensureClientLocked() (*torrent.Client, error) {
	if m.client != nil {
		return m.client, nil
	}
	_ = os.MkdirAll(m.cacheDir, 0o755)
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = m.cacheDir
	cfg.Seed = false
	cfg.NoUpload = false
	cfg.ListenPort = 0 // OS-assigned ephemeral port
	cl, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	m.client = cl
	return cl, nil
}

// RegisterMockSwarm registers a mock/synthetic swarm entry for deterministic unit testing of idle eviction.
func (m *TorrentEngineManager) RegisterMockSwarm(hash string, lastAccessed time.Time, cacheSubdir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan struct{})
	close(ch)
	m.swarms[strings.ToLower(hash)] = &managedSwarm{
		infoHash:     strings.ToLower(hash),
		lastAccessed: lastAccessed,
		ready:        true,
		readyCh:      ch,
		cacheSubdir:  cacheSubdir,
	}
}

// EvictIdleSwarms stops and removes any swarm whose lastAccessed timestamp is older than maxIdle.
func (m *TorrentEngineManager) EvictIdleSwarms(now time.Time, maxIdle time.Duration) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var evicted []string
	for key, sw := range m.swarms {
		if now.Sub(sw.lastAccessed) > maxIdle {
			if sw.t != nil {
				sw.t.Drop()
			}
			if sw.cacheSubdir != "" {
				_ = os.RemoveAll(sw.cacheSubdir)
			}
			delete(m.swarms, key)
			evicted = append(evicted, key)
		}
	}
	if len(m.swarms) == 0 && len(evicted) > 0 {
		entries, _ := os.ReadDir(m.cacheDir)
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(m.cacheDir, e.Name()))
		}
	}
	return evicted
}

// Release stops a torrent swarm and cleans up its temporary files.
func (m *TorrentEngineManager) Release(magnetURI string) bool {
	key := ExtractInfoHash(magnetURI)
	if key == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sw, ok := m.swarms[key]
	if !ok {
		return false
	}
	delete(m.swarms, key)
	if sw.t != nil {
		sw.t.Drop()
	}
	if sw.cacheSubdir != "" {
		_ = os.RemoveAll(sw.cacheSubdir)
	}
	return true
}

func (m *TorrentEngineManager) startSwarmLocked(magnetURI string, fileIndex *int) (*managedSwarm, error) {
	cleanMagnet := strings.TrimSpace(magnetURI)
	key := ExtractInfoHash(cleanMagnet)
	if key == "" {
		return nil, errors.New("Invalid magnet URI: missing valid BitTorrent infoHash.")
	}

	if existing, ok := m.swarms[key]; ok {
		if existing.errMsg != "" {
			if existing.t != nil {
				existing.t.Drop()
			}
			delete(m.swarms, key)
		} else {
			existing.lastAccessed = time.Now()
			return existing, nil
		}
	}

	cl, err := m.ensureClientLocked()
	if err != nil {
		return nil, err
	}

	t, err := cl.AddMagnet(cleanMagnet)
	if err != nil {
		var ih infohash.T
		if hexErr := ih.FromHexString(key); hexErr != nil {
			return nil, err
		}
		t, _ = cl.AddTorrentInfoHash(ih)
	}
	allTrackers := Dedupe(append(MagnetTrackers(cleanMagnet), KnownTrackers()...))
	t.AddTrackers([][]string{allTrackers})

	selected := -1
	if fileIndex != nil {
		selected = *fileIndex
	}
	sw := &managedSwarm{
		infoHash:       key,
		magnet:         cleanMagnet,
		t:              t,
		selectedIdx:    selected,
		lastAccessed:   time.Now(),
		readyCh:        make(chan struct{}),
		lastSampleTime: time.Now(),
	}
	m.swarms[key] = sw

	go func() {
		// Also asynchronously fetch fresh trackers
		trs := GetTrackers(context.Background())
		t.AddTrackers([][]string{trs})

		timer := time.NewTimer(45 * time.Second)
		defer timer.Stop()

		select {
		case <-t.GotInfo():
			m.mu.Lock()
			sw.ready = true
			m.applyFileSelectionAndPrioritiesLocked(sw, fileIndex)
			close(sw.readyCh)
			m.mu.Unlock()
		case <-timer.C:
			m.mu.Lock()
			if !sw.ready {
				sw.errMsg = "Could not reach any peers for this torrent. Try a source with more seeds."
				t.Drop()
				close(sw.readyCh)
			}
			m.mu.Unlock()
		}
	}()

	return sw, nil
}

func (m *TorrentEngineManager) applyFileSelectionAndPrioritiesLocked(sw *managedSwarm, explicitIdx *int) {
	if sw.t == nil || sw.t.Info() == nil {
		return
	}
	tFiles := sw.t.Files()
	if len(tFiles) == 0 {
		return
	}
	metas := make([]types.TorrentFileMeta, len(tFiles))
	for i, f := range tFiles {
		metas[i] = types.TorrentFileMeta{
			Index:  i,
			Name:   filepath.Base(f.DisplayPath()),
			Length: f.Length(),
			Path:   f.DisplayPath(),
		}
	}
	idx := explicitIdx
	if idx == nil && sw.selectedIdx >= 0 {
		idx = &sw.selectedIdx
	}
	chosen := SelectTargetVideoFileIndex(metas, idx)
	if chosen < 0 || chosen >= len(tFiles) {
		return
	}
	sw.selectedIdx = chosen

	for i, f := range tFiles {
		if i == chosen {
			f.Download()
		} else {
			f.SetPriority(torrent.PiecePriorityNone)
		}
	}

	targetFile := tFiles[chosen]
	pieceLen := sw.t.Info().PieceLength
	if pieceLen <= 0 {
		return
	}
	first, headEnd, tailStart, last, _ := CalculatePriorityPieceRanges(targetFile.Offset(), targetFile.Length(), pieceLen)
	numPieces := sw.t.NumPieces()
	for p := first; p <= headEnd && p < numPieces; p++ {
		sw.t.Piece(p).SetPriority(torrent.PiecePriorityNow)
	}
	for p := tailStart; p <= last && p < numPieces; p++ {
		sw.t.Piece(p).SetPriority(torrent.PiecePriorityHigh)
	}
}

func (m *TorrentEngineManager) sampleSpeedLocked(sw *managedSwarm) {
	if sw.t == nil {
		return
	}
	now := time.Now()
	elapsed := now.Sub(sw.lastSampleTime).Seconds()
	if elapsed < 1.0 {
		return
	}
	st := sw.t.Stats()
	downloaded := st.BytesReadUsefulData.Int64()
	sw.downloadSpeed = max(0, float64(downloaded-sw.lastDownloadByte)/elapsed)
	sw.lastDownloadByte = downloaded
	sw.lastSampleTime = now
}

// GetBufferStatus returns the non-blocking BufferStatus for magnetURI, starting the swarm on first call.
func (m *TorrentEngineManager) GetBufferStatus(magnetURI string, fileIndex *int, fromByte int64) BufferStatus {
	key := ExtractInfoHash(magnetURI)
	if key == "" {
		return BufferStatus{
			State: "error",
			Error: "Invalid magnet URI: missing valid BitTorrent infoHash.",
		}
	}

	m.mu.Lock()
	sw, exists := m.swarms[key]
	if !exists {
		var err error
		sw, err = m.startSwarmLocked(magnetURI, fileIndex)
		if err != nil {
			m.mu.Unlock()
			return BufferStatus{State: "error", Error: err.Error()}
		}
	}
	sw.lastAccessed = time.Now()
	if sw.errMsg != "" {
		errMsg := sw.errMsg
		m.mu.Unlock()
		return BufferStatus{State: "error", Error: errMsg}
	}
	if !sw.ready || sw.t == nil || sw.t.Info() == nil {
		peers := 0
		if sw.t != nil {
			peers = sw.t.Stats().ActivePeers
		}
		m.mu.Unlock()
		return BufferStatus{State: "connecting", Peers: peers}
	}

	if fileIndex != nil && *fileIndex >= 0 && *fileIndex < len(sw.t.Files()) && *fileIndex != sw.selectedIdx {
		m.applyFileSelectionAndPrioritiesLocked(sw, fileIndex)
	}

	tFiles := sw.t.Files()
	if sw.selectedIdx < 0 || sw.selectedIdx >= len(tFiles) {
		m.mu.Unlock()
		return BufferStatus{State: "connecting"}
	}
	targetFile := tFiles[sw.selectedIdx]
	pieceLen := sw.t.Info().PieceLength
	peers := sw.t.Stats().ActivePeers
	m.sampleSpeedLocked(sw)
	speed := sw.downloadSpeed
	tRef := sw.t
	offset := targetFile.Offset()
	fileLen := targetFile.Length()
	fileName := filepath.Base(targetFile.DisplayPath())
	m.mu.Unlock()

	numPieces := tRef.NumPieces()
	return ComputeBufferMetrics(offset, fileLen, pieceLen, fromByte, peers, speed, fileName, func(p int) bool {
		if p < 0 || p >= numPieces {
			return false
		}
		return tRef.PieceState(p).Complete
	})
}

// GetStats returns the current TorrentStats (including file metadata list) for magnetURI.
func (m *TorrentEngineManager) GetStats(magnetURI string) *types.TorrentStats {
	key := ExtractInfoHash(magnetURI)
	if key == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sw, ok := m.swarms[key]
	if !ok || sw.t == nil {
		return nil
	}
	var filesMeta []types.TorrentFileMeta
	var totalLength int64
	name := "Torrent Stream"
	if sw.ready && sw.t.Info() != nil {
		tFiles := sw.t.Files()
		filesMeta = make([]types.TorrentFileMeta, len(tFiles))
		for i, f := range tFiles {
			fName := filepath.Base(f.DisplayPath())
			filesMeta[i] = types.TorrentFileMeta{
				Index:  i,
				Name:   fName,
				Length: f.Length(),
				Path:   f.DisplayPath(),
			}
			totalLength += f.Length()
		}
		if sw.selectedIdx >= 0 && sw.selectedIdx < len(tFiles) {
			totalLength = tFiles[sw.selectedIdx].Length()
			name = filepath.Base(tFiles[sw.selectedIdx].DisplayPath())
		}
	}
	if filesMeta == nil {
		filesMeta = []types.TorrentFileMeta{}
	}
	st := sw.t.Stats()
	downloaded := st.BytesReadUsefulData.Int64()
	uploaded := st.BytesWrittenData.Int64()
	progress := 0.0
	if totalLength > 0 {
		progress = min(1.0, float64(downloaded)/float64(totalLength))
	}
	return &types.TorrentStats{
		InfoHash:      sw.infoHash,
		Name:          name,
		TotalSize:     totalLength,
		Downloaded:    downloaded,
		Uploaded:      uploaded,
		DownloadSpeed: sw.downloadSpeed,
		UploadSpeed:   0,
		Peers:         st.ActivePeers,
		Progress:      progress,
		Files:         filesMeta,
	}
}

// ServeStream serves the selected video file from a torrent swarm over HTTP with full 206 Partial Content Range support.
func (m *TorrentEngineManager) ServeStream(w http.ResponseWriter, r *http.Request, magnetURI string, fileIndex *int) {
	key := ExtractInfoHash(magnetURI)
	if key == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing or invalid magnet parameter")
		return
	}

	m.mu.Lock()
	sw, err := m.startSwarmLocked(magnetURI, fileIndex)
	m.mu.Unlock()
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	select {
	case <-sw.readyCh:
	case <-r.Context().Done():
		return
	}

	m.mu.Lock()
	if sw.errMsg != "" {
		errMsg := sw.errMsg
		m.mu.Unlock()
		writeJSONError(w, http.StatusBadGateway, errMsg)
		return
	}
	if fileIndex != nil && *fileIndex >= 0 && *fileIndex < len(sw.t.Files()) && *fileIndex != sw.selectedIdx {
		m.applyFileSelectionAndPrioritiesLocked(sw, fileIndex)
	}
	tFiles := sw.t.Files()
	if sw.selectedIdx < 0 || sw.selectedIdx >= len(tFiles) {
		m.mu.Unlock()
		writeJSONError(w, http.StatusNotFound, "No streamable video file found in torrent")
		return
	}
	targetFile := tFiles[sw.selectedIdx]
	sw.lastAccessed = time.Now()
	m.mu.Unlock()

	total := targetFile.Length()
	ext := strings.ToLower(filepath.Ext(targetFile.DisplayPath()))
	contentType := mimeByExt[ext]
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")

	rangeHeader := r.Header.Get("Range")
	var start int64 = 0
	var end int64 = total - 1
	status := http.StatusOK

	if rangeHeader != "" && strings.HasPrefix(rangeHeader, "bytes=") {
		spec := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.SplitN(spec, ",", 2)[0]
		bounds := strings.SplitN(parts, "-", 2)
		if len(bounds) == 2 {
			sStr, eStr := strings.TrimSpace(bounds[0]), strings.TrimSpace(bounds[1])
			if sStr != "" {
				if parsedS, err := strconv.ParseInt(sStr, 10, 64); err == nil {
					start = parsedS
				}
				if eStr != "" {
					if parsedE, err := strconv.ParseInt(eStr, 10, 64); err == nil {
						end = min(parsedE, total-1)
					}
				}
			} else if eStr != "" {
				if suffixLen, err := strconv.ParseInt(eStr, 10, 64); err == nil {
					start = max(int64(0), total-suffixLen)
					end = total - 1
				}
			}
		}
		if start >= total || start > end || start < 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", total))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		status = http.StatusPartialContent
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	}

	contentLength := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.WriteHeader(status)

	if r.Method == http.MethodHead {
		return
	}

	reader := targetFile.NewReader()
	defer func() { _ = reader.Close() }()
	reader.SetResponsive()
	reader.SetReadahead(16 * 1024 * 1024)

	if _, err := reader.Seek(start, io.SeekStart); err != nil {
		return
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-r.Context().Done():
			_ = reader.Close()
		case <-done:
		}
	}()

	_, _ = io.CopyN(w, reader, contentLength)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, msg)
}
