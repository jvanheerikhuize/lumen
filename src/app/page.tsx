'use client';

import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { ShieldCheck, Shuffle, Sparkles } from 'lucide-react';
import { Navbar, AppTab } from '@/components/Navbar';
import { HeroBanner } from '@/components/HeroBanner';
import { MediaCarousel, CarouselSkeleton } from '@/components/MediaCarousel';
import { buildMediaRequest, MediaDetailModal, PlaybackContext } from '@/components/MediaDetailModal';
import { VideoPlayerModal, SubtitleQuery, PlayableSource } from '@/components/VideoPlayerModal';
import { rankSources } from '@/lib/streaming/pickSource';
import { SettingsModal } from '@/components/SettingsModal';
import { ProfileManagerModal } from '@/components/ProfileManagerModal';
import { LiveChannelsGrid } from '@/components/LiveChannelsGrid';
import { CatalogBrowser } from '@/components/CatalogBrowser';
import { PageHeader } from '@/components/PageHeader';
import { MediaItem, StreamSource, LiveChannel } from '@/types';
import { FEATURED_HERO, SAMPLE_MEDIA_CATALOG } from '@/lib/metadata/sampleCatalog';
import { filterKidsSafeMedia, isKidsSafeMedia, KIDS_TASTE_GENRES } from '@/lib/kidsFilter';
import {
  getTasteProfile,
  isMediaLiked,
  personalizeRows,
  pickRandomHero,
  POPULAR_TASTE_GENRES,
  recordMediaInteraction,
  rememberHeroId,
  TasteProfile,
  toggleFavoriteGenre,
  toggleLikedMedia,
} from '@/lib/tasteProfile';
import {
  clearProfileLocalState,
  DEFAULT_HOUSEHOLD_PROFILES,
  flushStateToServer,
  getActiveProfileId,
  getHouseholdProfiles,
  getMemberDisplayName,
  getWatchProgressList,
  HouseholdMember,
  removeWatchProgress,
  saveHouseholdProfiles,
  saveWatchProgress,
  setActiveProfileId,
  syncStateFromServer,
  WatchProgressEntry,
} from '@/lib/watchProgress';

interface CatalogRow {
  key: string;
  title: string;
  subtitle?: string;
  items: MediaItem[];
}

interface HomeData {
  featured: MediaItem | null;
  rows: CatalogRow[];
}

const VALID_TABS = new Set<AppTab>(['home', 'movies', 'tv', 'anime', 'live']);
const PROFILE_COLORS = ['rose', 'sky', 'emerald', 'amber', 'purple'] as const;

function sanitizeTasteForProfile(profileTaste: TasteProfile, kidsMode: boolean): TasteProfile {
  if (!kidsMode) return profileTaste;
  const allowedGenres = new Set<string>(KIDS_TASTE_GENRES);
  return {
    ...profileTaste,
    favoriteGenres: profileTaste.favoriteGenres.filter((g) => allowedGenres.has(g)),
    likedItems: filterKidsSafeMedia(profileTaste.likedItems),
  };
}

function buildHomeState(
  rows: CatalogRow[],
  profileTaste: TasteProfile,
  fallbackFeatured: MediaItem | null,
  kidsMode: boolean,
  excludeHeroId?: string | number
): { featured: MediaItem | null; rows: CatalogRow[]; storedTaste: TasteProfile } {
  const safeRows = kidsMode
    ? rows.map((r) => ({ ...r, items: filterKidsSafeMedia(r.items) })).filter((r) => r.items.length > 0)
    : rows;
  const effectiveTaste = sanitizeTasteForProfile(profileTaste, kidsMode);
  const safeFallbackFeatured =
    kidsMode && fallbackFeatured && !isKidsSafeMedia(fallbackFeatured)
      ? safeRows[0]?.items[0] ?? null
      : fallbackFeatured ?? safeRows[0]?.items[0] ?? null;
  const hero = pickRandomHero(safeRows, effectiveTaste, excludeHeroId) ?? safeFallbackFeatured;
  const storedTaste = hero ? rememberHeroId(hero.id) : profileTaste;
  const personalizedTaste = sanitizeTasteForProfile(storedTaste, kidsMode);
  return {
    featured: hero,
    rows: personalizeRows(safeRows, personalizedTaste),
    storedTaste,
  };
}

