import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { LiveChannel, MediaItem } from '../src/types/index.ts';
import {
  KIDS_CINEMETA_GENRES,
  KIDS_TASTE_GENRES,
  filterKidsLiveChannels,
  filterKidsSafeMedia,
  isKidsSafeMedia,
} from '../src/lib/kidsFilter.ts';

function item(partial: Partial<MediaItem>): MediaItem {
  return {
    id: 'test-1',
    title: 'Test Title',
    overview: 'Overview',
    posterPath: null,
    backdropPath: null,
    voteAverage: 8.0,
    mediaType: 'movie',
    genres: [],
    ...partial,
  };
}

test('isKidsSafeMedia allows Family/Animation titles and blocks mature genres', () => {
  assert.equal(isKidsSafeMedia(item({ title: 'Spirited Away', genres: ['Animation', 'Adventure', 'Family'] })), true);
  assert.equal(isKidsSafeMedia(item({ title: 'The Lion King', genres: ['Family', 'Animation', 'Drama'] })), true);
  assert.equal(isKidsSafeMedia(item({ title: 'Haikyuu!!', genres: ['Comedy', 'Sports'], anime: { anilistId: 20583 } })), true);
  assert.equal(isKidsSafeMedia(item({ title: 'Bo Burnham: Inside', genres: ['Documentary', 'Comedy', 'Drama'] })), false);
  assert.equal(isKidsSafeMedia(item({ title: 'Rick and Morty', genres: ['Animation', 'Adventure', 'Comedy', 'Sci-Fi'] })), false);
  assert.equal(isKidsSafeMedia(item({ title: 'Attack on Titan', genres: ['Animation', 'Action', 'Adventure', 'Drama', 'Fantasy'] })), false);
  assert.equal(isKidsSafeMedia(item({ title: 'Saw', genres: ['Horror', 'Mystery', 'Thriller'] })), false);
  assert.equal(isKidsSafeMedia(item({ title: 'Family Thriller', genres: ['Family', 'Thriller'] })), false);
  assert.equal(isKidsSafeMedia(item({ title: 'Oppenheimer', genres: ['Drama', 'History', 'Biography'] })), false);
  assert.equal(isKidsSafeMedia(item({ title: 'Unclassified', genres: [] })), false);

  const filtered = filterKidsSafeMedia([
    item({ id: '1', title: 'Coco', genres: ['Animation', 'Family'] }),
    item({ id: '2', title: 'Breaking Bad', genres: ['Crime', 'Drama', 'Thriller'] }),
  ]);
  assert.deepEqual(
    filtered.map((m) => m.id),
    ['1']
  );
});

test('kids genre lists exclude mature genres and filterKidsLiveChannels keeps Kids category', () => {
  for (const blocked of ['Horror', 'Crime', 'Thriller', 'War', 'Romance', 'Mystery']) {
    assert.equal((KIDS_CINEMETA_GENRES as readonly string[]).includes(blocked), false);
    assert.equal(KIDS_TASTE_GENRES.includes(blocked), false);
  }

  const channels: LiveChannel[] = [
    {
      id: 'cbbc',
      name: 'CBBC',
      category: 'Kids',
      country: 'United Kingdom',
      countryCode: 'GB',
      language: 'English',
      logo: '',
      streamUrl: 'https://example.com/cbbc.m3u8',
      description: '',
      isLive: true,
    },
    {
      id: 'dw',
      name: 'DW News',
      category: 'News',
      country: 'Germany',
      countryCode: 'DE',
      language: 'English',
      logo: '',
      streamUrl: 'https://example.com/dw.m3u8',
      description: '',
      isLive: true,
    },
  ];
  assert.deepEqual(
    filterKidsLiveChannels(channels).map((c) => c.id),
    ['cbbc']
  );
});
