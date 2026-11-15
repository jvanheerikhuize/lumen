import type { MediaItem } from '../types/index.ts';

const PROGRESS_STORAGE_KEY = 'lumen_watch_progress_v1';
const FAVORITE_CHANNELS_KEY = 'lumen_favorite_channels_v1';
const ACTIVE_PROFILE_KEY = 'lumen_active_profile_v1';
const HOUSEHOLD_PROFILES_KEY = 'lumen_household_profiles_v1';
const MAX_PROGRESS_ITEMS = 40;
/** Threshold (97%) at which completed movies clear automatically from Continue Watching. */
export const MOVIE_COMPLETION_RATIO = 0.97;

export interface HouseholdMember {
  id: string;
  name: string;
  nickname?: string;
  color: string;
  isKids?: boolean;
}

export const DEFAULT_HOUSEHOLD_PROFILES: HouseholdMember[] = [
  { id: 'default', name: 'Member 1', color: 'rose' },
  { id: 'member-2', name: 'Member 2', color: 'sky' },
  { id: 'kids', name: 'Kids', color: 'emerald', isKids: true },
];

export function getMemberDisplayName(member: Pick<HouseholdMember, 'name' | 'nickname'> | null | undefined): string {
  if (!member) return 'Member';
  const nick = member.nickname?.trim();
  return nick || member.name.trim() || 'Member';
}

function normalizeProfiles(raw: unknown): HouseholdMember[] {
  if (!Array.isArray(raw) || raw.length === 0) return DEFAULT_HOUSEHOLD_PROFILES;
  const valid = raw
    .filter((p): p is HouseholdMember => Boolean(p && typeof p === 'object' && typeof (p as HouseholdMember).id === 'string' && (p as HouseholdMember).id.trim() && typeof (p as HouseholdMember).name === 'string' && (p as HouseholdMember).name.trim()))
    .map((p) => {
      const nickname = typeof p.nickname === 'string' ? p.nickname.trim() : '';
      const isKids = typeof p.isKids === 'boolean' ? p.isKids : p.id === 'kids';
      return {
        id: p.id.trim(),
        name: p.name.trim(),
        ...(nickname ? { nickname } : {}),
        color: typeof p.color === 'string' && p.color.trim() ? p.color.trim() : 'rose',
        ...(isKids ? { isKids: true } : {}),
      };
    });
  return valid.length > 0 ? valid : DEFAULT_HOUSEHOLD_PROFILES;
}

export function getActiveProfileId(): string {
  if (typeof window === 'undefined') return 'default';
  try {
    return localStorage.getItem(ACTIVE_PROFILE_KEY) || 'default';
  } catch {
    return 'default';
  }
}

export function setActiveProfileId(profileId: string): string {
  const next = profileId.trim() || 'default';
  if (typeof window !== 'undefined') {
    try {
      localStorage.setItem(ACTIVE_PROFILE_KEY, next);
    } catch {
      // ignore
    }
  }
  return next;
}

export function getHouseholdProfiles(): HouseholdMember[] {
  if (typeof window === 'undefined') return DEFAULT_HOUSEHOLD_PROFILES;
  try {
    const raw = localStorage.getItem(HOUSEHOLD_PROFILES_KEY);
    if (!raw) return DEFAULT_HOUSEHOLD_PROFILES;
    return normalizeProfiles(JSON.parse(raw));
  } catch {
    return DEFAULT_HOUSEHOLD_PROFILES;
  }
}

export function saveHouseholdProfiles(profiles: HouseholdMember[], syncServer = true): HouseholdMember[] {
  const clean = normalizeProfiles(profiles);
  if (typeof window !== 'undefined') {
    try {
      localStorage.setItem(HOUSEHOLD_PROFILES_KEY, JSON.stringify(clean));
    } catch {
      // ignore
    }
    if (syncServer) pushStateToServer();
  }
  return clean;
}

export function clearProfileLocalState(profileId: string): void {
  if (typeof window === 'undefined') return;
  const trimmed = profileId.trim();
  if (!trimmed) return;
  try {
    localStorage.removeItem(progressStorageKey(trimmed));
    localStorage.removeItem(favoriteChannelsStorageKey(trimmed));
    const tasteKey = trimmed === 'default' ? 'screentime_taste_v1' : `screentime_taste_v1:${trimmed}`;
    localStorage.removeItem(tasteKey);
  } catch {
    // ignore
  }
}

function progressStorageKey(profileId = getActiveProfileId()): string {
  return profileId === 'default' ? PROGRESS_STORAGE_KEY : `${PROGRESS_STORAGE_KEY}:${profileId}`;
}

function favoriteChannelsStorageKey(profileId = getActiveProfileId()): string {
  return profileId === 'default' ? FAVORITE_CHANNELS_KEY : `${FAVORITE_CHANNELS_KEY}:${profileId}`;
}

