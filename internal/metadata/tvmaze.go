package metadata

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

const tvmazeBase = "https://api.tvmaze.com"

// EpisodeRef represents the next scheduled episode reference.
type EpisodeRef struct {
	Season   int    `json:"season"`
	Episode  int    `json:"episode"`
	Name     string `json:"name"`
	Airstamp string `json:"airstamp,omitempty"`
}

// ShowStatus represents the series status, network, and next episode.
type ShowStatus struct {
	Status      string      `json:"status"`
	Network     string      `json:"network,omitempty"`
	NextEpisode *EpisodeRef `json:"nextEpisode,omitempty"`
}

type tvmazeEpisode struct {
	Season   *int   `json:"season"`
	Number   *int   `json:"number"`
	Name     string `json:"name"`
	Airstamp string `json:"airstamp"`
}

type tvmazeShow struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Weight    int      `json:"weight"`
	Summary   string   `json:"summary"`
	Premiered string   `json:"premiered"`
	Genres    []string `json:"genres"`
	Image     *struct {
		Medium   string `json:"medium"`
		Original string `json:"original"`
	} `json:"image"`
	Rating *struct {
		Average *float64 `json:"average"`
	} `json:"rating"`
	Externals *struct {
		IMDb string `json:"imdb"`
	} `json:"externals"`
	Network *struct {
		Name string `json:"name"`
	} `json:"network"`
	WebChannel *struct {
		Name string `json:"name"`
	} `json:"webChannel"`
	Embedded *struct {
		NextEpisode *tvmazeEpisode `json:"nextepisode"`
	} `json:"_embedded"`
}

type tvmazeScheduleEntry struct {
	Show     *tvmazeShow `json:"show"`
	Embedded *struct {
		Show *tvmazeShow `json:"show"`
	} `json:"_embedded"`
}

var (
	reStripTags = regexp.MustCompile(`<[^>]+>`)
	reMultiWS   = regexp.MustCompile(`\s+`)
	reDailyShow = regexp.MustCompile(`(?i)^(News|Talk Show|Game Show|Sports)$`)
)

func toEpisodeRef(ep *tvmazeEpisode) *EpisodeRef {
	if ep == nil || ep.Season == nil || ep.Number == nil {
		return nil
	}
	name := ep.Name
	if name == "" {
		name = fmt.Sprintf("Episode %d", *ep.Number)
	}
	return &EpisodeRef{
		Season:   *ep.Season,
		Episode:  *ep.Number,
		Name:     name,
		Airstamp: ep.Airstamp,
	}
}

func stripHTMLSummary(html string) string {
	s := reStripTags.ReplaceAllString(html, "")
	s = reMultiWS.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// GetShowStatus returns status, network, and next episode for a series by IMDb ID.
func GetShowStatus(ctx context.Context, imdbID string) *ShowStatus {
	lookup, err := CachedJSON[struct {
		ID int `json:"id"`
	}](ctx, fmt.Sprintf("%s/lookup/shows?imdb=%s", tvmazeBase, url.QueryEscape(imdbID)), 10*time.Second)
	if err != nil || lookup == nil || lookup.ID == 0 {
		return nil
	}
	show, err := CachedJSON[tvmazeShow](ctx, fmt.Sprintf("%s/shows/%d?embed[]=nextepisode", tvmazeBase, lookup.ID), 10*time.Second)
	if err != nil || show == nil || show.ID == 0 {
		return nil
	}
	status := show.Status
	if status == "" {
		status = "Unknown"
	}
	network := ""
	if show.Network != nil && show.Network.Name != "" {
		network = show.Network.Name
	} else if show.WebChannel != nil && show.WebChannel.Name != "" {
		network = show.WebChannel.Name
	}
	var nextEp *EpisodeRef
	if show.Embedded != nil {
		nextEp = toEpisodeRef(show.Embedded.NextEpisode)
	}
	return &ShowStatus{
		Status:      status,
		Network:     network,
		NextEpisode: nextEp,
	}
}

// FilterAndRankAiringToday deduplicates, filters daily/live formats, sorts by weight descending, and maps schedule entries to MediaItem.
func FilterAndRankAiringToday(entries []tvmazeScheduleEntry, limit int) []types.MediaItem {
	if limit <= 0 {
		limit = 30
	}
	type showEntry struct {
		imdb   string
		show   *tvmazeShow
		weight int
		order  int
	}
	seen := make(map[string]bool)
	var list []showEntry
	for idx, entry := range entries {
		var show *tvmazeShow
		if entry.Embedded != nil && entry.Embedded.Show != nil {
			show = entry.Embedded.Show
		} else {
			show = entry.Show
		}
		if show == nil || show.Externals == nil || show.Externals.IMDb == "" || show.Image == nil {
			continue
		}
		imdb := show.Externals.IMDb
		if seen[imdb] {
			continue
		}
		if reDailyShow.MatchString(show.Type) {
			continue
		}
		seen[imdb] = true
		list = append(list, showEntry{imdb: imdb, show: show, weight: show.Weight, order: idx})
	}

	sort.SliceStable(list, func(i, j int) bool {
		return list[i].weight > list[j].weight
	})
	if len(list) > limit {
		list = list[:limit]
	}

	items := make([]types.MediaItem, 0, len(list))
	for _, se := range list {
		show := se.show
		var posterPath *string
		if show.Image != nil {
			img := show.Image.Original
			if img == "" {
				img = show.Image.Medium
			}
			if img != "" {
				posterPath = &img
			}
		}
		backdrop := fmt.Sprintf("https://images.metahub.space/background/medium/%s/img", se.imdb)
		vote := 0.0
		if show.Rating != nil && show.Rating.Average != nil {
			vote = *show.Rating.Average
		}
		genres := show.Genres
		if genres == nil {
			genres = []string{}
		}
		items = append(items, types.MediaItem{
			ID:           se.imdb,
			ImdbID:       se.imdb,
			Title:        show.Name,
			Overview:     stripHTMLSummary(show.Summary),
			PosterPath:   posterPath,
			BackdropPath: &backdrop,
			MediaType:    "tv",
			FirstAirDate: show.Premiered,
			VoteAverage:  vote,
			Genres:       genres,
		})
	}
	return items
}

// GetAiringToday returns shows with a new episode out today (streaming + US broadcast), most popular first.
func GetAiringToday(ctx context.Context, limit int) []types.MediaItem {
	if limit <= 0 {
		limit = 30
	}
	today := time.Now().UTC().Format("2006-01-02")
	var (
		web       *[]tvmazeScheduleEntry
		broadcast *[]tvmazeScheduleEntry
		wg        sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		web, _ = CachedJSON[[]tvmazeScheduleEntry](ctx, fmt.Sprintf("%s/schedule/web?date=%s", tvmazeBase, today), 10*time.Second)
	}()
	go func() {
		defer wg.Done()
		broadcast, _ = CachedJSON[[]tvmazeScheduleEntry](ctx, fmt.Sprintf("%s/schedule?date=%s&country=US", tvmazeBase, today), 10*time.Second)
	}()
	wg.Wait()

	var combined []tvmazeScheduleEntry
	if web != nil {
		combined = append(combined, *web...)
	}
	if broadcast != nil {
		combined = append(combined, *broadcast...)
	}
	return FilterAndRankAiringToday(combined, limit)
}
