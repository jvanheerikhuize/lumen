import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { MediaItem } from '../src/types/index.ts';
import {
  clearProfileLocalState,
  getEpisodeProgress,
  getFavoriteChannelIds,
  getHouseholdProfiles,
  getMediaProgress,
  getMemberDisplayName,
  getWatchProgressList,
  removeWatchProgress,
  saveHouseholdProfiles,
  saveWatchProgress,
  setActiveProfileId,
  toggleFavoriteChannel,
} from '../src/lib/watchProgress.ts';
import { getTasteProfile, toggleFavoriteGenre } from '../src/lib/tasteProfile.ts';

function mockLocalStorage() {
  const store = new Map<string, string>();
  (globalThis as unknown as { window: unknown; localStorage: Storage }).window = {};
  (globalThis as unknown as { localStorage: Storage }).localStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => {
      store.set(k, v);
    },
    removeItem: (k: string) => {
      store.delete(k);
    },
    clear: () => store.clear(),
    key: () => null,
    length: 0,
  };
}

function makeMedia(id: string, title: string, mediaType: 'movie' | 'tv'): MediaItem {
  return {
    id,
    imdbId: id,
    title,
    overview: `${title} overview`,
    posterPath: null,
    backdropPath: null,
    voteAverage: 8.0,
    mediaType,
    genres: ['Sci-Fi'],
  };
}

test('saveWatchProgress records movie progress and removes movie when >= 94% complete', () => {
  mockLocalStorage();
  const movie = makeMedia('tt100', 'Interstellar', 'movie');

  const list1 = saveWatchProgress(movie, 1200, 7200);
  assert.equal(list1.length, 1);
  assert.equal(getMediaProgress('tt100')?.position, 1200);

  // Finishing the movie (>= 94%) removes it from Continue Watching
  const list2 = saveWatchProgress(movie, 7000, 7200);
  assert.equal(list2.length, 0);
  assert.equal(getMediaProgress('tt100'), null);
});

test('saveWatchProgress remembers last-watched season/episode and per-episode progress for series', () => {
  mockLocalStorage();
  const show = makeMedia('tt200', 'Severance', 'tv');

  saveWatchProgress(show, 1400, 2800, 1, 2);
  saveWatchProgress(show, 600, 2900, 1, 3);

  const entry = getMediaProgress('tt200');
  assert.equal(entry?.season, 1);
  assert.equal(entry?.episode, 3);
  assert.equal(getEpisodeProgress('tt200', 1, 2)?.position, 1400);
  assert.equal(getEpisodeProgress('tt200', 1, 3)?.position, 600);

  removeWatchProgress('tt200');
  assert.equal(getWatchProgressList().length, 0);
});

test('household members each maintain isolated watch progress, favorite channels, and taste profiles', () => {
  mockLocalStorage();
  const show = makeMedia('tt300', 'Arcane', 'tv');

  setActiveProfileId('alice');
  saveWatchProgress(show, 500, 2400, 1, 1);
  toggleFavoriteChannel('dw-news-en');
  toggleFavoriteGenre('Sci-Fi');

  setActiveProfileId('bob');
  saveWatchProgress(show, 1900, 2400, 2, 4);
  toggleFavoriteChannel('nhk-world');
  toggleFavoriteGenre('Comedy');

  setActiveProfileId('alice');
  assert.equal(getMediaProgress('tt300')?.season, 1);
  assert.equal(getMediaProgress('tt300')?.episode, 1);
  assert.equal(getMediaProgress('tt300')?.position, 500);
  assert.deepEqual(getFavoriteChannelIds(), ['dw-news-en']);
  assert.deepEqual(getTasteProfile().favoriteGenres, ['Sci-Fi']);

  setActiveProfileId('bob');
  assert.equal(getMediaProgress('tt300')?.season, 2);
  assert.equal(getMediaProgress('tt300')?.episode, 4);
  assert.equal(getMediaProgress('tt300')?.position, 1900);
  assert.deepEqual(getFavoriteChannelIds(), ['nhk-world']);
  assert.deepEqual(getTasteProfile().favoriteGenres, ['Comedy']);
});

test('household profile nicknames, Kids mode flag, and local cleanup behave consistently', () => {
  mockLocalStorage();
  const defaults = getHouseholdProfiles();
  assert.equal(defaults.find((p) => p.id === 'kids')?.isKids, true);
  assert.equal(getMemberDisplayName({ name: 'Member 1', nickname: '  Jerry  ' }), 'Jerry');
  assert.equal(getMemberDisplayName({ name: 'Member 2', nickname: '   ' }), 'Member 2');

  const saved = saveHouseholdProfiles(
    [
      { id: 'default', name: 'Member 1', nickname: 'Jerry', color: 'amber' },
      { id: 'kids-2', name: 'Leo', nickname: 'Little Explorer', color: 'emerald', isKids: true },
    ],
    false
  );
  assert.equal(saved.length, 2);
  const reloaded = getHouseholdProfiles();
  assert.equal(reloaded[0]?.nickname, 'Jerry');
  assert.equal(reloaded[1]?.isKids, true);

  setActiveProfileId('temp-user');
  saveWatchProgress(makeMedia('tt400', 'Spirited Away', 'movie'), 300, 7200);
  toggleFavoriteChannel('cbbc');
  clearProfileLocalState('temp-user');
  assert.equal(getWatchProgressList().length, 0);
  assert.deepEqual(getFavoriteChannelIds(), []);
});

