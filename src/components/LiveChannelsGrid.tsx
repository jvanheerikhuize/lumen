'use client';

import React, { useEffect, useMemo, useState } from 'react';
import { Play, Search, MapPin, Clock, X, Star, Globe } from 'lucide-react';
import { LiveChannel } from '@/types';
import { PageHeader } from '@/components/PageHeader';
import { SelectDropdown, SelectOption } from '@/components/SelectDropdown';
import { getFavoriteChannelIds, toggleFavoriteChannel } from '@/lib/watchProgress';
import { filterKidsLiveChannels } from '@/lib/kidsFilter';

interface LiveChannelsGridProps {
  onPlayChannel: (channel: LiveChannel, channelList?: LiveChannel[]) => void;
  isKidsMode?: boolean;
}

const COUNTRY_KEY = 'lumen_live_country';
const KIDS_COUNTRY_KEY = 'lumen_live_country_kids';
const PAGE = 60;
const ALL = 'all';
const INTL = 'INTL';
const CATEGORIES = ['All', 'Favorites', 'Kids', 'News', 'Sports', 'Documentary', 'Music & Culture', 'Entertainment', 'General'] as const;
const KIDS_CATEGORIES = ['All', 'Favorites', 'Kids'] as const;
/** Filter key: the ISO country, or "International" for themed channels without one. */
const regionKey = (c: LiveChannel) => c.countryCode || INTL;

function readCountry(storageKey: string): string | null {
  try {
    return localStorage.getItem(storageKey) || (storageKey === COUNTRY_KEY ? localStorage.getItem('screentime_live_country') : null);
  } catch {
    return null;
  }
}

function browserCountry(): string | undefined {
  for (const lang of navigator.languages || [navigator.language]) {
    const region = lang.split('-')[1];
    if (region && /^[A-Za-z]{2}$/.test(region)) return region.toUpperCase();
  }
  return undefined;
}