export interface EpisodeProgress {
  position: number;
  duration: number;
  updatedAt: number;
}

export interface WatchProgressEntry {
  media: MediaItem;
  season?: number;
  episode?: number;
  position: number;
  duration: number;
  updatedAt: number;
  /** Per-episode progress map keyed by `${season}:${episode}` for TV/anime series. */
  episodes?: Record<string, EpisodeProgress>;
}

/** Normalize a media item before storing in localStorage so we don't bloat storage with 500-episode season trees. */
function compactMediaItem(media: MediaItem): MediaItem {
  const copy: MediaItem = { ...media };
  delete copy.seasons;
  return copy;
}

export function getWatchProgressList(): WatchProgressEntry[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem(progressStorageKey());
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((item): item is WatchProgressEntry => Boolean(item?.media?.id && typeof item.position === 'number'))
      .sort((a, b) => b.updatedAt - a.updatedAt);
  } catch {
    return [];
  }
}

function writeWatchProgressList(list: WatchProgressEntry[], syncServer = true): WatchProgressEntry[] {
  const sorted = [...list].sort((a, b) => b.updatedAt - a.updatedAt).slice(0, MAX_PROGRESS_ITEMS);
  if (typeof window !== 'undefined') {
    try {
      localStorage.setItem(progressStorageKey(), JSON.stringify(sorted));
    } catch {
      // ignore storage quota errors
    }
    if (syncServer) pushStateToServer();
  }
  return sorted;
}

export function getMediaProgress(mediaId: string | number, list = getWatchProgressList()): WatchProgressEntry | null {
  const key = String(mediaId);
  return list.find((entry) => String(entry.media.id) === key || (entry.media.imdbId && entry.media.imdbId === key)) || null;
}

export function getEpisodeProgress(
  mediaId: string | number,
  season: number,
  episode: number,
  list = getWatchProgressList()
): EpisodeProgress | null {
  const entry = getMediaProgress(mediaId, list);
  if (!entry) return null;
  const epKey = `${season}:${episode}`;
  if (entry.episodes?.[epKey]) return entry.episodes[epKey];
  if (entry.season === season && entry.episode === episode) {
    return { position: entry.position, duration: entry.duration, updatedAt: entry.updatedAt };
  }
  return null;
}

/**
 * Record playback progress for a movie or episode.
 * - For movies: if >= 97% complete, removes the movie from Continue Watching (finished).
 * - For series: keeps the series in Continue Watching with the latest season/episode and per-episode progress.
 */
export function saveWatchProgress(
  media: MediaItem,
  position: number,
  duration: number,
  season?: number,
  episode?: number
): WatchProgressEntry[] {
  if (!Number.isFinite(position) || position < 5) return getWatchProgressList();
  const safeDuration = Number.isFinite(duration) && duration > 0 ? duration : 0;
  const ratio = safeDuration > 0 ? position / safeDuration : 0;

  const current = getWatchProgressList();
  const key = String(media.id);
  const existing = current.find((e) => String(e.media.id) === key);

  // If a movie is finished (>= 97% watched), remove it from Continue Watching.
  if (media.mediaType === 'movie' && ratio >= MOVIE_COMPLETION_RATIO) {
    return removeWatchProgress(media.id);
  }

  const now = Date.now();
  const episodesMap: Record<string, EpisodeProgress> = { ...(existing?.episodes || {}) };
  if (season !== undefined && episode !== undefined) {
    episodesMap[`${season}:${episode}`] = {
      position: Math.floor(position),
      duration: Math.floor(safeDuration),
      updatedAt: now,
    };
  }

  const updatedEntry: WatchProgressEntry = {
    media: compactMediaItem(media),
    season,
    episode,
    position: Math.floor(position),
    duration: Math.floor(safeDuration),
    updatedAt: now,
    ...(Object.keys(episodesMap).length > 0 ? { episodes: episodesMap } : {}),
  };

  const next = [updatedEntry, ...current.filter((e) => String(e.media.id) !== key)];
  return writeWatchProgressList(next);
}

export function removeWatchProgress(mediaId: string | number): WatchProgressEntry[] {
  const key = String(mediaId);
  pendingRemovedMediaIds.add(key);
  const next = getWatchProgressList().filter(
    (e) => String(e.media.id) !== key && (!e.media.imdbId || e.media.imdbId !== key)
  );
  return writeWatchProgressList(next);
}

export function getFavoriteChannelIds(): string[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem(favoriteChannelsStorageKey());
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === 'string') : [];
  } catch {
    return [];
  }
}

