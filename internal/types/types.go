package types

import "fmt"

// AnimeRef holds cross-referenced identifiers for an AniList anime entry.
type AnimeRef struct {
	AnilistID int    `json:"anilistId"`
	KitsuID   *int   `json:"kitsuId,omitempty"`
	ImdbID    string `json:"imdbId,omitempty"`
	Season    *int   `json:"season,omitempty"`
}

// Episode represents a single TV or anime episode.
type Episode struct {
	ID            int      `json:"id"`
	EpisodeNumber int      `json:"episodeNumber"`
	SeasonNumber  int      `json:"seasonNumber"`
	Name          string   `json:"name"`
	Overview      string   `json:"overview"`
	StillPath     *string  `json:"stillPath"`
	AirDate       string   `json:"airDate,omitempty"`
	VoteAverage   *float64 `json:"voteAverage,omitempty"`
}

// Season represents a numbered season and its episodes.
type Season struct {
	SeasonNumber int       `json:"seasonNumber"`
	Name         string    `json:"name"`
	EpisodeCount int       `json:"episodeCount"`
	Overview     string    `json:"overview,omitempty"`
	PosterPath   string    `json:"posterPath,omitempty"`
	Episodes     []Episode `json:"episodes"`
}

// MediaItem represents a movie, series, or anime item in catalogs and detail views.
type MediaItem struct {
	ID            any            `json:"id"`
	TmdbID        *int           `json:"tmdbId,omitempty"`
	ImdbID        string         `json:"imdbId,omitempty"`
	Title         string         `json:"title"`
	OriginalTitle string         `json:"originalTitle,omitempty"`
	Overview      string         `json:"overview"`
	PosterPath    *string        `json:"posterPath"`
	BackdropPath  *string        `json:"backdropPath"`
	MediaType     string         `json:"mediaType"` // "movie" | "tv"
	ReleaseDate   string         `json:"releaseDate,omitempty"`
	FirstAirDate  string         `json:"firstAirDate,omitempty"`
	VoteAverage   float64        `json:"voteAverage"`
	VoteCount     *int           `json:"voteCount,omitempty"`
	Genres        []string       `json:"genres"`
	Duration      *int           `json:"duration,omitempty"`
	TrailerURL    string         `json:"trailerUrl,omitempty"`
	Seasons       []Season       `json:"seasons,omitempty"`
	Anime         *AnimeRef      `json:"anime,omitempty"`
	Streams       []StreamSource `json:"streams,omitempty"`
}

// IDString returns the canonical string representation of the MediaItem ID.
func (m MediaItem) IDString() string {
	if m.ID == nil {
		return ""
	}
	return fmt.Sprintf("%v", m.ID)
}

// StreamSource represents a candidate torrent or direct HTTP stream.
type StreamSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Magnet   string `json:"magnet,omitempty"`
	URL      string `json:"url,omitempty"`
	InfoHash string `json:"infoHash,omitempty"`
	FileIdx  *int   `json:"fileIdx,omitempty"`
	Quality  string `json:"quality"` // "4K" | "1080p" | "720p" | "480p" | "Auto"
	Codec    string `json:"codec,omitempty"`
	Size     string `json:"size,omitempty"`
	Seeds    *int   `json:"seeds,omitempty"`
	Peers    *int   `json:"peers,omitempty"`
	Provider string `json:"provider"`
	Type     string `json:"type"` // "torrent" | "direct" | "debrid" | "live"
}

// LiveChannel represents a free-to-air or IPTV live stream channel.
type LiveChannel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode,omitempty"`
	Source      string `json:"source,omitempty"` // "curated" | "freetv"
	GeoBlocked  bool   `json:"geoBlocked"`
	PartTime    bool   `json:"partTime"`
	Language    string `json:"language"`
	Logo        string `json:"logo"`
	StreamURL   string `json:"streamUrl"`
	Description string `json:"description"`
	IsLive      bool   `json:"isLive"`
}

// TorrentFileMeta describes a file inside a BitTorrent swarm.
type TorrentFileMeta struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Length int64  `json:"length"`
	Path   string `json:"path"`
}

// TorrentStats describes real-time statistics for an active torrent swarm.
type TorrentStats struct {
	InfoHash      string            `json:"infoHash"`
	Name          string            `json:"name"`
	TotalSize     int64             `json:"totalSize"`
	Downloaded    int64             `json:"downloaded"`
	Uploaded      int64             `json:"uploaded"`
	DownloadSpeed float64           `json:"downloadSpeed"`
	UploadSpeed   float64           `json:"uploadSpeed"`
	Peers         int               `json:"peers"`
	Progress      float64           `json:"progress"`
	Files         []TorrentFileMeta `json:"files"`
}

// CatalogRow represents a named horizontal media row on the Home view.
type CatalogRow struct {
	Key      string      `json:"key"`
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle,omitempty"`
	Items    []MediaItem `json:"items"`
}

// Helper functions for pointer fields.
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func IntPtr(i int) *int {
	return &i
}

func Float64Ptr(f float64) *float64 {
	return &f
}
