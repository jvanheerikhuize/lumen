import type { LiveChannel, MediaItem } from '../types/index.ts';

const KIDS_BLOCKED_GENRES = new Set([
  'horror',
  'crime',
  'thriller',
  'war',
  'psychological',
  'mystery',
  'romance',
  'ecchi',
  'hentai',
  'adult',
  'erotica',
  'film-noir',
  'cyberpunk',
]);

const KIDS_PRIMARY_GENRES = new Set(['family', 'kids', 'children']);

const KIDS_ANIME_ALLOWED_GENRES = new Set([
  'adventure',
  'comedy',
  'fantasy',
  'music',
  'slice of life',
  'sports',
  'sci-fi',
  'family',
  'kids',
]);

export const KIDS_CINEMETA_GENRES = [
  'Family',
  'Animation',
  'Adventure',
  'Comedy',
  'Fantasy',
  'Music',
  'Sport',
] as const;

export const KIDS_ANIME_GENRES = [
  'Adventure',
  'Comedy',
  'Fantasy',
  'Music',
  'Slice of Life',
  'Sports',
] as const;

export const KIDS_TASTE_GENRES = [
  'Family',
  'Animation',
  'Adventure',
  'Comedy',
  'Fantasy',
  'Music',
  'Sport',
];

/** Returns true when a title has kid-appropriate genres and no blocked mature genres. */
export function isKidsSafeMedia(item: MediaItem | null | undefined): boolean {
  if (!item) return false;
  const genres = item.genres || [];
  if (genres.length === 0) return false;
  const isAnime = Boolean(item.anime);
  let hasPrimary = false;
  let hasMatureTone = false;
  let hasAnimeAllowed = false;
  for (const raw of genres) {
    const norm = raw.trim().toLowerCase();
    if (!norm) continue;
    if (KIDS_BLOCKED_GENRES.has(norm)) return false;
    if (KIDS_PRIMARY_GENRES.has(norm)) hasPrimary = true;
    if (norm === 'action' || norm === 'drama') hasMatureTone = true;
    if (isAnime && KIDS_ANIME_ALLOWED_GENRES.has(norm)) hasAnimeAllowed = true;
  }
  return hasPrimary || (hasAnimeAllowed && !hasMatureTone);
}

export function filterKidsSafeMedia(items: MediaItem[]): MediaItem[] {
  return items.filter(isKidsSafeMedia);
}

export function filterKidsLiveChannels(channels: LiveChannel[]): LiveChannel[] {
  return channels.filter((ch) => ch.category === 'Kids');
}
