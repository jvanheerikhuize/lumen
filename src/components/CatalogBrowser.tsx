'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Calendar } from 'lucide-react';
import { MediaItem } from '@/types';
import { PosterCard, PosterSkeleton } from '@/components/PosterCard';
import { SelectDropdown, SelectOption } from '@/components/SelectDropdown';
import { KIDS_ANIME_GENRES, KIDS_CINEMETA_GENRES } from '@/lib/kidsFilter';

type BrowseType = 'movie' | 'tv' | 'anime';
type BrowseSource = 'top' | 'year' | 'trending' | 'popular' | 'season';

interface CatalogBrowserProps {
  mediaType: BrowseType;
  onSelectMedia: (item: MediaItem) => void;
  initialGenre?: string | null;
  onGenreChange?: (genre: string | null) => void;
  isKidsMode?: boolean;
}

const ANIME_GENRES = [
  'Action', 'Adventure', 'Comedy', 'Drama', 'Fantasy', 'Horror', 'Mecha', 'Music', 'Mystery',
  'Psychological', 'Romance', 'Sci-Fi', 'Slice of Life', 'Sports', 'Supernatural', 'Thriller',
];

const SOURCES: Record<BrowseType, { key: BrowseSource; label: string; genres: boolean }[]> = {
  movie: [
    { key: 'top', label: 'Popular', genres: true },
    { key: 'year', label: 'By year', genres: false },
  ],
  tv: [
    { key: 'top', label: 'Popular', genres: true },
    { key: 'year', label: 'By year', genres: false },
  ],
  anime: [
    { key: 'trending', label: 'Trending', genres: true },
    { key: 'season', label: 'This season', genres: false },
    { key: 'popular', label: 'All-time popular', genres: true },
    { key: 'top', label: 'Top rated', genres: true },
  ],
};

const GENRES = [
  'Action', 'Adventure', 'Animation', 'Biography', 'Comedy', 'Crime', 'Documentary',
  'Drama', 'Family', 'Fantasy', 'History', 'Horror', 'Mystery', 'Romance',
  'Sci-Fi', 'Sport', 'Thriller', 'War', 'Western',
];

const CURRENT_YEAR = new Date().getFullYear();
const YEARS = Array.from({ length: CURRENT_YEAR - 1979 }, (_, i) => String(CURRENT_YEAR - i));
const YEAR_OPTIONS: SelectOption[] = YEARS.map((y) => ({ value: y, label: y }));

const CINEMETA_PAGE = 50;

async function fetchPage(
  mediaType: BrowseType,
  source: BrowseSource,
  genre: string | null,
  pageIndex: number,
  isKidsMode = false
): Promise<{ items: MediaItem[]; hasMore: boolean }> {
  const params = new URLSearchParams();
  params.set('type', mediaType);
  params.set('catalog', source);
  params.set('skip', String(pageIndex * CINEMETA_PAGE));
  if (genre) params.set('genre', genre);
  if (isKidsMode) params.set('kids', '1');
  const res = await fetch(`/api/catalog?${params.toString()}`);
  if (!res.ok) throw new Error(`Catalog request failed (${res.status})`);
  const data = await res.json();
  return { items: data.items || [], hasMore: Boolean(data.hasMore) };
}