export function toggleFavoriteChannel(channelId: string): string[] {
  const current = getFavoriteChannelIds();
  const exists = current.includes(channelId);
  if (exists) {
    pendingRemovedChannelIds.add(channelId);
  } else {
    pendingRemovedChannelIds.delete(channelId);
  }
  const next = exists ? current.filter((id) => id !== channelId) : [channelId, ...current];
  if (typeof window !== 'undefined') {
    try {
      localStorage.setItem(favoriteChannelsStorageKey(), JSON.stringify(next));
    } catch {
      // ignore storage errors
    }
    pushStateToServer();
  }
  return next;
}

let syncTimeout: ReturnType<typeof setTimeout> | null = null;
const pendingRemovedMediaIds = new Set<string>();
const pendingRemovedChannelIds = new Set<string>();

/** Immediately push client state to `/api/state`, cancelling any pending debounce timer. */
export async function flushStateToServer(): Promise<void> {
  if (typeof window === 'undefined') return;
  if (syncTimeout) {
    clearTimeout(syncTimeout);
    syncTimeout = null;
  }
  const profileId = getActiveProfileId();
  const profiles = getHouseholdProfiles();
  const watchProgress = getWatchProgressList();
  const favoriteChannelIds = getFavoriteChannelIds();
  const removedMediaIds = Array.from(pendingRemovedMediaIds);
  const removedChannelIds = Array.from(pendingRemovedChannelIds);
  pendingRemovedMediaIds.clear();
  pendingRemovedChannelIds.clear();
  try {
    await fetch(`/api/state?profile=${encodeURIComponent(profileId)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ profileId, profiles, watchProgress, favoriteChannelIds, removedMediaIds, removedChannelIds }),
    });
  } catch {
    // ignore background sync failures
  }
}

/** Debounced background push of client watch progress & favorites to `/api/state` for LAN device sync. */
export function pushStateToServer(): void {
  if (typeof window === 'undefined') return;
  if (syncTimeout) clearTimeout(syncTimeout);
  syncTimeout = setTimeout(() => {
    syncTimeout = null;
    void flushStateToServer();
  }, 400);
}

/** Merge server-side LAN state with local state for the active household member profile. */
export async function syncStateFromServer(profileOverride?: string): Promise<{
  profileId: string;
  profiles: HouseholdMember[];
  watchProgress: WatchProgressEntry[];
  favoriteChannelIds: string[];
}> {
  const profileId = profileOverride ? setActiveProfileId(profileOverride) : getActiveProfileId();
  const localProfiles = getHouseholdProfiles();
  const localProgress = getWatchProgressList();
  const localFavorites = getFavoriteChannelIds();
  try {
    const res = await fetch(`/api/state?profile=${encodeURIComponent(profileId)}`, { cache: 'no-store' });
    if (!res.ok) return { profileId, profiles: localProfiles, watchProgress: localProgress, favoriteChannelIds: localFavorites };
    const data = await res.json();
    const serverProfiles: HouseholdMember[] =
      Array.isArray(data?.profiles) && data.profiles.length > 0 ? data.profiles : localProfiles;
    const serverProgress: WatchProgressEntry[] = Array.isArray(data?.watchProgress) ? data.watchProgress : [];
    const serverFavorites: string[] = Array.isArray(data?.favoriteChannelIds) ? data.favoriteChannelIds : [];

    const cleanProfiles = saveHouseholdProfiles(serverProfiles, false);
    const activeValid = cleanProfiles.some((p) => p.id === profileId);
    const effectiveProfileId = activeValid ? profileId : setActiveProfileId(cleanProfiles[0].id);

    if (!activeValid) {
      return {
        profileId: effectiveProfileId,
        profiles: cleanProfiles,
        watchProgress: getWatchProgressList(),
        favoriteChannelIds: getFavoriteChannelIds(),
      };
    }

    // Merge watchProgress by media.id, keeping whichever entry has the newer updatedAt timestamp.
    const byId = new Map<string, WatchProgressEntry>();
    for (const item of [...serverProgress, ...localProgress]) {
      if (!item?.media?.id) continue;
      const key = String(item.media.id);
      const prev = byId.get(key);
      if (!prev || item.updatedAt > prev.updatedAt) {
        byId.set(key, {
          ...item,
          episodes: { ...(prev?.episodes || {}), ...(item.episodes || {}) },
        });
      }
    }
    const mergedProgress = writeWatchProgressList(Array.from(byId.values()), false);

    // Union favorite channels
    const mergedFavorites = Array.from(new Set([...localFavorites, ...serverFavorites]));
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem(favoriteChannelsStorageKey(effectiveProfileId), JSON.stringify(mergedFavorites));
      } catch {
        // ignore
      }
    }
    return { profileId: effectiveProfileId, profiles: cleanProfiles, watchProgress: mergedProgress, favoriteChannelIds: mergedFavorites };
  } catch {
    return { profileId, profiles: localProfiles, watchProgress: localProgress, favoriteChannelIds: localFavorites };
  }
}
