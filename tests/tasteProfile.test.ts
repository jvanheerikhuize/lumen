import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { MediaItem } from '../src/types/index.ts';
import {
  personalizeRows,
  pickRandomHero,
  recordDiscoveryEvent,
  scoreMediaForUser,
  type TasteProfile,
} from '../src/lib/tasteProfile.ts';

function makeMedia(id: string, title: string, genres: string[], voteAverage = 7.5): MediaItem {
  return {
    id,
    imdbId: id,
    title,
    overview: `${title} overview`,
    posterPath: `https://example.com/${id}-poster.jpg`,
    backdropPath: `https://example.com/${id}-backdrop.jpg`,
    releaseDate: '2025-01-01',
    voteAverage,
    mediaType: 'movie',
    genres,
  };
}

const emptyProfile: TasteProfile = {
  genreScores: {},
  favoriteGenres: [],
  likedItems: [],
  recentHeroIds: [],
};

test('scoreMediaForUser scores movies higher when they match favorite genres and learned genre scores', () => {
  const sciFi = makeMedia('tt1', 'Dune', ['Sci-Fi', 'Adventure'], 8.4);
  const comedy = makeMedia('tt2', 'Superbad', ['Comedy'], 7.6);
  const profile: TasteProfile = {
    ...emptyProfile,
    favoriteGenres: ['Sci-Fi'],
    genreScores: { 'Sci-Fi': 6 },
  };

  assert.ok(scoreMediaForUser(sciFi, profile) > scoreMediaForUser(comedy, profile));
});

test('pickRandomHero avoids recently shown heroes and excludeId', () => {
  const m1 = makeMedia('tt1', 'Movie One', ['Action']);
  const m2 = makeMedia('tt2', 'Movie Two', ['Sci-Fi']);
  const m3 = makeMedia('tt3', 'Movie Three', ['Thriller']);
  const rows = [{ key: 'popular-movies', items: [m1, m2, m3] }];

  const profile: TasteProfile = {
    ...emptyProfile,
    recentHeroIds: ['tt1'],
  };

  // With tt1 in recentHeroIds and tt2 excluded, only tt3 should be chosen.
  const picked = pickRandomHero(rows, profile, 'tt2');
  assert.equal(picked?.id, 'tt3');
});

test('pickRandomHero maintains 12-launch uniqueness across 30 simulated launches and recordDiscoveryEvent accumulates counters', () => {
  const catalog = Array.from({ length: 24 }, (_, idx) =>
    makeMedia(`tt${idx + 1}`, `Movie ${idx + 1}`, idx % 2 === 0 ? ['Sci-Fi'] : ['Drama'], 7.0 + (idx % 3))
  );
  const rows = [{ key: 'popular-movies', items: catalog }];
  let recentHeroIds: string[] = [];
  let prevId: string | undefined;
  const chosenIds: string[] = [];

  for (let launch = 0; launch < 30; launch++) {
    const picked = pickRandomHero(rows, { ...emptyProfile, recentHeroIds }, prevId);
    assert.ok(picked);
    const pickedId = String(picked.id);
    assert.notEqual(pickedId, prevId);
    chosenIds.push(pickedId);
    recentHeroIds = [pickedId, ...recentHeroIds.filter((id) => id !== pickedId)].slice(0, 12);
    prevId = pickedId;
  }

  for (let start = 0; start <= chosenIds.length - 12; start++) {
    assert.equal(new Set(chosenIds.slice(start, start + 12)).size, 12);
  }

  let tracked: TasteProfile = { ...emptyProfile };
  tracked = recordDiscoveryEvent('launch', tracked);
  tracked = recordDiscoveryEvent('hero', tracked);
  tracked = recordDiscoveryEvent('picked_for_you', tracked);
  tracked = recordDiscoveryEvent('search', tracked);
  tracked = recordDiscoveryEvent('reroll', tracked);
  assert.equal(tracked.discoveryMetrics?.launches, 1);
  assert.equal(tracked.discoveryMetrics?.heroOpens, 1);
  assert.equal(tracked.discoveryMetrics?.pickedForYouOpens, 1);
  assert.equal(tracked.discoveryMetrics?.searchOpens, 1);
  assert.equal(tracked.discoveryMetrics?.rerolls, 1);
});

test('personalizeRows creates Picked for You and Your Liked Titles rows and prioritizes favorite genres', () => {
  const m1 = makeMedia('tt1', 'Blade Runner 2049', ['Sci-Fi', 'Thriller'], 8.1);
  const m2 = makeMedia('tt2', 'Arrival', ['Sci-Fi', 'Drama'], 7.9);
  const m3 = makeMedia('tt3', 'The Notebook', ['Romance', 'Drama'], 7.8);

  const profile: TasteProfile = {
    genreScores: { 'Sci-Fi': 12 },
    favoriteGenres: ['Sci-Fi'],
    likedItems: [m1],
    recentHeroIds: [],
  };

  const rows = personalizeRows(
    [
      { key: 'popular-movies', title: 'Popular Movies', items: [m3, m2, m1] },
      { key: 'genre-romance', title: 'Romance', items: [m3] },
      { key: 'genre-sci-fi', title: 'Sci-Fi', items: [m1, m2] },
    ],
    profile
  );

  assert.equal(rows[0].key, 'for-you');
  assert.equal(rows[0].title, 'Picked for You');
  assert.equal(rows[1].key, 'my-likes');
  assert.equal(rows[1].title, 'Your Liked Titles');
  // Genre row for Sci-Fi should rank above Romance
  const sciFiIdx = rows.findIndex((r) => r.key === 'genre-sci-fi');
  const romanceIdx = rows.findIndex((r) => r.key === 'genre-romance');
  assert.ok(sciFiIdx < romanceIdx);
});