export const CatalogBrowser: React.FC<CatalogBrowserProps> = ({
  mediaType,
  onSelectMedia,
  initialGenre = null,
  onGenreChange,
  isKidsMode = false,
}) => {
  const [source, setSource] = useState<BrowseSource>(SOURCES[mediaType][0].key);
  const [genre, setGenre] = useState<string | null>(initialGenre);
  const [items, setItems] = useState<MediaItem[]>([]);
  const [page, setPage] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryCount, setRetryCount] = useState(0);
  // Bumped on every filter change so in-flight "load more" responses for the old filter are dropped.
  const filterVersion = useRef(0);

  const sources = SOURCES[mediaType];
  const genres = useMemo(() => {
    if (mediaType === 'anime') return isKidsMode ? [...KIDS_ANIME_GENRES] : ANIME_GENRES;
    return isKidsMode ? [...KIDS_CINEMETA_GENRES] : GENRES;
  }, [mediaType, isKidsMode]);
  const showGenres = sources.find((s) => s.key === source)?.genres ?? false;
  const effectiveGenre = genre && (source === 'year' || genres.includes(genre)) ? genre : null;

  // First page for the current filter.
  useEffect(() => {
    let cancelled = false;
    fetchPage(mediaType, source, effectiveGenre, 0, isKidsMode)
      .then((result) => {
        if (cancelled) return;
        setItems(result.items);
        setHasMore(result.hasMore);
        setPage(0);
        setError(null);
      })
      .catch((e) => {
        if (!cancelled) {
          console.error('Failed to load catalog page:', e);
          setError('Could not load catalog right now. Check your connection and try again.');
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [mediaType, source, effectiveGenre, retryCount, isKidsMode]);

  const changeFilter = (nextSource: BrowseSource, nextGenre: string | null) => {
    if (nextSource === source && nextGenre === effectiveGenre) return;
    filterVersion.current++;
    setSource(nextSource);
    setGenre(nextGenre);
    onGenreChange?.(nextGenre);
    setItems([]);
    setHasMore(false);
    setError(null);
    setLoading(true);
  };

  const retry = () => {
    setError(null);
    setLoading(true);
    setRetryCount((c) => c + 1);
  };

  const loadMore = () => {
    const version = filterVersion.current;
    const nextPage = page + 1;
    setLoading(true);
    setError(null);
    fetchPage(mediaType, source, effectiveGenre, nextPage, isKidsMode)
      .then((result) => {
        if (version !== filterVersion.current) return;
        setItems((prev) => {
          const seen = new Set(prev.map((i) => String(i.id)));
          return [...prev, ...result.items.filter((i) => !seen.has(String(i.id)))];
        });
        setHasMore(result.hasMore);
        setPage(nextPage);
      })
      .catch((e) => {
        if (version === filterVersion.current) {
          setError('Could not load more titles. Try again in a moment.');
        }
        console.error('Failed to load catalog page:', e);
      })
      .finally(() => {
        if (version === filterVersion.current) setLoading(false);
      });
  };

  const chip = (active: boolean) =>
    `h-8 px-3.5 rounded-full inline-flex items-center justify-center text-xs font-semibold leading-none whitespace-nowrap transition-colors ${
      active
        ? 'bg-white text-zinc-950 shadow-sm'
        : 'bg-zinc-900/80 text-zinc-300 ring-1 ring-inset ring-white/10 hover:bg-zinc-800 hover:text-white'
    }`;

  return (
    <div className="space-y-6">
      {/* Popular / By year, plus the year picker */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="inline-flex items-center p-1 rounded-full bg-zinc-900/85 ring-1 ring-inset ring-white/12">
          {sources.map((s) => (
            <button
              key={s.key}
              type="button"
              onClick={() => changeFilter(s.key, s.key === 'year' ? String(CURRENT_YEAR) : null)}
              className={`h-8 px-4 rounded-full inline-flex items-center justify-center text-sm font-semibold leading-none transition-all ${
                source === s.key
                  ? 'bg-gradient-to-r from-rose-500 to-orange-400 text-white shadow-sm shadow-rose-950/50'
                  : 'text-zinc-300 hover:text-white'
              }`}
            >
              <span className="text-trim">{s.label}</span>
            </button>
          ))}
        </div>
        {source === 'year' && (
          <SelectDropdown
            value={effectiveGenre ?? String(CURRENT_YEAR)}
            onChange={(val) => changeFilter('year', val)}
            options={YEAR_OPTIONS}
            ariaLabel="Year"
            icon={<Calendar className="w-3.5 h-3.5" />}
            align="left"
            menuWidthClass="w-36"
          />
        )}
      </div>

      {/* Genre chips wrap onto extra lines instead of scrolling */}
      {showGenres && (
        <div className="flex flex-wrap gap-2">
          {[null, ...genres].map((g) => (
            <button key={g ?? 'all'} type="button" onClick={() => changeFilter(source, g)} className={chip(effectiveGenre === g)}>
              <span className="text-trim">{g ?? 'All'}</span>
            </button>
          ))}
        </div>
      )}

      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-x-4 gap-y-7">
        {items.map((item) => (
          <PosterCard key={item.id} item={item} onSelect={onSelectMedia} />
        ))}
        {loading && Array.from({ length: items.length ? 6 : 18 }).map((_, i) => <PosterSkeleton key={`s${i}`} />)}
      </div>

      {!loading && error && (
        <div className="py-12 text-center space-y-3">
          <p className="text-sm text-rose-400">{error}</p>
          <button
            type="button"
            onClick={items.length > 0 ? loadMore : retry}
            className="h-9 px-5 rounded-full inline-flex items-center justify-center bg-zinc-900 hover:bg-zinc-800 text-xs font-semibold leading-none text-white ring-1 ring-inset ring-white/15 transition-colors"
          >
            <span className="text-trim">Try again</span>
          </button>
        </div>
      )}

      {!loading && !error && items.length === 0 && (
        <p className="text-sm text-zinc-500 py-16 text-center">No titles found for this filter.</p>
      )}

      {!loading && !error && hasMore && (
        <div className="flex justify-center pt-2">
          <button
            type="button"
            onClick={loadMore}
            className="h-10 px-6 rounded-full inline-flex items-center justify-center bg-zinc-900/85 hover:bg-zinc-800 ring-1 ring-inset ring-white/12 text-sm font-semibold leading-none text-zinc-200 hover:text-white transition-colors"
          >
            <span className="text-trim">Show more</span>
          </button>
        </div>
      )}
    </div>
  );
};

