package streaming

import (
	"fmt"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

// OpenSourceStreams contains legal open-source Creative Commons streams and test swarms.
var OpenSourceStreams = map[string][]types.StreamSource{
	"823464": {
		{
			ID:       "src-cosmos-1080p",
			Name:     "Cosmos Laundromat (1080p WebM/MP4)",
			Title:    "Cosmos.Laundromat.2015.1080p.Blender.OpenMovie",
			Magnet:   "magnet:?xt=urn:btih:c9e15763f722f23e98a29decdfae341b98d53056&dn=Cosmos+Laundromat&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.empire-js.us%3A1337&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=wss%3A%2F%2Ftracker.btorrent.xyz&tr=wss%3A%2F%2Ftracker.fastcast.nz&tr=wss%3A%2F%2Ftracker.openwebtorrent.com",
			Quality:  "1080p",
			Size:     "210 MB",
			Seeds:    types.IntPtr(42),
			Peers:    types.IntPtr(15),
			Provider: "Blender Foundation Open Torrent",
			Type:     "torrent",
		},
		{
			ID:       "src-cosmos-direct",
			Name:     "Cosmos Laundromat (MP4)",
			Title:    "Cosmos Laundromat Direct Fast Stream",
			URL:      "https://archive.org/download/cosmos-laundromat-first-cycle/Cosmos%20Laundromat%20-%20First%20Cycle.mp4",
			Quality:  "720p",
			Size:     "204 MB",
			Provider: "Blender Open Movie",
			Type:     "direct",
		},
	},
	"101": {
		{
			ID:       "src-bbb-torrent",
			Name:     "Big Buck Bunny (1080p H.264)",
			Title:    "Big.Buck.Bunny.1080p.surround.mp4",
			Magnet:   "magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c&dn=Big+Buck+Bunny&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.empire-js.us%3A1337&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337",
			Quality:  "1080p",
			Size:     "276 MB",
			Seeds:    types.IntPtr(85),
			Peers:    types.IntPtr(28),
			Provider: "Blender Foundation Torrent Swarm",
			Type:     "torrent",
		},
		{
			ID:       "src-bbb-direct",
			Name:     "Direct MP4 Stream",
			Title:    "Big Buck Bunny Direct Stream",
			URL:      "https://archive.org/download/BigBuckBunny_124/Content/big_buck_bunny_720p_surround.mp4",
			Quality:  "720p",
			Size:     "59 MB",
			Provider: "Blender Open Movie",
			Type:     "direct",
		},
	},
	"102": {
		{
			ID:       "src-tos-torrent",
			Name:     "Tears of Steel (1080p VFX)",
			Title:    "Tears.of.Steel.2012.1080p.mkv",
			Magnet:   "magnet:?xt=urn:btih:209c310cd6b191af46b911a474694077f09a5390&dn=Tears+of+Steel&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337",
			Quality:  "1080p",
			Size:     "560 MB",
			Seeds:    types.IntPtr(34),
			Peers:    types.IntPtr(12),
			Provider: "Open VFX Foundation Swarm",
			Type:     "torrent",
		},
		{
			ID:       "src-tos-direct",
			Name:     "Direct MP4 Stream",
			Title:    "Tears of Steel Direct Stream",
			URL:      "https://archive.org/download/tearsofsteelblendervfxopenmovie800p/Tears%20of%20Steel%20-%20Blender%20VFX%20Open%20Movie%20800p.mp4",
			Quality:  "720p",
			Size:     "236 MB",
			Provider: "Blender Open Movie",
			Type:     "direct",
		},
	},
	"103": {
		{
			ID:       "src-sintel-torrent",
			Name:     "Sintel (4K & 1080p)",
			Title:    "Sintel.2010.1080p.mkv",
			Magnet:   "magnet:?xt=urn:btih:08a806048a1da9843640d71478377e6d44acb0f8&dn=Sintel&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337",
			Quality:  "1080p",
			Size:     "650 MB",
			Seeds:    types.IntPtr(61),
			Peers:    types.IntPtr(19),
			Provider: "Blender Foundation Torrent Swarm",
			Type:     "torrent",
		},
		{
			ID:       "src-sintel-direct",
			Name:     "Direct MP4 Stream",
			Title:    "Sintel Direct Stream",
			URL:      "https://archive.org/download/Sintel/sintel-2048-surround.mp4",
			Quality:  "1080p",
			Size:     "296 MB",
			Provider: "Blender Open Movie",
			Type:     "direct",
		},
	},
}

// GetStreamsForMedia returns any built-in open-source streams for mediaID.
func GetStreamsForMedia(mediaID any) []types.StreamSource {
	key := fmt.Sprintf("%v", mediaID)
	if list, ok := OpenSourceStreams[key]; ok {
		out := make([]types.StreamSource, len(list))
		copy(out, list)
		return out
	}
	return []types.StreamSource{}
}
