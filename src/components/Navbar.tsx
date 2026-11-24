'use client';

import React, { useEffect, useRef, useState } from 'react';
import {
  Search,
  Settings2,
  Film,
  Tv,
  Loader2,
  X,
  Check,
  Plus,
  Users,
  ChevronDown,
  ShieldCheck,
  SlidersHorizontal,
} from 'lucide-react';
import { LiveChannel, MediaItem } from '@/types';
import { Logo } from '@/components/Logo';
import { HouseholdMember, getMemberDisplayName } from '@/lib/watchProgress';

export type AppTab = 'home' | 'movies' | 'tv' | 'anime' | 'live';

interface NavbarProps {
  activeTab: AppTab;
  setActiveTab: (tab: AppTab) => void;
  onSelectMedia: (media: MediaItem) => void;
  onSelectLiveChannel?: (channel: LiveChannel) => void;
  onOpenSettingsModal: () => void;
  profiles?: HouseholdMember[];
  activeProfileId?: string;
  onSwitchProfile?: (profileId: string) => void;
  onAddProfile?: (name: string, isKids?: boolean) => void;
  onOpenProfileManager?: () => void;
}

export const PROFILE_BADGE_CLASSES: Record<string, string> = {
  rose: 'bg-rose-500/25 text-rose-200 ring-rose-500/50',
  sky: 'bg-sky-500/25 text-sky-200 ring-sky-500/50',
  emerald: 'bg-emerald-500/25 text-emerald-200 ring-emerald-500/50',
  amber: 'bg-amber-500/25 text-amber-200 ring-amber-500/50',
  purple: 'bg-purple-500/25 text-purple-200 ring-purple-500/50',
};

const TABS: { key: AppTab; label: string }[] = [
  { key: 'home', label: 'Home' },
  { key: 'movies', label: 'Movies' },
  { key: 'tv', label: 'Series' },
  { key: 'anime', label: 'Anime' },
  { key: 'live', label: 'Live TV' },
];