export const LiveChannelsGrid: React.FC<LiveChannelsGridProps> = ({ onPlayChannel, isKidsMode = false }) => {
  const [channels, setChannels] = useState<LiveChannel[] | null>(null);
  const [country, setCountry] = useState<string>(ALL);
  const [category, setCategory] = useState<string>('All');
  const [query, setQuery] = useState('');
  const [limit, setLimit] = useState(PAGE);
  const [favoriteIds, setFavoriteIds] = useState<string[]>([]);
  // Which streams are reachable right now (checked on the server, in batches as they scroll into the list).
  const [status, setStatus] = useState<Record<string, 'ok' | 'offline'>>({});
  const [showOffline, setShowOffline] = useState(false);

  const activeCountryKey = isKidsMode ? KIDS_COUNTRY_KEY : COUNTRY_KEY;
  const categories = isKidsMode ? KIDS_CATEGORIES : CATEGORIES;

  useEffect(() => {
    let cancelled = false;
    fetch('/api/live')
      .then((r) => r.json())
      .then((data) => {
        if (cancelled) return;
        const rawList: LiveChannel[] = data.channels || [];
        const list = isKidsMode ? filterKidsLiveChannels(rawList) : rawList;
        setChannels(list);
        setFavoriteIds(getFavoriteChannelIds());
        // Remembered choice, else the browser's country if it has channels (disabled for Kids mode so all Kids stations appear), else everything.
        const saved = readCountry(activeCountryKey);
        const guess = isKidsMode ? undefined : browserCountry();
        if (saved && (saved === ALL || list.some((c) => regionKey(c) === saved))) setCountry(saved);
        else if (guess && list.some((c) => c.countryCode === guess)) setCountry(guess);
        else setCountry(ALL);
      })
      .catch(() => !cancelled && setChannels([]));
    return () => {
      cancelled = true;
    };
  }, [isKidsMode, activeCountryKey]);

  const countries = useMemo(() => {
    const counts = new Map<string, { name: string; count: number }>();
    for (const c of channels || []) {
      const key = regionKey(c);
      const entry = counts.get(key) || { name: key === INTL ? 'International' : c.country, count: 0 };
      entry.count++;
      counts.set(key, entry);
    }
    return [...counts.entries()].sort((a, b) => a[1].name.localeCompare(b[1].name));
  }, [channels]);

  const countryOptions = useMemo<SelectOption[]>(() => {
    const total = (channels || []).length;
    return [
      { value: ALL, label: 'All countries', badge: total || undefined },
      ...countries.map(([code, { name, count }]) => ({
        value: code,
        label: name,
        badge: count,
      })),
    ];
  }, [channels, countries]);

  const favSet = useMemo(() => new Set(favoriteIds), [favoriteIds]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (channels || [])
      .filter((c) =>
        category === 'Favorites'
          ? favSet.has(c.id)
          : country === ALL || regionKey(c) === country
      )
      .filter((c) => category === 'All' || category === 'Favorites' || c.category === category)
      .filter((c) => !q || c.name.toLowerCase().includes(q) || c.country.toLowerCase().includes(q))
      .sort((a, b) => {
        const favDiff = Number(favSet.has(b.id)) - Number(favSet.has(a.id));
        if (favDiff !== 0) return favDiff;
        return Number(Boolean(a.geoBlocked)) - Number(Boolean(b.geoBlocked));
      });
  }, [channels, country, category, query, favSet]);

  const offlineCount = filtered.filter((c) => status[c.streamUrl] === 'offline').length;
  const shown = useMemo(() => {
    const rank = (c: LiveChannel) => {
      const favBonus = favSet.has(c.id) ? -10 : 0;
      return favBonus + (status[c.streamUrl] === 'ok' ? 0 : status[c.streamUrl] === 'offline' ? 2 : 1);
    };
    return filtered.filter((c) => showOffline || status[c.streamUrl] !== 'offline').sort((a, b) => rank(a) - rank(b));
  }, [filtered, status, showOffline, favSet]);

  // Check the channels on screen that haven't been checked yet.
  const pending = useMemo(
    () => shown.slice(0, limit).map((c) => c.streamUrl).filter((u) => !(u in status)),
    [shown, limit, status]
  );
  const pendingKey = pending.slice(0, 24).join('|');
  useEffect(() => {
    if (!pendingKey) return;
    let cancelled = false;
    const urls = pendingKey.split('|');
    fetch('/api/live/status', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ urls }) })
      .then((r) => r.json())
      .then((data) => {
        if (cancelled) return;
        // Anything the server skipped counts as offline so we don't ask again.
        setStatus((prev) => ({ ...prev, ...Object.fromEntries(urls.map((u) => [u, data.status?.[u] || 'offline'])) }));
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [pendingKey]);

  const chooseCountry = (code: string) => {
    setCountry(code);
    setLimit(PAGE);
    try {
      localStorage.setItem(activeCountryKey, code);
    } catch {
      // ignore
    }
  };

  const countryLabel = country === ALL ? 'all countries' : countries.find(([code]) => code === country)?.[1].name || country;

  return (
    <div>
      <PageHeader
        eyebrow={isKidsMode ? 'Kids · Live' : 'Live'}
        title={isKidsMode ? 'Kids Live TV' : 'Live TV'}
        description={
          isKidsMode
            ? 'Kid-friendly free-to-air channels and children’s networks from the Free-TV directory.'
            : 'Free-to-air broadcasters and free ad-supported channels from around the world.'
        }
      />

      {/* Category filter chips */}
      <div className="flex flex-wrap items-center gap-2 mb-6">
        {categories.map((cat) => (
          <button
            key={cat}
            type="button"
            onClick={() => {
              setCategory(cat);
              setLimit(PAGE);
            }}
            className={`h-8 px-3.5 rounded-full inline-flex items-center justify-center text-xs font-semibold leading-none whitespace-nowrap transition-colors ${
              category === cat
                ? 'bg-white text-zinc-950 shadow-sm'
                : 'bg-zinc-900/80 text-zinc-300 ring-1 ring-inset ring-white/10 hover:bg-zinc-800 hover:text-white'
            }`}
          >
            <span className="text-trim">{cat}</span>
          </button>
        ))}
      </div>

      {/* All channels */}
      <section>
        <div className="flex flex-wrap items-end justify-between gap-3 mb-4">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="font-display text-lg sm:text-xl font-semibold text-white">Channels</h2>
              {country !== ALL && (
                <button
                  type="button"
                  onClick={() => chooseCountry(ALL)}
                  title="Show channels from all countries"
                  className="h-6 inline-flex items-center justify-center gap-1 px-2.5 rounded-full bg-rose-500/15 text-rose-300 ring-1 ring-inset ring-rose-500/30 text-xs font-semibold leading-none hover:bg-rose-500/25 transition-colors"
                >
                  <span className="text-trim">{countryLabel}</span>
                  <X className="w-3 h-3 shrink-0" />
                </button>
              )}
            </div>
            {channels !== null && (
              <p className="text-xs text-zinc-500">
                {filtered.length - offlineCount} channel{filtered.length - offlineCount === 1 ? '' : 's'} in {countryLabel}
                {offlineCount > 0 && (
                  <>
                    {' · '}
                    <button
                      type="button"
                      onClick={() => setShowOffline((v) => !v)}
                      className="underline decoration-zinc-700 underline-offset-2 hover:text-zinc-300"
                    >
                      {showOffline ? 'hide' : 'show'} {offlineCount} offline
                    </button>
                  </>
                )}
              </p>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative flex items-center">
              <Search className="absolute left-3.5 w-4 h-4 text-zinc-400 pointer-events-none" />
              <input
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setLimit(PAGE);
                }}
                placeholder="Find a channel"
                aria-label="Find a channel"
                className="h-9 w-44 sm:w-52 bg-zinc-900/85 hover:bg-zinc-800/95 focus:bg-zinc-900 ring-1 ring-inset ring-white/12 focus:ring-rose-500/60 rounded-full pl-9 pr-8 text-xs sm:text-sm text-white placeholder-zinc-400 outline-none shadow-sm transition-all"
              />
              {query && (
                <button
                  type="button"
                  onClick={() => setQuery('')}
                  aria-label="Clear channel search"
                  className="absolute right-2.5 w-5 h-5 rounded-full inline-flex items-center justify-center text-zinc-400 hover:text-white"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              )}
            </div>
            <SelectDropdown
              value={country}
              onChange={chooseCountry}
              options={countryOptions}
              ariaLabel="Country"
              icon={<Globe className="w-3.5 h-3.5" />}
              searchable
              searchPlaceholder="Filter countries…"
              align="right"
              menuWidthClass="w-72"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
          {channels === null
            ? Array.from({ length: 12 }).map((_, i) => <div key={i} className="aspect-[4/3] rounded-xl skeleton" />)
            : shown.slice(0, limit).map((channel) => (
                <LogoCard
                  key={channel.id}
                  channel={channel}
                  status={status[channel.streamUrl]}
                  isFavorite={favSet.has(channel.id)}
                  onToggleFavorite={(id) => setFavoriteIds(toggleFavoriteChannel(id))}
                  onPlay={(c) => onPlayChannel(c, shown)}
                />
              ))}
        </div>

        {channels !== null && shown.length === 0 && (
          <p className="py-16 text-center text-sm text-zinc-500">
            {isKidsMode
              ? 'No Kids channels match. Try All countries or clear your search.'
              : 'No channels match. Try another country, category, or search.'}
          </p>
        )}
        {shown.length > limit && (
          <div className="flex justify-center pt-6">
            <button
              type="button"
              onClick={() => setLimit((l) => l + PAGE)}
              className="h-10 px-6 rounded-full inline-flex items-center justify-center bg-zinc-900/85 hover:bg-zinc-800 ring-1 ring-inset ring-white/12 text-sm font-semibold leading-none text-zinc-200 hover:text-white transition-colors"
            >
              <span className="text-trim">Show more ({shown.length - limit} left)</span>
            </button>
          </div>
        )}
        <p className="mt-8 text-[11px] text-zinc-600">
          Channel list from the community-maintained Free-TV project. Some channels only work from their own country or aren’t on air 24/7.
        </p>
      </section>
    </div>
  );
};

