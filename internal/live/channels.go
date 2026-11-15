package live

import (
	"context"

	"github.com/jvanheerikhuize/lumen/internal/types"
)

// LiveChannels is the fallback list of international 24/7 HD streams used only when the Free-TV playlist is unreachable.
var LiveChannels = []types.LiveChannel{
	{
		ID:          "dw-news-en",
		Name:        "DW News English HD",
		Category:    "News",
		Country:     "Germany",
		CountryCode: "DE",
		Language:    "English",
		Logo:        "https://i.imgur.com/A1xzjOI.png",
		StreamURL:   "https://dwamdstream102.akamaized.net/hls/live/2015525/dwstream102/index.m3u8",
		Description: "24/7 in-depth global breaking news, geopolitics, documentaries, and business analysis from Deutsche Welle.",
		IsLive:      true,
		Source:      "curated",
	},
	{
		ID:          "bloomberg-us",
		Name:        "Bloomberg TV",
		Category:    "News",
		Country:     "United States",
		CountryCode: "US",
		Language:    "English",
		Logo:        "https://i.imgur.com/OuogLHx.png",
		StreamURL:   "https://bloomberg.com/media-manifest/streams/us.m3u8",
		Description: "Live business and market news, interviews and analysis from Bloomberg.",
		IsLive:      true,
		Source:      "curated",
	},
	{
		ID:          "france24-en",
		Name:        "France 24 English HD",
		Category:    "News",
		Country:     "France",
		CountryCode: "FR",
		Language:    "English",
		Logo:        "https://i.imgur.com/61MSiq9.png",
		StreamURL:   "https://live.france24.com/hls/live/2037218/F24_EN_HI_HLS/master_5000.m3u8",
		Description: "International news broadcasting live around the clock in English from Paris.",
		IsLive:      true,
		Source:      "curated",
	},
	{
		ID:          "redbull-tv",
		Name:        "Red Bull TV HD",
		Category:    "Sports",
		Country:     "International",
		CountryCode: "",
		Language:    "English",
		Logo:        "",
		StreamURL:   "https://rbmn-live.akamaized.net/hls/live/590964/BoRB-AT/master.m3u8",
		Description: "Extreme sports, world-class racing, outdoor adventure documentaries, music festivals, and live athletic events.",
		IsLive:      true,
		Source:      "curated",
	},
	{
		ID:          "nhk-world-en",
		Name:        "NHK World Japan",
		Category:    "Documentary",
		Country:     "Japan",
		CountryCode: "JP",
		Language:    "English",
		Logo:        "https://upload.wikimedia.org/wikipedia/commons/thumb/8/8d/NHK_World-Japan_TV.svg/500px-NHK_World-Japan_TV.svg.png",
		StreamURL:   "https://media-osa.hls.nhkworld.jp/hls/w/live/master.m3u8",
		Description: "Japanese culture, high-tech innovations, nature documentaries, culinary arts, and Asian news.",
		IsLive:      true,
		Source:      "curated",
	},
	{
		ID:          "aljazeera-en",
		Name:        "Al Jazeera English Live",
		Category:    "News",
		Country:     "International",
		CountryCode: "",
		Language:    "English",
		Logo:        "https://i.imgur.com/BB93NQP.png",
		StreamURL:   "https://live-hls-apps-aje-fa.getaj.net/AJE/index.m3u8",
		Description: "Award-winning global breaking news and investigative journalism from all continents.",
		IsLive:      true,
		Source:      "curated",
	},
	{
		ID:          "trt-world",
		Name:        "TRT World",
		Category:    "News",
		Country:     "Turkey",
		CountryCode: "TR",
		Language:    "English",
		Logo:        "https://upload.wikimedia.org/wikipedia/commons/thumb/2/27/TRT_World.svg/500px-TRT_World.svg.png",
		StreamURL:   "https://tv-trtworld.medya.trt.com.tr/master.m3u8",
		Description: "International news and documentaries in English from Istanbul.",
		IsLive:      true,
		Source:      "curated",
	},
}

// GetAllChannels returns the live Free-TV channel directory, falling back to LiveChannels only if Free-TV is unreachable.
func GetAllChannels(ctx context.Context) []types.LiveChannel {
	freeTV := GetFreeTVChannels(ctx)
	if len(freeTV) > 0 {
		return freeTV
	}
	out := make([]types.LiveChannel, len(LiveChannels))
	copy(out, LiveChannels)
	return out
}