/** Compute the next (season, episode) in a TV/anime series. */
function findNextEpisode(
  media: MediaItem,
  season: number,
  episode: number
): { season: number; episode: number; label: string } | null {
  if (media.mediaType !== 'tv') return null;
  const seasons = media.seasons || [];
  if (seasons.length > 0) {
    const curSeason = seasons.find((s) => s.seasonNumber === season);
    if (curSeason) {
      const nextEpInSeason = curSeason.episodes.find((e) => e.episodeNumber === episode + 1);
      if (nextEpInSeason) {
        return {
          season,
          episode: nextEpInSeason.episodeNumber,
          label: media.anime ? `Episode ${nextEpInSeason.episodeNumber}` : `S${season} · E${nextEpInSeason.episodeNumber}`,
        };
      }
      const nextSeason = seasons.find((s) => s.seasonNumber === season + 1 && s.episodes.length > 0);
      if (nextSeason) {
        const firstEp = nextSeason.episodes[0].episodeNumber;
        return {
          season: nextSeason.seasonNumber,
          episode: firstEp,
          label: `S${nextSeason.seasonNumber} · E${firstEp}`,
        };
      }
      return null;
    }
  }
  const nextEp = episode + 1;
  return {
    season,
    episode: nextEp,
    label: media.anime ? `Episode ${nextEp}` : `S${season} · E${nextEp}`,
  };
}

/** Rows built from the bundled sample catalog; only used if the catalog API is unreachable. */
function fallbackHome(
  data?: { featured?: MediaItem; trending?: MediaItem[]; movies?: MediaItem[]; series?: MediaItem[] },
  kidsMode = false
): HomeData {
  const baseAll = data?.trending?.length ? data.trending : SAMPLE_MEDIA_CATALOG;
  const all = kidsMode ? filterKidsSafeMedia(baseAll) : baseAll;
  const moviesRaw = data?.movies?.length ? data.movies : all.filter((m) => m.mediaType === 'movie');
  const seriesRaw = data?.series?.length ? data.series : all.filter((m) => m.mediaType === 'tv');
  const rows: CatalogRow[] = [
    { key: 'trending', title: kidsMode ? 'Kids picks' : 'Trending now', items: all },
    { key: 'movies', title: 'Movies', items: kidsMode ? filterKidsSafeMedia(moviesRaw) : moviesRaw },
    { key: 'series', title: 'Series', items: kidsMode ? filterKidsSafeMedia(seriesRaw) : seriesRaw },
  ];
  const featured = kidsMode
    ? data?.featured && isKidsSafeMedia(data.featured)
      ? data.featured
      : all[0] ?? null
    : data?.featured ?? FEATURED_HERO;
  return { featured, rows: rows.filter((r) => r.items.length > 0) };
}