const LogoCard: React.FC<{
  channel: LiveChannel;
  status?: 'ok' | 'offline';
  isFavorite: boolean;
  onToggleFavorite: (id: string) => void;
  onPlay: (c: LiveChannel) => void;
}> = ({ channel, status, isFavorite, onToggleFavorite, onPlay }) => {
  const [broken, setBroken] = useState(!channel.logo);
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onPlay(channel)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onPlay(channel);
        }
      }}
      className={`group cursor-pointer text-left rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rose-500 ${status === 'offline' ? 'opacity-40' : ''}`}
      title={channel.name}
    >
      <div className="relative aspect-[4/3] rounded-xl overflow-hidden bg-gradient-to-br from-zinc-800/80 to-zinc-900 ring-1 ring-inset ring-white/[0.07] group-hover:ring-white/25 transition-all duration-300 group-hover:-translate-y-0.5 flex items-center justify-center p-5">
        {broken ? (
          <span className="font-display text-lg font-semibold text-zinc-400 text-center line-clamp-2">{channel.name}</span>
        ) : (
          <img
            src={channel.logo}
            alt=""
            loading="lazy"
            onError={() => setBroken(true)}
            className="max-w-full max-h-full object-contain drop-shadow-[0_2px_8px_rgba(0,0,0,0.5)] transition-transform duration-300 group-hover:scale-105"
          />
        )}
        <span className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
          <span className="w-10 h-10 rounded-full bg-white text-black inline-flex items-center justify-center shadow-xl">
            <Play className="w-4 h-4 fill-current shrink-0" />
          </span>
        </span>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleFavorite(channel.id);
          }}
          aria-label={isFavorite ? `Unfavorite ${channel.name}` : `Favorite ${channel.name}`}
          aria-pressed={isFavorite}
          title={isFavorite ? 'Remove from favorites' : 'Pin to favorites'}
          className={`absolute top-2 right-2 w-7 h-7 rounded-full inline-flex items-center justify-center transition-all ${
            isFavorite
              ? 'bg-black/75 text-amber-400 opacity-100 ring-1 ring-inset ring-amber-400/30'
              : 'bg-black/65 text-zinc-300 hover:text-white opacity-0 group-hover:opacity-100 ring-1 ring-inset ring-white/10'
          }`}
        >
          <Star className={`w-3.5 h-3.5 shrink-0 ${isFavorite ? 'fill-amber-400' : ''}`} />
        </button>
      </div>
      <p className="mt-2 text-sm font-medium text-zinc-100 truncate">{channel.name}</p>
      <p className="text-xs text-zinc-500 truncate flex items-center gap-1.5">
        <span
          className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${
            status === 'ok' ? 'bg-emerald-400' : status === 'offline' ? 'bg-zinc-600' : 'bg-zinc-500 animate-pulse'
          }`}
          title={status === 'ok' ? 'On air' : status === 'offline' ? 'Offline' : 'Checking…'}
        />
        <span className="truncate">{channel.country}</span>
        {channel.geoBlocked && (
          <span className="inline-flex items-center gap-0.5 text-amber-400/80 shrink-0" title="Only available from this country">
            <MapPin className="w-3 h-3 shrink-0" />
            <span>local</span>
          </span>
        )}
        {channel.partTime && (
          <span className="inline-flex items-center gap-0.5 text-zinc-500 shrink-0" title="Not on air 24/7">
            <Clock className="w-3 h-3 shrink-0" />
          </span>
        )}
      </p>
    </div>
  );
};