export const Navbar: React.FC<NavbarProps> = ({
  activeTab,
  setActiveTab,
  onSelectMedia,
  onSelectLiveChannel,
  onOpenSettingsModal,
  profiles = [],
  activeProfileId = 'default',
  onSwitchProfile,
  onAddProfile,
  onOpenProfileManager,
}) => {
  const [scrolled, setScrolled] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<MediaItem[]>([]);
  const [channels, setChannels] = useState<LiveChannel[]>([]);
  const [searching, setSearching] = useState(false);
  const [open, setOpen] = useState(false);
  const [profileMenuOpen, setProfileMenuOpen] = useState(false);
  const [newProfileName, setNewProfileName] = useState('');
  const [newProfileIsKids, setNewProfileIsKids] = useState(false);
  const [failedPosters, setFailedPosters] = useState<Record<string, boolean>>({});
  const searchRef = useRef<HTMLDivElement>(null);
  const profileRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const requestId = useRef(0);

  const activeMember = profiles.find((p) => p.id === activeProfileId) || profiles[0];
  const isKidsMode = Boolean(activeMember?.isKids);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24);
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  // Press '/' anywhere (outside inputs) to focus search.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== '/' || e.metaKey || e.ctrlKey || e.altKey) return;
      // The innermost target: events from inputs inside web components are retargeted to their host.
      const target = e.composedPath()[0] as HTMLElement | undefined;
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement ||
        target?.isContentEditable
      ) {
        return;
      }
      e.preventDefault();
      inputRef.current?.focus();
      inputRef.current?.select();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  // Close the results and profile menu when clicking elsewhere or pressing Escape.
  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) setOpen(false);
      if (profileRef.current && !profileRef.current.contains(e.target as Node)) setProfileMenuOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false);
        setProfileMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, []);

  // Reset stale search results when toggling Kids mode.
  useEffect(() => {
    requestId.current++;
    setResults([]);
    setChannels([]);
  }, [isKidsMode]);

  // Debounced search; stale responses are ignored.
  useEffect(() => {
    const q = query.trim();
    if (!q) return;
    const id = ++requestId.current;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      setSearching(true);
      fetch(`/api/search?q=${encodeURIComponent(q)}${isKidsMode ? '&kids=1' : ''}`, {
        signal: controller.signal,
      })
        .then((r) => r.json())
        .then((data) => {
          if (id === requestId.current) {
            setResults(data.results || []);
            setChannels(data.channels || []);
          }
        })
        .catch(() => {
          if (id === requestId.current) {
            setResults([]);
            setChannels([]);
          }
        })
        .finally(() => {
          if (id === requestId.current) setSearching(false);
        });
    }, 250);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [query, isKidsMode]);

  const clearSearch = () => {
    requestId.current++;
    setQuery('');
    setResults([]);
    setChannels([]);
    setSearching(false);
    setOpen(false);
  };

  const solid = scrolled || activeTab !== 'home';
  const hasResults = results.length > 0 || channels.length > 0;

  return (
    <header
      className={`fixed top-0 inset-x-0 z-40 transition-all duration-300 ${
        solid
          ? 'bg-zinc-950/95 border-b border-white/[0.1] shadow-lg shadow-black/45'
          : 'bg-gradient-to-b from-black/95 via-black/80 to-transparent'
      }`}
    >
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2.5 md:py-0 md:h-16 flex flex-wrap md:flex-nowrap items-center gap-x-5 gap-y-2.5">
        <Logo onClick={() => setActiveTab('home')} />

        <nav
          aria-label="Primary"
          className="order-3 md:order-none w-full md:w-auto inline-flex items-center gap-1 p-1 rounded-full bg-zinc-950/85 backdrop-blur-md ring-1 ring-inset ring-white/20 shadow-[0_4px_20px_rgba(0,0,0,0.55)] min-w-0 overflow-x-auto no-scrollbar"
        >
          {TABS.map((t) => {
            const active = activeTab === t.key;
            return (
              <button
                key={t.key}
                type="button"
                onClick={() => setActiveTab(t.key)}
                aria-current={active ? 'page' : undefined}
                className={`h-8 px-3.5 rounded-full inline-flex items-center justify-center gap-1.5 text-[13px] sm:text-sm font-semibold leading-none whitespace-nowrap transition-all ${
                  active
                    ? 'bg-gradient-to-r from-rose-600 via-rose-500 to-amber-600 text-white shadow-sm shadow-black/40'
                    : 'text-zinc-200 hover:text-white hover:bg-white/[0.1]'
                }`}
              >
                <span className="text-trim">{t.label}</span>
                {t.key === 'live' && (
                  <span
                    className={`w-1.5 h-1.5 rounded-full shrink-0 ${
                      active ? 'bg-white' : 'bg-rose-400 animate-pulse'
                    }`}
                  />
                )}
              </button>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <div ref={searchRef} className="relative">
            <div className="relative flex items-center">
              <Search className="absolute left-3.5 w-4 h-4 text-zinc-300 pointer-events-none" />
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={(e) => {
                  const val = e.target.value;
                  setQuery(val);
                  setOpen(Boolean(val.trim()));
                  if (!val.trim()) {
                    requestId.current++;
                    setResults([]);
                    setChannels([]);
                    setSearching(false);
                  }
                }}
                onFocus={() => query.trim() && setOpen(true)}
                onKeyDown={(e) => e.key === 'Escape' && clearSearch()}
                placeholder={isKidsMode ? 'Search Kids' : 'Search'}
                aria-label="Search titles and live channels (Press / to focus)"
                className="h-9 w-28 sm:w-48 focus:w-40 sm:focus:w-72 transition-[width,background-color] duration-300 bg-zinc-950/85 hover:bg-zinc-900/95 focus:bg-zinc-900 backdrop-blur-md ring-1 ring-inset ring-white/20 focus:ring-rose-500/60 rounded-full pl-9 pr-8 text-sm text-white placeholder-zinc-300 outline-none shadow-[0_4px_20px_rgba(0,0,0,0.45)]"
              />
              {query ? (
                <button
                  type="button"
                  onClick={clearSearch}
                  aria-label="Clear search"
                  className="absolute right-2 w-6 h-6 rounded-full inline-flex items-center justify-center text-zinc-400 hover:text-white hover:bg-white/10 transition-colors"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              ) : (
                <kbd className="hidden sm:inline-flex items-center justify-center absolute right-2.5 h-5 px-1.5 text-[10px] font-mono leading-none text-zinc-300 bg-white/[0.1] ring-1 ring-inset ring-white/15 rounded pointer-events-none">
                  /
                </kbd>
              )}
            </div>

            {open && (
              <div className="max-sm:fixed max-sm:left-4 max-sm:right-4 max-sm:top-[4.25rem] sm:absolute sm:right-0 sm:top-full sm:mt-2 sm:w-96 rounded-2xl bg-zinc-950/[0.98] backdrop-blur-2xl ring-1 ring-white/15 shadow-2xl shadow-black/80 overflow-hidden z-50">
                <div className="max-h-[65vh] overflow-y-auto py-1.5">
                  {searching && !hasResults ? (
                    <div className="p-5 flex items-center justify-center gap-2 text-xs text-zinc-400">
                      <Loader2 className="w-4 h-4 animate-spin text-rose-400" />
                      <span className="text-trim">Searching…</span>
                    </div>
                  ) : hasResults ? (
                    <>
                      {results.map((item) => {
                        const badge = item.anime ? 'Anime' : item.mediaType === 'tv' ? 'Series' : 'Movie';
                        const badgeColor = item.anime
                          ? 'bg-purple-500/15 text-purple-300'
                          : item.mediaType === 'tv'
                            ? 'bg-sky-500/15 text-sky-300'
                            : 'bg-amber-500/15 text-amber-300';
                        return (
                          <button
                            key={`${item.mediaType}-${item.id}`}
                            type="button"
                            onClick={() => {
                              onSelectMedia(item);
                              clearSearch();
                            }}
                            className="w-full px-3 py-2 flex items-center gap-3 text-left hover:bg-white/[0.06] transition-colors"
                          >
                            {item.posterPath && !failedPosters[item.posterPath] ? (
                              <img
                                src={item.posterPath}
                                alt=""
                                onError={() => setFailedPosters((prev) => ({ ...prev, [item.posterPath!]: true }))}
                                className="w-9 h-[54px] object-cover rounded-md bg-zinc-800 flex-shrink-0"
                              />
                            ) : (
                              <div className="w-9 h-[54px] rounded-md bg-zinc-800 flex items-center justify-center flex-shrink-0">
                                <Film className="w-4 h-4 text-zinc-600" />
                              </div>
                            )}
                            <div className="min-w-0 flex-1">
                              <p className="text-sm font-medium text-white truncate">{item.title}</p>
                              <div className="flex items-center gap-1.5 mt-0.5 text-xs text-zinc-500">
                                <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold leading-none text-trim ${badgeColor}`}>
                                  {badge}
                                </span>
                                {(item.releaseDate || item.firstAirDate) && (
                                  <span>{(item.releaseDate || item.firstAirDate)!.split('-')[0]}</span>
                                )}
                              </div>
                            </div>
                            {item.voteAverage > 0 && <span className="text-xs font-medium text-zinc-400">★ {item.voteAverage.toFixed(1)}</span>}
                          </button>
                        );
                      })}

                      {channels.length > 0 && (
                        <>
                          {results.length > 0 && <div className="my-1 border-t border-white/[0.06]" />}
                          {channels.map((ch) => (
                            <button
                              key={`live-${ch.id}`}
                              type="button"
                              onClick={() => {
                                if (onSelectLiveChannel) onSelectLiveChannel(ch);
                                else setActiveTab('live');
                                clearSearch();
                              }}
                              className="w-full px-3 py-2 flex items-center gap-3 text-left hover:bg-white/[0.06] transition-colors"
                            >
                              <div className="w-9 h-9 rounded-md bg-zinc-800 flex items-center justify-center flex-shrink-0 overflow-hidden p-1">
                                {ch.logo && !failedPosters[ch.logo] ? (
                                  <img
                                    src={ch.logo}
                                    alt=""
                                    onError={() => setFailedPosters((prev) => ({ ...prev, [ch.logo]: true }))}
                                    className="max-w-full max-h-full object-contain"
                                  />
                                ) : (
                                  <Tv className="w-4 h-4 text-rose-400" />
                                )}
                              </div>
                              <div className="min-w-0 flex-1">
                                <p className="text-sm font-medium text-white truncate">{ch.name}</p>
                                <div className="flex items-center gap-1.5 mt-0.5 text-xs text-zinc-500">
                                  <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold leading-none text-trim bg-rose-500/15 text-rose-300">
                                    Live TV
                                  </span>
                                  <span className="truncate">{ch.country}</span>
                                </div>
                              </div>
                            </button>
                          ))}
                        </>
                      )}
                    </>
                  ) : (
                    <p className="p-5 text-center text-xs text-zinc-500">No results for “{query}”</p>
                  )}
                </div>
              </div>
            )}
          </div>

          {profiles.length > 0 && onSwitchProfile && activeMember && (
            <div ref={profileRef} className="relative">
              {(() => {
                const badgeClass = PROFILE_BADGE_CLASSES[activeMember.color] || PROFILE_BADGE_CLASSES.rose;
                const displayName = getMemberDisplayName(activeMember);
                return (
                  <button
                    type="button"
                    onClick={() => setProfileMenuOpen((v) => !v)}
                    title={`Household member: ${displayName}${activeMember.isKids ? ' (Kids account)' : ''}`}
                    aria-label={`Switch household member (${displayName})`}
                    aria-expanded={profileMenuOpen}
                    className={`h-9 inline-flex items-center justify-center gap-2 pl-1.5 pr-2.5 sm:pr-3 rounded-full bg-zinc-950/85 hover:bg-zinc-900/95 backdrop-blur-md ring-1 ring-inset text-xs font-semibold text-zinc-100 hover:text-white shadow-[0_4px_20px_rgba(0,0,0,0.45)] transition-all ${
                      profileMenuOpen ? 'ring-rose-500/60 bg-zinc-900' : 'ring-white/20 hover:ring-white/30'
                    }`}
                  >
                    <span className={`w-6 h-6 rounded-full ring-1 inline-flex items-center justify-center text-[11px] font-bold uppercase leading-none shrink-0 ${badgeClass}`}>
                      <span className="text-trim">{displayName.slice(0, 1)}</span>
                    </span>
                    <span className="hidden sm:inline max-w-[6.5rem] truncate leading-snug">{displayName}</span>
                    {activeMember.isKids && (
                      <span className="hidden md:inline-flex items-center justify-center px-1.5 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 ring-1 ring-emerald-500/40 text-[10px] font-bold leading-none text-trim shrink-0">
                        Kids
                      </span>
                    )}
                    <ChevronDown
                      className={`w-3.5 h-3.5 text-zinc-300 shrink-0 transition-transform duration-200 ${
                        profileMenuOpen ? 'rotate-180 text-rose-400' : ''
                      }`}
                    />
                  </button>
                );
              })()}

              {profileMenuOpen && (
                <div className="absolute right-0 top-full mt-2 w-72 max-w-[calc(100vw-2rem)] rounded-2xl bg-zinc-950/[0.98] backdrop-blur-2xl ring-1 ring-white/15 shadow-2xl shadow-black/80 p-2 text-xs z-50">
                  <div className="px-2 py-2 flex items-center justify-between gap-2 border-b border-white/[0.08] mb-1.5">
                    <span className="inline-flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider text-zinc-400">
                      <Users className="w-3.5 h-3.5 text-rose-400 shrink-0" />
                      <span className="text-trim">Who’s watching?</span>
                    </span>
                    {onOpenProfileManager && (
                      <button
                        type="button"
                        onClick={() => {
                          setProfileMenuOpen(false);
                          onOpenProfileManager();
                        }}
                        className="h-6 px-2 rounded-lg inline-flex items-center justify-center gap-1 bg-white/[0.07] hover:bg-white/[0.14] text-[11px] font-semibold leading-none text-zinc-200 hover:text-white ring-1 ring-inset ring-white/10 transition-colors"
                      >
                        <SlidersHorizontal className="w-3 h-3 shrink-0" />
                        <span className="text-trim">Manage</span>
                      </button>
                    )}
                  </div>

                  <div className="space-y-0.5 max-h-60 overflow-y-auto">
                    {profiles.map((member) => {
                      const active = member.id === activeProfileId;
                      const badgeClass = PROFILE_BADGE_CLASSES[member.color] || PROFILE_BADGE_CLASSES.rose;
                      const displayName = getMemberDisplayName(member);
                      const hasNickname = Boolean(member.nickname?.trim() && member.nickname.trim() !== member.name.trim());
                      return (
                        <button
                          key={member.id}
                          type="button"
                          onClick={() => {
                            onSwitchProfile(member.id);
                            setProfileMenuOpen(false);
                          }}
                          className={`w-full flex items-center justify-between gap-2.5 px-2.5 py-2 rounded-xl text-left transition-colors ${
                            active
                              ? 'bg-rose-500/15 text-white ring-1 ring-inset ring-rose-500/30'
                              : 'text-zinc-300 hover:bg-white/[0.07] hover:text-white'
                          }`}
                        >
                          <span className="flex items-center gap-2.5 min-w-0">
                            <span className={`w-6 h-6 rounded-full ring-1 inline-flex items-center justify-center text-[11px] font-bold uppercase leading-none shrink-0 ${badgeClass}`}>
                              <span className="text-trim">{displayName.slice(0, 1)}</span>
                            </span>
                            <span className="min-w-0 flex flex-col">
                              <span className="truncate font-semibold leading-tight text-white">{displayName}</span>
                              {hasNickname && (
                                <span className="truncate text-[10px] text-zinc-400 leading-tight">{member.name}</span>
                              )}
                            </span>
                          </span>
                          <span className="inline-flex items-center gap-1.5 shrink-0">
                            {member.isKids && (
                              <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-emerald-500/15 text-emerald-300 ring-1 ring-emerald-500/30 text-[10px] font-semibold leading-none text-trim">
                                Kids
                              </span>
                            )}
                            {active && <Check className="w-3.5 h-3.5 text-rose-400 shrink-0" />}
                          </span>
                        </button>
                      );
                    })}
                  </div>

                  {onAddProfile && (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        const trimmed = newProfileName.trim();
                        if (!trimmed) return;
                        onAddProfile(trimmed, newProfileIsKids);
                        setNewProfileName('');
                        setNewProfileIsKids(false);
                        setProfileMenuOpen(false);
                      }}
                      className="mt-2 pt-2 border-t border-white/[0.08] flex items-center gap-1.5"
                    >
                      <input
                        type="text"
                        value={newProfileName}
                        onChange={(e) => setNewProfileName(e.target.value)}
                        placeholder="Add member…"
                        aria-label="New household member name"
                        maxLength={24}
                        className="h-8 flex-1 min-w-0 bg-white/[0.06] ring-1 ring-inset ring-white/12 focus:ring-rose-500/50 rounded-xl px-2.5 text-xs text-white placeholder-zinc-500 outline-none"
                      />
                      <button
                        type="button"
                        onClick={() => setNewProfileIsKids((v) => !v)}
                        aria-pressed={newProfileIsKids}
                        title={newProfileIsKids ? 'Creating Kids profile' : 'Switch to Kids profile'}
                        className={`h-8 px-2 rounded-xl inline-flex items-center justify-center gap-1 text-[11px] font-semibold leading-none ring-1 ring-inset transition-colors shrink-0 ${
                          newProfileIsKids
                            ? 'bg-emerald-500/20 text-emerald-300 ring-emerald-500/40'
                            : 'bg-white/[0.05] text-zinc-400 hover:text-zinc-200 ring-white/10'
                        }`}
                      >
                        <ShieldCheck className="w-3 h-3 shrink-0" />
                        <span className="text-trim">Kids</span>
                      </button>
                      <button
                        type="submit"
                        title="Add household member"
                        aria-label="Add household member"
                        className="h-8 w-8 rounded-xl inline-flex items-center justify-center bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 ring-1 ring-inset ring-rose-500/35 transition-colors shrink-0"
                      >
                        <Plus className="w-3.5 h-3.5" />
                      </button>
                    </form>
                  )}
                </div>
              )}
            </div>
          )}

          <button
            type="button"
            onClick={onOpenSettingsModal}
            title="Settings"
            aria-label="Settings"
            className="w-9 h-9 rounded-full inline-flex items-center justify-center bg-zinc-950/85 hover:bg-zinc-900/95 backdrop-blur-md ring-1 ring-inset ring-white/20 hover:ring-white/30 text-zinc-200 hover:text-white shadow-[0_4px_20px_rgba(0,0,0,0.45)] transition-all shrink-0"
          >
            <Settings2 className="w-4 h-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