export default function Home() {
  const [activeTab, setActiveTab] = useState<AppTab>('home');
  const [activeGenre, setActiveGenre] = useState<string | null>(null);
  const [rawRows, setRawRows] = useState<CatalogRow[]>([]);
  const [home, setHome] = useState<HomeData | null>(null);
  const [profiles, setProfiles] = useState<HouseholdMember[]>(DEFAULT_HOUSEHOLD_PROFILES);
  const [activeProfileId, setActiveProfileState] = useState<string>('default');
  const [profileReady, setProfileReady] = useState(false);
  const [taste, setTaste] = useState<TasteProfile>({
    genreScores: {},
    favoriteGenres: [],
    likedItems: [],
    recentHeroIds: [],
  });
  const [watchProgress, setWatchProgress] = useState<WatchProgressEntry[]>([]);

  const [selectedMedia, setSelectedMedia] = useState<MediaItem | null>(null);
  const [isSettingsModalOpen, setIsSettingsModalOpen] = useState(false);
  const [isProfileManagerOpen, setIsProfileManagerOpen] = useState(false);
  const [activePlayer, setActivePlayer] = useState<{
    key: string;
    title: string;
    sources: PlayableSource[];
    initialSourceIndex?: number;
    initialStartTime?: number;
    isLive?: boolean;
    subtitleQuery?: SubtitleQuery;
    media?: MediaItem;
    season?: number;
    episode?: number;
  } | null>(null);

  const activeMember = useMemo(
    () => profiles.find((p) => p.id === activeProfileId) || profiles[0],
    [profiles, activeProfileId]
  );
  const isKidsMode = Boolean(activeMember?.isKids);

  // Sync initial state and browser Back/Forward with URL search params (?tab=, ?genre=, ?media=).
  const applyUrlParams = useCallback(
    (rowsData?: HomeData | null, kidsOverride?: boolean) => {
      if (typeof window === 'undefined') return;
      const kids = kidsOverride ?? isKidsMode;
      const params = new URLSearchParams(window.location.search);
      const tabParam = params.get('tab') as AppTab | null;
      if (tabParam && VALID_TABS.has(tabParam)) {
        setActiveTab(tabParam);
      } else {
        setActiveTab('home');
      }
      setActiveGenre(params.get('genre'));

      const mediaParam = params.get('media');
      if (!mediaParam) {
        setSelectedMedia(null);
      } else if (rowsData) {
        const allItems = [
          ...(rowsData.featured ? [rowsData.featured] : []),
          ...rowsData.rows.flatMap((r) => r.items),
          ...SAMPLE_MEDIA_CATALOG,
        ];
        const found = allItems.find((m) => String(m.imdbId || m.id) === mediaParam || String(m.id) === mediaParam);
        if (found && (!kids || isKidsSafeMedia(found))) setSelectedMedia(found);
      }
    },
    [isKidsMode]
  );

  useEffect(() => {
    const onPopState = () => applyUrlParams(home);
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, [applyUrlParams, home]);

  const updateUrl = useCallback((tab: AppTab, genre: string | null, media: MediaItem | null) => {
    if (typeof window === 'undefined') return;
    const params = new URLSearchParams();
    if (tab !== 'home') params.set('tab', tab);
    if (genre && (tab === 'movies' || tab === 'tv' || tab === 'anime')) params.set('genre', genre);
    if (media) params.set('media', String(media.imdbId || media.id));
    const qs = params.toString();
    const nextUrl = qs ? `${window.location.pathname}?${qs}` : window.location.pathname;
    if (nextUrl !== `${window.location.pathname}${window.location.search}`) {
      window.history.pushState(null, '', nextUrl);
    }
  }, []);

  // Load persisted profile state and sync with LAN server once on mount.
  useEffect(() => {
    const localProfiles = getHouseholdProfiles();
    const rawActiveId = getActiveProfileId();
    const initialProfileId = localProfiles.some((p) => p.id === rawActiveId)
      ? rawActiveId
      : setActiveProfileId(localProfiles[0].id);
    setProfiles(localProfiles);
    setActiveProfileState(initialProfileId);
    setTaste(getTasteProfile());
    setWatchProgress(getWatchProgressList());
    setProfileReady(true);

    syncStateFromServer(initialProfileId).then((synced) => {
      setProfiles(synced.profiles);
      setActiveProfileState(synced.profileId);
      setTaste(getTasteProfile());
      setWatchProgress(synced.watchProgress);
    });
  }, []);

  // Load home trending rows tailored to standard vs Kids mode.
  useEffect(() => {
    if (!profileReady) return;
    let cancelled = false;
    fetch(isKidsMode ? '/api/media/trending?kids=1' : '/api/media/trending')
      .then((res) => res.json())
      .then((data) => {
        if (cancelled) return;
        const base =
          Array.isArray(data.rows) && data.rows.length > 0
            ? { featured: data.featured ?? data.rows[0]?.items?.[0] ?? null, rows: data.rows as CatalogRow[] }
            : fallbackHome(data, isKidsMode);
        const sanitizedRows = isKidsMode
          ? base.rows.map((r) => ({ ...r, items: filterKidsSafeMedia(r.items) })).filter((r) => r.items.length > 0)
          : base.rows;
        const currentTaste = getTasteProfile();
        const built = buildHomeState(sanitizedRows, currentTaste, base.featured, isKidsMode);
        setRawRows(sanitizedRows);
        setTaste(built.storedTaste);
        const nextHome: HomeData = { featured: built.featured, rows: built.rows };
        setHome(nextHome);
        applyUrlParams(nextHome, isKidsMode);
      })
      .catch((err) => {
        if (cancelled) return;
        console.error('Failed to load catalog:', err);
        const fallback = fallbackHome(undefined, isKidsMode);
        setRawRows(fallback.rows);
        setHome(fallback);
        applyUrlParams(fallback, isKidsMode);
      });
    return () => {
      cancelled = true;
    };
  }, [profileReady, isKidsMode, applyUrlParams]);

  const shuffleHome = useCallback(
    (profileOverride?: TasteProfile, kidsOverride?: boolean) => {
      if (!rawRows.length) return;
      const kids = kidsOverride ?? isKidsMode;
      const activeTaste = profileOverride ?? getTasteProfile();
      const built = buildHomeState(rawRows, activeTaste, home?.featured ?? null, kids, home?.featured?.id);
      setTaste(built.storedTaste);
      setHome({ featured: built.featured, rows: built.rows });
    },
    [rawRows, home?.featured, isKidsMode]
  );

  const handleSwitchProfile = useCallback(
    (profileId: string, profilesOverride?: HouseholdMember[]) => {
      const list = profilesOverride ?? profiles;
      const targetMember = list.find((p) => p.id === profileId) || list[0];
      const nextIsKids = Boolean(targetMember?.isKids);
      const normalized = setActiveProfileId(profileId);
      setActiveProfileState(normalized);
      const nextTaste = getTasteProfile();
      setTaste(nextTaste);
      setWatchProgress(getWatchProgressList());

      if (nextIsKids !== isKidsMode) {
        // Clear stale rows immediately while trending?kids=... reloads.
        setHome(null);
        setRawRows([]);
        if (nextIsKids) {
          if (selectedMedia && !isKidsSafeMedia(selectedMedia)) {
            setSelectedMedia(null);
            updateUrl(activeTab, null, null);
          } else if (activeGenre) {
            setActiveGenre(null);
            updateUrl(activeTab, null, selectedMedia);
          }
        }
      } else {
        shuffleHome(nextTaste, nextIsKids);
      }

      syncStateFromServer(normalized).then((synced) => {
        setProfiles(synced.profiles);
        setActiveProfileState(synced.profileId);
        setWatchProgress(synced.watchProgress);
      });
    },
    [profiles, isKidsMode, selectedMedia, activeGenre, activeTab, updateUrl, shuffleHome]
  );

  const handleAddProfile = useCallback(
    async (name: string, isKids = false) => {
      const trimmed = name.trim();
      if (!trimmed) return;
      const baseId =
        trimmed
          .toLowerCase()
          .replace(/[^a-z0-9_-]/g, '-')
          .replace(/-+/g, '-')
          .replace(/^-|-$/g, '') || `member-${Date.now()}`;
      let uniqueId = baseId;
      let suffix = 2;
      while (profiles.some((p) => p.id === uniqueId)) {
        uniqueId = `${baseId}-${suffix++}`;
      }
      const color = isKids ? 'emerald' : PROFILE_COLORS[profiles.length % PROFILE_COLORS.length];
      const created: HouseholdMember = {
        id: uniqueId,
        name: trimmed,
        color,
        ...(isKids ? { isKids: true } : {}),
      };
      const saved = saveHouseholdProfiles([...profiles, created], false);
      setProfiles(saved);
      setActiveProfileId(uniqueId);
      setActiveProfileState(uniqueId);
      const nextTaste = getTasteProfile();
      setTaste(nextTaste);
      setWatchProgress(getWatchProgressList());
      if (Boolean(isKids) !== isKidsMode) {
        setHome(null);
        setRawRows([]);
      } else {
        shuffleHome(nextTaste, isKids);
      }
      await flushStateToServer();
    },
    [profiles, isKidsMode, shuffleHome]
  );

  const handleSaveProfiles = useCallback(
    async (nextProfiles: HouseholdMember[], deletedIds: string[]) => {
      for (const id of deletedIds) {
        clearProfileLocalState(id);
      }
      const saved = saveHouseholdProfiles(nextProfiles, false);
      setProfiles(saved);

      const activeStillExists = saved.some((p) => p.id === activeProfileId);
      const nextId = activeStillExists ? activeProfileId : saved[0].id;
      const nextMember = saved.find((p) => p.id === nextId) || saved[0];
      const nextIsKids = Boolean(nextMember?.isKids);

      if (!activeStillExists) {
        setActiveProfileId(nextId);
        setActiveProfileState(nextId);
        const nextTaste = getTasteProfile();
        setTaste(nextTaste);
        setWatchProgress(getWatchProgressList());
      }
      if (nextIsKids !== isKidsMode) {
        setHome(null);
        setRawRows([]);
        if (nextIsKids && selectedMedia && !isKidsSafeMedia(selectedMedia)) {
          setSelectedMedia(null);
        }
      } else if (!activeStillExists && rawRows.length) {
        shuffleHome(getTasteProfile(), nextIsKids);
      }
      await flushStateToServer();
    },
    [activeProfileId, isKidsMode, selectedMedia, rawRows.length, shuffleHome]
  );

  const handleToggleFavoriteGenre = (genre: string) => {
    const nextProfile = toggleFavoriteGenre(genre);
    shuffleHome(nextProfile);
  };

  const handleToggleLike = (media: MediaItem) => {
    const { profile: nextProfile } = toggleLikedMedia(media);
    setTaste(nextProfile);
    if (rawRows.length) {
      const effectiveTaste = sanitizeTasteForProfile(nextProfile, isKidsMode);
      setHome((prev) =>
        prev
          ? {
              ...prev,
              rows: personalizeRows(rawRows, effectiveTaste),
            }
          : prev
      );
    }
  };

  const switchTab = (tab: AppTab) => {
    setActiveTab(tab);
    setActiveGenre(null);
    updateUrl(tab, null, selectedMedia);
    window.scrollTo({ top: 0 });
  };

  const selectMedia = useCallback(
    (media: MediaItem | null) => {
      if (media) {
        const nextProfile = recordMediaInteraction(media, 1);
        setTaste(nextProfile);
      }
      setSelectedMedia(media);
      updateUrl(activeTab, activeGenre, media);
    },
    [activeTab, activeGenre, updateUrl]
  );

  const handleGenreChange = (genre: string | null) => {
    setActiveGenre(genre);
    updateUrl(activeTab, genre, selectedMedia);
  };

  const toPlayable = (sources: StreamSource[]): PlayableSource[] =>
    sources
      .filter((s) => (s.type === 'torrent' ? Boolean(s.magnet) : Boolean(s.magnet || s.url)))
      .map((s) => (s.magnet ? { magnet: s.magnet, fileIdx: s.fileIdx, label: s.name } : { url: s.url, label: s.name }));

  const handlePlaySource = useCallback(
    (sources: StreamSource[], media: MediaItem, context: PlaybackContext) => {
      const playable = toPlayable(sources);
      if (!playable.length) return;
      const nextProfile = recordMediaInteraction(media, 3);
      setTaste(nextProfile);
      selectMedia(null);
      const episodeTag = context.season ? ` · S${context.season}E${context.episode}` : '';
      const subtitleQuery: SubtitleQuery | undefined = media.anime
        ? media.anime.imdbId
          ? { imdbId: media.anime.imdbId, mediaType: media.mediaType, season: media.anime.season ?? 1, episode: context.episode }
          : undefined
        : media.imdbId
          ? { imdbId: media.imdbId, mediaType: media.mediaType, season: context.season, episode: context.episode }
          : undefined;
      setActivePlayer({
        key: `${media.id}-${context.season ?? 0}-${context.episode ?? 0}`,
        title: `${media.title}${episodeTag}`,
        sources: playable,
        initialStartTime: context.startAt || 0,
        subtitleQuery,
        media,
        season: context.season,
        episode: context.episode,
      });
    },
    [selectMedia]
  );

  const handlePlayMedia = (media: MediaItem) => {
    selectMedia(media);
  };

  const handlePlayLiveChannel = (channel: LiveChannel, channelList?: LiveChannel[]) => {
    const list = channelList && channelList.length > 0 ? channelList : [channel];
    const idx = Math.max(0, list.findIndex((c) => c.id === channel.id || c.streamUrl === channel.streamUrl));
    setActivePlayer({
      key: `live-${channel.id}`,
      title: channel.name,
      sources: list.map((c) => ({ url: c.streamUrl, label: c.name })),
      initialSourceIndex: idx,
      isLive: channel.isLive,
    });
  };

  const handleWatchProgress = useCallback(
    (position: number, duration: number) => {
      if (!activePlayer?.media) return;
      const updated = saveWatchProgress(
        activePlayer.media,
        position,
        duration,
        activePlayer.season,
        activePlayer.episode
      );
      setWatchProgress(updated);
    },
    [activePlayer]
  );

  const nextEpisodeAction = useMemo(() => {
    if (!activePlayer?.media || activePlayer.media.mediaType !== 'tv') return undefined;
    const curSeason = activePlayer.season ?? 1;
    const curEpisode = activePlayer.episode ?? 1;
    const next = findNextEpisode(activePlayer.media, curSeason, curEpisode);
    if (!next) return undefined;
    const media = activePlayer.media;
    return {
      label: next.label,
      onPlayNext: () => {
        const req = buildMediaRequest(media, next.season, next.episode);
        fetch(req.url, { headers: req.headers })
          .then((r) => r.json())
          .then((data) => {
            const ranked = rankSources(Array.isArray(data.streams) ? data.streams : []).filter(
              (s) => s.magnet || s.url
            );
            if (ranked.length > 0) {
              handlePlaySource(ranked.slice(0, 8), media, {
                season: next.season,
                episode: next.episode,
                startAt: 0,
              });
            }
          })
          .catch(() => {});
      },
    };
  }, [activePlayer, handlePlaySource]);

  const continueWatchingItems = useMemo(() => {
    const items = watchProgress.map((w) => w.media);
    return isKidsMode ? filterKidsSafeMedia(items) : items;
  }, [watchProgress, isKidsMode]);

  const progressByMediaId = useMemo(() => {
    const map: Record<string, { ratio: number; label: string }> = {};
    for (const entry of watchProgress) {
      const ratio = entry.duration > 0 ? Math.min(1, entry.position / entry.duration) : 0.15;
      const minsLeft = entry.duration > entry.position ? Math.max(1, Math.round((entry.duration - entry.position) / 60)) : 0;
      const epTag =
        entry.media.mediaType === 'tv' && entry.episode
          ? entry.media.anime
            ? `Ep ${entry.episode}`
            : `S${entry.season ?? 1} · E${entry.episode}`
          : '';
      const timeTag = minsLeft > 0 ? `${minsLeft}m left` : `${Math.floor(entry.position / 60)}m watched`;
      map[String(entry.media.id)] = {
        ratio,
        label: [epTag, timeTag].filter(Boolean).join(' · '),
      };
    }
    return map;
  }, [watchProgress]);

  const tasteGenres = isKidsMode ? KIDS_TASTE_GENRES : POPULAR_TASTE_GENRES;

  return (
    <div className="min-h-screen flex flex-col">
      <Navbar
        activeTab={activeTab}
        setActiveTab={switchTab}
        onSelectMedia={selectMedia}
        onSelectLiveChannel={(ch) => handlePlayLiveChannel(ch)}
        onOpenSettingsModal={() => setIsSettingsModalOpen(true)}
        profiles={profiles}
        activeProfileId={activeProfileId}
        onSwitchProfile={(id) => handleSwitchProfile(id)}
        onAddProfile={(name, kids) => void handleAddProfile(name, kids)}
        onOpenProfileManager={() => setIsProfileManagerOpen(true)}
      />

      {activeTab === 'home' ? (
        <main className="flex-1 pb-24">
          <HeroBanner
            media={home?.featured ?? null}
            onPlay={handlePlayMedia}
            onMoreInfo={selectMedia}
            onShuffle={() => shuffleHome()}
            isLiked={home?.featured ? isMediaLiked(home.featured.id, taste) : false}
            onToggleLike={handleToggleLike}
          />
          <div className="relative z-10 -mt-8 sm:-mt-12 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 space-y-10 sm:space-y-12">
            {/* Personalization & Shuffle Bar */}
            <div className="flex flex-wrap items-center gap-2 pt-2">
              {isKidsMode ? (
                <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-300 mr-1">
                  <ShieldCheck className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                  <span className="text-trim">Kids taste ({getMemberDisplayName(activeMember)}):</span>
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-zinc-400 mr-1">
                  <Sparkles className="w-3.5 h-3.5 text-rose-400 shrink-0" />
                  <span className="text-trim">Your taste:</span>
                </span>
              )}
              {tasteGenres.map((g) => {
                const active = taste.favoriteGenres.includes(g);
                return (
                  <button
                    key={g}
                    type="button"
                    aria-pressed={active}
                    onClick={() => handleToggleFavoriteGenre(g)}
                    className={`h-7 px-3 rounded-full inline-flex items-center justify-center text-xs font-medium leading-none transition-colors ${
                      active
                        ? 'bg-gradient-to-r from-rose-500 to-orange-400 text-white shadow-sm shadow-rose-950/50'
                        : 'bg-white/[0.05] text-zinc-300 ring-1 ring-inset ring-white/[0.08] hover:bg-white/10 hover:text-white'
                    }`}
                  >
                    <span className="text-trim">{g}</span>
                  </button>
                );
              })}
              <button
                type="button"
                onClick={() => shuffleHome()}
                className="ml-auto h-7 px-3.5 rounded-full inline-flex items-center justify-center gap-1.5 bg-white/[0.07] hover:bg-white/15 text-xs font-medium leading-none text-zinc-200 hover:text-white ring-1 ring-inset ring-white/10 transition-colors"
              >
                <Shuffle className="w-3.5 h-3.5 text-rose-400 shrink-0" />
                <span className="text-trim">Shuffle picks</span>
              </button>
            </div>

            {continueWatchingItems.length > 0 && (
              <MediaCarousel
                title="Continue Watching"
                subtitle="Pick up right where you left off"
                items={continueWatchingItems}
                onSelectMedia={selectMedia}
                onPlayMedia={handlePlayMedia}
                onRemoveItem={(m) => setWatchProgress(removeWatchProgress(m.id))}
                progressByMediaId={progressByMediaId}
              />
            )}

            {home
              ? home.rows.map((row) => (
                  <MediaCarousel
                    key={row.key}
                    title={row.title}
                    subtitle={row.subtitle}
                    items={row.items}
                    onSelectMedia={selectMedia}
                    onPlayMedia={handlePlayMedia}
                  />
                ))
              : Array.from({ length: 4 }).map((_, i) => <CarouselSkeleton key={i} />)}
          </div>
        </main>
      ) : (
        <main className="flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 pt-32 md:pt-28 pb-24">
          {activeTab === 'movies' && (
            <>
              <PageHeader
                eyebrow={isKidsMode ? 'Kids · Browse' : 'Browse'}
                title={isKidsMode ? 'Kids Movies' : 'Movies'}
                description={isKidsMode ? 'Family-friendly films and animations, by genre or by year.' : 'Popular films, by genre or by year.'}
              />
              <CatalogBrowser
                key={`movies-${isKidsMode ? 'kids' : 'std'}`}
                mediaType="movie"
                onSelectMedia={selectMedia}
                initialGenre={activeGenre}
                onGenreChange={handleGenreChange}
                isKidsMode={isKidsMode}
              />
            </>
          )}
          {activeTab === 'tv' && (
            <>
              <PageHeader
                eyebrow={isKidsMode ? 'Kids · Browse' : 'Browse'}
                title={isKidsMode ? 'Kids Series' : 'Series'}
                description={isKidsMode ? 'Family series and animated shows, ready to stream.' : 'Every season and episode, ready to stream.'}
              />
              <CatalogBrowser
                key={`tv-${isKidsMode ? 'kids' : 'std'}`}
                mediaType="tv"
                onSelectMedia={selectMedia}
                initialGenre={activeGenre}
                onGenreChange={handleGenreChange}
                isKidsMode={isKidsMode}
              />
            </>
          )}
          {activeTab === 'anime' && (
            <>
              <PageHeader
                eyebrow={isKidsMode ? 'Kids · Browse' : 'Browse'}
                title={isKidsMode ? 'Kids Anime' : 'Anime'}
                description="Trending, seasonal and all-time favourites, powered by AniList."
              />
              <CatalogBrowser
                key={`anime-${isKidsMode ? 'kids' : 'std'}`}
                mediaType="anime"
                onSelectMedia={selectMedia}
                initialGenre={activeGenre}
                onGenreChange={handleGenreChange}
                isKidsMode={isKidsMode}
              />
            </>
          )}
          {activeTab === 'live' && (
            <LiveChannelsGrid
              key={`${activeProfileId}-${isKidsMode ? 'kids' : 'std'}`}
              onPlayChannel={handlePlayLiveChannel}
              isKidsMode={isKidsMode}
            />
          )}
        </main>
      )}

      {selectedMedia && (
        <MediaDetailModal
          key={`${selectedMedia.mediaType}-${selectedMedia.id}`}
          media={selectedMedia}
          onClose={() => selectMedia(null)}
          onPlaySource={handlePlaySource}
          isLiked={isMediaLiked(selectedMedia.id, taste)}
          onToggleLike={handleToggleLike}
        />
      )}

      {isSettingsModalOpen && <SettingsModal onClose={() => setIsSettingsModalOpen(false)} />}

      {isProfileManagerOpen && (
        <ProfileManagerModal
          profiles={profiles}
          activeProfileId={activeProfileId}
          onSaveProfiles={(next, deleted) => void handleSaveProfiles(next, deleted)}
          onSwitchProfile={(id) => handleSwitchProfile(id)}
          onClose={() => setIsProfileManagerOpen(false)}
        />
      )}

      {activePlayer && (
        <VideoPlayerModal
          key={activePlayer.key}
          title={activePlayer.title}
          sources={activePlayer.sources}
          initialSourceIndex={activePlayer.initialSourceIndex}
          initialStartTime={activePlayer.initialStartTime}
          isLive={activePlayer.isLive}
          subtitleQuery={activePlayer.subtitleQuery}
          onWatchProgress={handleWatchProgress}
          nextEpisode={nextEpisodeAction}
          onClose={() => setActivePlayer(null)}
        />
      )}
    </div>
  );
}
