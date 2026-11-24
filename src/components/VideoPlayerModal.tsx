'use client';

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  X, Play, Pause, Volume2, VolumeX, Maximize, Minimize, RotateCcw, RotateCw,
  Captions, AlertCircle, Check, SkipForward, SkipBack, Layers, PictureInPicture2, Settings,
} from 'lucide-react';
import Hls from 'hls.js';
import { formatLang, getPreferredSubtitleLang, matchesLanguage, setPreferredSubtitleLang } from '@/lib/subtitleLanguages';

export interface SubtitleQuery {
  imdbId?: string;
  mediaType: 'movie' | 'tv';
  season?: number;
  episode?: number;
}

/** One way to play the title; the player falls back through these in order. */
export interface PlayableSource {
  magnet?: string;
  url?: string;
  fileIdx?: number;
  label?: string;
}

interface VideoPlayerModalProps {
  title: string;
  sources: PlayableSource[];
  initialSourceIndex?: number;
  initialStartTime?: number;
  isLive?: boolean;
  subtitleQuery?: SubtitleQuery;
  onWatchProgress?: (position: number, duration: number) => void;
  nextEpisode?: {
    label: string;
    onPlayNext: () => void;
  };
  onClose: () => void;
}

interface SubtitleOption {
  id: string;
  lang: string;
  label: string;
  src: string;
}

interface ProbeStreamInfo {
  index: number;
  codec: string;
  language?: string;
  title?: string;
  channels?: number;
  isDefault: boolean;
}

interface PrepareStatus {
  state: 'connecting' | 'buffering' | 'ready' | 'error';
  error?: string;
  fileSize: number;
  headBytes: number;
  headTarget: number;
  aheadBytes: number;
  complete: boolean;
  peers: number;
  downloadSpeed: number;
  progress?: number;
  downloadedRanges?: [number, number][];
  files: { index: number; name: string; length: number }[];
}

type Phase = 'preparing' | 'probing' | 'ready';
type Mode = 'direct' | 'transcode';
type Menu = 'status' | 'source' | 'subtitles' | 'settings';

const VIDEO_FILE = /\.(mp4|mkv|webm|avi|mov|m4v|wmv|ts)$/i;
const PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2] as const;
// Seconds that must be available ahead before (re)starting after a stall; doubles per stall.
const START_BUFFER_SECONDS = 15;
const MAX_BUFFER_SECONDS = 60;
// Failover thresholds while a torrent is still preparing.
const NO_PEERS_MS = 25_000;
const SLOW_CHECK_MS = 30_000;
const MAX_PREPARE_MS = 90_000;
const MAX_ETA_SECONDS = 90;
// Mid-playback: switch source if a stall on a too-slow swarm lasts this long.
const MAX_STALL_MS = 45_000;

// Torrents are released shortly after the player leaves them; the delay absorbs React
// StrictMode's dev-only unmount/remount so we don't kill a swarm we're about to reuse.
const pendingRelease = new Map<string, ReturnType<typeof setTimeout>>();
function scheduleRelease(magnet: string) {
  clearTimeout(pendingRelease.get(magnet));
  pendingRelease.set(
    magnet,
    setTimeout(() => {
      pendingRelease.delete(magnet);
      fetch(`/api/stream/release?magnet=${encodeURIComponent(magnet)}`, { method: 'POST', keepalive: true }).catch(() => {});
    }, 1500)
  );
}
function cancelRelease(magnet: string) {
  clearTimeout(pendingRelease.get(magnet));
  pendingRelease.delete(magnet);
}

function formatTime(secs: number): string {
  if (!isFinite(secs) || secs < 0) return '0:00';
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = Math.floor(secs % 60);
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}` : `${m}:${String(s).padStart(2, '0')}`;
}

function formatRate(bytesPerSec: number): string {
  return `${(bytesPerSec / 1024 / 1024).toFixed(1)} MB/s`;
}

function bufferedAhead(video: HTMLVideoElement): number {
  const t = video.currentTime;
  for (let i = 0; i < video.buffered.length; i++) {
    if (video.buffered.start(i) <= t + 0.25 && video.buffered.end(i) >= t) return video.buffered.end(i) - t;
  }
  return 0;
}

export const VideoPlayerModal: React.FC<VideoPlayerModalProps> = ({
  title,
  sources,
  initialSourceIndex = 0,
  initialStartTime = 0,
  isLive = false,
  subtitleQuery,
  onWatchProgress,
  nextEpisode,
  onClose,
}) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // --- Source selection & pipeline state ---
  const [sourceIndex, setSourceIndex] = useState(initialSourceIndex);
  const [fileOverride, setFileOverride] = useState<number | undefined>(undefined);
  const source = sources[sourceIndex];
  const magnet = source?.magnet;
  const isTorrent = Boolean(magnet);
  const fileIndex = fileOverride ?? source?.fileIdx;
  const displayTitle = isLive && source?.label ? source.label : title;

  const [rawPhase, setPhase] = useState<Phase>(isTorrent ? 'preparing' : 'probing');
  const [mode, setMode] = useState<Mode>('direct');
  const [offset, setOffset] = useState(0); // transcode start position (seconds into the film)
  const [knownDuration, setKnownDuration] = useState(0);
  const [prepare, setPrepare] = useState<PrepareStatus | null>(null);
  const [reencoding, setReencoding] = useState(false);

  // --- Audio & Embedded Subtitle tracks from ffprobe ---
  const [audioStreams, setAudioStreams] = useState<ProbeStreamInfo[]>([]);
  const [selectedAudioIndex, setSelectedAudioIndex] = useState<number | undefined>(undefined);
  const [embeddedSubs, setEmbeddedSubs] = useState<SubtitleOption[]>([]);

  // --- Playback UI state ---
  const [isPlaying, setIsPlaying] = useState(false);
  const [buffering, setBuffering] = useState(true);
  const [bufferTarget, setBufferTarget] = useState(START_BUFFER_SECONDS);
  const [ahead, setAhead] = useState(0);
  const [currentTime, setCurrentTime] = useState(initialStartTime);
  const [mediaDuration, setMediaDuration] = useState(0);
  const [bufferedEnd, setBufferedEnd] = useState(0);
  const [bufferedRanges, setBufferedRanges] = useState<[number, number][]>([]);
  const [hoverSeek, setHoverSeek] = useState<{ ratio: number; time: number } | null>(null);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [playbackRate, setPlaybackRate] = useState<number>(1);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isPip, setIsPip] = useState(false);
  const pipSupported = typeof document !== 'undefined' && Boolean(document.pictureInPictureEnabled);
  const [showControls, setShowControls] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [externalSubs, setExternalSubs] = useState<SubtitleOption[]>([]);
  const [userSubChoice, setUserSubChoice] = useState<SubtitleOption | 'off' | undefined>(undefined);
  const [subtitleDelay, setSubtitleDelay] = useState(0); // seconds (+ delays subtitles, - advances them)
  // Only one player menu is open at a time.
  const [openMenu, setOpenMenu] = useState<Menu | null>(null);
  const toggleMenu = (menu: Menu) => setOpenMenu((current) => (current === menu ? null : menu));

  const wantPlay = useRef(true);
  const autoPausing = useRef(false);
  const justSeeked = useRef(false);
  const hideTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mouseInBar = useRef(false);
  const prepareRef = useRef<PrepareStatus | null>(null);
  const sourceStartedAt = useRef(0);
  const bufferingSince = useRef(0);
  const resumeAt = useRef(initialStartTime);
  const lastProgressReportAt = useRef(0);
  const lastTouchTap = useRef<{ time: number; x: number }>({ time: 0, x: 0 });

  // The film timeline: in transcode mode the <video> starts at 0 at `offset`.
  const base = mode === 'transcode' ? offset : 0;
  const duration = mode === 'transcode' ? knownDuration : mediaDuration || knownDuration;
  const timeline = useRef({ base: 0, duration: 0 });
  useEffect(() => {
    timeline.current = { base, duration };
  }, [base, duration]);
  const filmTime = () => timeline.current.base + (videoRef.current?.currentTime || 0);

  const sourceQuery = magnet
    ? `magnet=${encodeURIComponent(magnet)}${fileIndex !== undefined ? `&fileIndex=${fileIndex}` : ''}`
    : `url=${encodeURIComponent(source?.url || '')}`;
  const directUrl = magnet ? `/api/stream?${sourceQuery}` : source?.url || '';
  // Live/HLS streams can't be probed or converted: go straight to playback.
  const skipProbe = isLive || /\.m3u8(\?|$)/i.test(directUrl);
  const phase: Phase = rawPhase === 'probing' && skipProbe ? 'ready' : rawPhase;
  // Live channels whose servers block cross-origin reads are relayed through our server.
  const [liveRelay, setLiveRelay] = useState(false);
  const activeUrl =
    mode === 'transcode'
      ? `/api/playback/transcode?${sourceQuery}&start=${offset.toFixed(2)}${selectedAudioIndex !== undefined ? `&audioStream=${selectedAudioIndex}` : ''}`
      : isLive && liveRelay && /^https?:\/\//i.test(directUrl)
        ? `/api/live/proxy?url=${encodeURIComponent(directUrl)}`
        : directUrl;

  useEffect(() => {
    sourceStartedAt.current = Date.now();
  }, [sourceIndex, fileOverride]);

  // Lock page scroll behind the player.
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = prev;
    };
  }, []);

  // Stop downloading a torrent once the player leaves it (source switch or close).
  useEffect(() => {
    if (!magnet) return;
    cancelRelease(magnet);
    return () => scheduleRelease(magnet);
  }, [magnet]);

  // --- Source switching ---
  const resetPipeline = useCallback((nextIsTorrent: boolean) => {
    setPhase(nextIsTorrent ? 'preparing' : 'probing');
    setMode('direct');
    setOffset(0);
    setKnownDuration(0);
    setMediaDuration(0);
    setBufferedEnd(0);
    setBufferedRanges([]);
    setPrepare(null);
    prepareRef.current = null;
    setReencoding(false);
    setAudioStreams([]);
    setSelectedAudioIndex(undefined);
    setEmbeddedSubs([]);
    setLiveRelay(false);
    setBuffering(true);
    setBufferTarget(START_BUFFER_SECONDS);
    setError(null);
    bufferingSince.current = 0;
  }, []);

  const selectSource = useCallback(
    (nextIdx: number, keepPosition: boolean, noticeMsg?: string) => {
      if (nextIdx < 0 || nextIdx >= sources.length || nextIdx === sourceIndex) return;
      resumeAt.current = keepPosition && !isLive ? filmTime() : 0;
      if (noticeMsg) setNotice(noticeMsg);
      setFileOverride(undefined);
      setSourceIndex(nextIdx);
      setOpenMenu(null);
      resetPipeline(Boolean(sources[nextIdx].magnet));
    },
    [sourceIndex, sources, isLive, resetPipeline]
  );

  const tryNextSource = useCallback(
    (reason: string, keepPosition: boolean) => {
      const next = sourceIndex + 1;
      if (next >= sources.length) {
        setError(`${reason} No other sources are available for this title.`);
        return;
      }
      selectSource(next, keepPosition, `${reason} Switching to another source (${next + 1} of ${sources.length})…`);
    },
    [sourceIndex, sources.length, selectSource]
  );
  const tryNextRef = useRef(tryNextSource);
  useEffect(() => {
    tryNextRef.current = tryNextSource;
  }, [tryNextSource]);

  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(() => setNotice(null), 6000);
    return () => clearTimeout(t);
  }, [notice]);

  // --- 1. Torrent status: pre-buffer, stats, and failover while preparing ---
  useEffect(() => {
    if (!magnet) return;
    let cancelled = false;
    let failures = 0;
    const poll = async () => {
      try {
        const size = prepareRef.current?.fileSize || 0;
        const { duration: dur } = timeline.current;
        const from = size && dur > 0 ? Math.floor((filmTime() / dur) * size) : 0;
        const res = await fetch(`/api/stream/prepare?${sourceQuery}&from=${from}`);
        if (!res.ok) throw new Error(`prepare failed: ${res.status}`);
        const data: PrepareStatus = await res.json();
        if (cancelled) return;
        failures = 0;
        prepareRef.current = data;
        setPrepare(data);

        setPhase((p) => {
          if (p !== 'preparing') return p;
          if (data.state === 'ready') return 'probing';
          const elapsed = Date.now() - sourceStartedAt.current;
          const remaining = Math.max(0, data.headTarget - data.headBytes);
          const eta = data.downloadSpeed > 0 ? remaining / data.downloadSpeed : Infinity;
          if (data.state === 'error') tryNextRef.current('Couldn’t connect to this source.', false);
          else if (elapsed > NO_PEERS_MS && data.peers === 0) tryNextRef.current('No peers for this source.', false);
          else if (elapsed > SLOW_CHECK_MS && data.state === 'buffering' && eta > MAX_ETA_SECONDS) tryNextRef.current('This source is too slow.', false);
          else if (elapsed > MAX_PREPARE_MS) tryNextRef.current('This source is too slow.', false);
          return p;
        });
      } catch {
        if (!cancelled && ++failures >= 6) setError('The streaming server isn’t responding. Try again in a moment.');
      }
    };
    poll();
    const id = setInterval(poll, 1500);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [magnet, sourceQuery]);

  // --- 2. Probe: can the browser play this file directly, or does it need converting? ---
  useEffect(() => {
    if (phase !== 'probing') return;
    let cancelled = false;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 60_000);
    fetch(`/api/playback/probe?${sourceQuery}`, { signal: controller.signal })
      .then((r) => (r.ok ? r.json() : null))
      .then((info) => {
        if (cancelled) return;
        const start = resumeAt.current;
        if (info?.duration) setKnownDuration(info.duration);
        if (Array.isArray(info?.audioStreams)) setAudioStreams(info.audioStreams);
        if (typeof info?.selectedAudioIndex === 'number') setSelectedAudioIndex(info.selectedAudioIndex);
        if (Array.isArray(info?.subtitleStreams)) {
          const emb: SubtitleOption[] = info.subtitleStreams.map((s: ProbeStreamInfo) => ({
            id: `emb-${s.index}`,
            lang: s.language || 'und',
            label: `${formatLang(s.language || 'und')}${s.title ? ` (${s.title})` : ''} · Embedded`,
            src: `/api/subtitles/embedded?${sourceQuery}&stream=${s.index}`,
          }));
          setEmbeddedSubs(emb);
        }
        if (info && !info.directPlayable) {
          setMode('transcode');
          setOffset(start);
          setReencoding(Boolean(info.reencodesVideo));
          resumeAt.current = 0;
        } else {
          setMode('direct');
        }
        setPhase('ready');
      })
      .catch(() => {
        if (cancelled) return;
        // Couldn't inspect the file: let the browser try; a playback error falls back to converting.
        setMode('direct');
        setPhase('ready');
      })
      .finally(() => clearTimeout(timer));
    return () => {
      cancelled = true;
      controller.abort();
      clearTimeout(timer);
    };
  }, [phase, sourceQuery]);

  // --- 3. Attach the media source ---
  useEffect(() => {
    const video = videoRef.current;
    if (!video || phase !== 'ready' || !activeUrl) return;

    let hls: Hls | null = null;
    let stallTimer: ReturnType<typeof setTimeout> | undefined;
    if ((/\.m3u8(\?|$)/i.test(activeUrl) || isLive) && Hls.isSupported()) {
      hls = new Hls({ enableWorker: true, lowLatencyMode: true, backBufferLength: 90 });
      hls.loadSource(activeUrl);
      hls.attachMedia(video);
      let networkRetries = 0;
      let mediaRetries = 0;
      if (isLive && !liveRelay) {
        stallTimer = setTimeout(() => {
          if (video.readyState < 2) setLiveRelay(true);
        }, 10_000);
      }
      hls.on(Hls.Events.ERROR, (_e, data) => {
        if (!data.fatal) return;
        const manifestFailed = data.details === Hls.ErrorDetails.MANIFEST_LOAD_ERROR || data.details === Hls.ErrorDetails.MANIFEST_LOAD_TIMEOUT;
        if (isLive && !liveRelay && data.type === Hls.ErrorTypes.NETWORK_ERROR) setLiveRelay(true);
        else if (data.type === Hls.ErrorTypes.NETWORK_ERROR && !manifestFailed && networkRetries++ < 3) hls?.startLoad();
        else if (data.type === Hls.ErrorTypes.MEDIA_ERROR && mediaRetries++ < 2) hls?.recoverMediaError();
        else setError(isLive ? 'This channel is offline or not available in your region right now.' : 'Could not load this stream.');
      });
    } else {
      video.src = activeUrl;
      if (isLive && !liveRelay) {
        stallTimer = setTimeout(() => {
          if (video.readyState < 2) setLiveRelay(true);
        }, 10_000);
      }
    }

    return () => {
      clearTimeout(stallTimer);
      hls?.destroy();
      video.removeAttribute('src');
      video.load();
    };
  }, [activeUrl, isLive, phase, liveRelay]);

  // Keep video playbackRate synced across source/mode changes.
  useEffect(() => {
    if (videoRef.current && !isLive) {
      videoRef.current.playbackRate = playbackRate;
    }
  }, [playbackRate, activeUrl, isLive]);

  // --- 4. Buffer gate: hold playback until enough is available ahead, then (re)start ---
  useEffect(() => {
    if (!buffering || phase !== 'ready') return;
    if (!bufferingSince.current) bufferingSince.current = Date.now();
    const id = setInterval(() => {
      const video = videoRef.current;
      if (!video) return;
      const a = bufferedAhead(video);
      const { duration: dur } = timeline.current;
      const now = filmTime();
      const nearEnd = dur > 0 && now + a >= dur - 0.5;
      const browserIdle = video.networkState === video.NETWORK_IDLE && a >= 1;
      let enough: boolean;
      let shown = a;

      if (isLive) {
        enough = video.readyState >= 3;
      } else if (magnet) {
        const st = prepareRef.current;
        const bytesPerSec = st?.fileSize && dur > 0 ? st.fileSize / dur : 0;
        const serverAhead = st?.complete ? Infinity : bytesPerSec ? (st?.aheadBytes || 0) / bytesPerSec : 0;
        shown = Math.max(a, isFinite(serverAhead) ? serverAhead : bufferTarget);
        enough = nearEnd || serverAhead >= bufferTarget || a >= bufferTarget || (!bytesPerSec && browserIdle);

        const tooSlow = bytesPerSec > 0 && !st?.complete && (st?.downloadSpeed || 0) < bytesPerSec * 0.9;
        if (!enough && tooSlow && isPlaying && Date.now() - bufferingSince.current > MAX_STALL_MS && sources.length > sourceIndex + 1) {
          tryNextRef.current('This source can’t keep up.', true);
          return;
        }
      } else {
        enough = nearEnd || a >= bufferTarget || browserIdle;
      }

      setAhead(shown);
      if (enough && video.readyState >= 3) {
        setBuffering(false);
        bufferingSince.current = 0;
        if (wantPlay.current) video.play().catch(() => setIsPlaying(false));
      }
    }, 400);
    return () => clearInterval(id);
  }, [buffering, phase, bufferTarget, isLive, magnet, isPlaying, sourceIndex, sources.length]);

  const onWaiting = () => {
    const video = videoRef.current;
    if (!video || isLive) return;
    if (!justSeeked.current && isPlaying) setBufferTarget((t) => Math.min(MAX_BUFFER_SECONDS, t * 2));
    justSeeked.current = false;
    if (!video.paused) {
      autoPausing.current = true;
      video.pause();
    }
    setBuffering(true);
  };

  const onVideoError = () => {
    if (phase !== 'ready' || isLive) {
      if (isLive) {
        if (!liveRelay) setLiveRelay(true);
        else setError('Could not play this live stream.');
      }
      return;
    }
    if (mode === 'direct') {
      const at = filmTime();
      setMode('transcode');
      setOffset(at);
      setBuffering(true);
      return;
    }
    tryNextRef.current('This file couldn’t be played.', true);
  };

  // --- 5. Subtitles (Embedded + OpenSubtitles) & Subtitle Delay ---
  useEffect(() => {
    if (!subtitleQuery?.imdbId) return;
    let cancelled = false;
    const { imdbId, mediaType, season, episode } = subtitleQuery;
    const qs = new URLSearchParams({ imdbId, type: mediaType });
    if (mediaType === 'tv') {
      qs.set('season', String(season ?? 1));
      qs.set('episode', String(episode ?? 1));
    }
    fetch(`/api/subtitles?${qs.toString()}`)
      .then((r) => r.json())
      .then((data) => {
        if (cancelled) return;
        setExternalSubs(data.subtitles || []);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [subtitleQuery]);

  const subtitles = useMemo(() => {
    const pref = getPreferredSubtitleLang();
    const ext = subtitleQuery?.imdbId ? externalSubs : [];
    const rank = (s: SubtitleOption) => (matchesLanguage(s.lang, pref) ? 0 : matchesLanguage(s.lang, 'en') ? 1 : 2);
    return [...embeddedSubs, ...ext].sort((a, b) => rank(a) - rank(b));
  }, [embeddedSubs, externalSubs, subtitleQuery?.imdbId]);

  const activeSub = useMemo<SubtitleOption | null>(() => {
    if (userSubChoice === 'off') return null;
    if (userSubChoice && subtitles.some((s) => s.id === userSubChoice.id)) return userSubChoice;
    const pref = getPreferredSubtitleLang();
    if (pref === 'off') return null;
    return subtitles.find((s) => matchesLanguage(s.lang, pref)) || null;
  }, [userSubChoice, subtitles]);

  const effectiveSubOffset = base - subtitleDelay;
  const subtitleSrc = activeSub ? `${activeSub.src}&offset=${effectiveSubOffset.toFixed(2)}` : '';

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const apply = () => {
      for (let i = 0; i < video.textTracks.length; i++) {
        const track = video.textTracks[i];
        track.mode = activeSub && track.label === activeSub.label ? 'showing' : 'disabled';
      }
    };
    apply();
    video.textTracks.addEventListener('addtrack', apply);
    return () => video.textTracks.removeEventListener('addtrack', apply);
  }, [activeSub, subtitleSrc]);

  const chooseSubtitle = useCallback((sub: SubtitleOption | null) => {
    setUserSubChoice(sub ?? 'off');
    setPreferredSubtitleLang(sub ? sub.lang : 'off');
    setOpenMenu(null);
  }, []);

  const adjustSubtitleDelay = useCallback((delta: number) => {
    setSubtitleDelay((prev) => {
      const next = Math.round((prev + delta) * 10) / 10;
      setNotice(`Subtitle delay: ${next > 0 ? `+${next.toFixed(1)}s` : `${next.toFixed(1)}s`}`);
      return next;
    });
  }, []);

  const chooseAudioTrack = (streamIndex: number) => {
    setOpenMenu(null);
    if (streamIndex === selectedAudioIndex) return;
    const at = filmTime();
    setSelectedAudioIndex(streamIndex);
    setMode('transcode');
    setOffset(at);
    setBuffering(true);
  };

  // --- Controls ---
  const togglePlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    if (buffering) {
      wantPlay.current = !wantPlay.current;
      setIsPlaying(wantPlay.current);
    } else if (video.paused) {
      wantPlay.current = true;
      video.play().catch(() => {});
    } else {
      wantPlay.current = false;
      video.pause();
    }
  }, [buffering]);

  const seekTo = useCallback(
    (target: number) => {
      const video = videoRef.current;
      if (!video || isLive || phase !== 'ready') return;
      const t = Math.max(0, Math.min(duration > 0 ? duration - 1 : target, target));
      justSeeked.current = true;
      setCurrentTime(t);
      if (mode === 'transcode') {
        const local = t - offset;
        const end = video.buffered.length ? video.buffered.end(video.buffered.length - 1) : 0;
        if (local >= 0 && local <= end) {
          video.currentTime = local;
        } else {
          setOffset(t);
          setBuffering(true);
        }
      } else {
        video.currentTime = t;
      }
    },
    [duration, isLive, mode, offset, phase]
  );

  const toggleMute = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    video.muted = !video.muted;
    setMuted(video.muted);
  }, []);

  const toggleFullscreen = useCallback(() => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) containerRef.current.requestFullscreen().catch(() => {});
    else document.exitFullscreen().catch(() => {});
  }, []);

  const togglePip = useCallback(() => {
    const video = videoRef.current;
    if (!video || !document.pictureInPictureEnabled) return;
    if (document.pictureInPictureElement) {
      document.exitPictureInPicture().catch(() => {});
    } else {
      video.requestPictureInPicture().catch(() => {});
    }
  }, []);

  useEffect(() => {
    const onChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const onEnter = () => setIsPip(true);
    const onLeave = () => setIsPip(false);
    video.addEventListener('enterpictureinpicture', onEnter);
    video.addEventListener('leavepictureinpicture', onLeave);
    return () => {
      video.removeEventListener('enterpictureinpicture', onEnter);
      video.removeEventListener('leavepictureinpicture', onLeave);
    };
  }, []);

  // Wire navigator.mediaSession for hardware media keys and mobile lock-screen controls.
  useEffect(() => {
    if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
    const ms = navigator.mediaSession;
    try {
      ms.metadata = new MediaMetadata({
        title: displayTitle,
        artist: 'Screentime',
      });
      ms.setActionHandler('play', () => {
        wantPlay.current = true;
        videoRef.current?.play().catch(() => {});
      });
      ms.setActionHandler('pause', () => {
        wantPlay.current = false;
        videoRef.current?.pause();
      });
      ms.setActionHandler('seekbackward', isLive ? null : () => seekTo(filmTime() - 10));
      ms.setActionHandler('seekforward', isLive ? null : () => seekTo(filmTime() + 10));
      ms.setActionHandler('previoustrack', sources.length > 1 ? () => selectSource((sourceIndex - 1 + sources.length) % sources.length, !isLive) : null);
      ms.setActionHandler('nexttrack', sources.length > 1 ? () => selectSource((sourceIndex + 1) % sources.length, !isLive) : null);
    } catch {
      // ignore unsupported handlers
    }
    return () => {
      try {
        ms.setActionHandler('play', null);
        ms.setActionHandler('pause', null);
        ms.setActionHandler('seekbackward', null);
        ms.setActionHandler('seekforward', null);
        ms.setActionHandler('previoustrack', null);
        ms.setActionHandler('nexttrack', null);
      } catch {
        // ignore
      }
    };
  }, [displayTitle, isLive, seekTo, selectSource, sourceIndex, sources.length]);

  const revealControls = useCallback(() => {
    setShowControls(true);
    if (hideTimer.current) clearTimeout(hideTimer.current);
    const hide = () => {
      const bars = containerRef.current?.querySelectorAll('[data-player-bar]') ?? [];
      const inUse =
        mouseInBar.current || Array.from(bars).some((bar) => bar.querySelector(':focus-visible') !== null);
      if (inUse) hideTimer.current = setTimeout(hide, 1000);
      else setShowControls(false);
    };
    hideTimer.current = setTimeout(hide, 3000);
  }, []);

  useEffect(
    () => () => {
      if (hideTimer.current) clearTimeout(hideTimer.current);
    },
    []
  );

  const handleVideoPointerUp = (e: React.PointerEvent<HTMLVideoElement>) => {
    if (e.pointerType === 'touch') {
      const now = Date.now();
      const rect = e.currentTarget.getBoundingClientRect();
      const relX = (e.clientX - rect.left) / Math.max(1, rect.width);
      // Double-tap left/right third on touch to seek ±10s.
      if (!isLive && now - lastTouchTap.current.time < 300 && Math.abs(e.clientX - lastTouchTap.current.x) < 80) {
        if (relX < 0.35) {
          seekTo(filmTime() - 10);
          setNotice('−10s');
          lastTouchTap.current = { time: 0, x: 0 };
          return;
        }
        if (relX > 0.65) {
          seekTo(filmTime() + 10);
          setNotice('+10s');
          lastTouchTap.current = { time: 0, x: 0 };
          return;
        }
      }
      lastTouchTap.current = { time: now, x: e.clientX };
      if (!showControls) {
        revealControls();
      } else {
        setShowControls(false);
      }
      return;
    }
    togglePlay();
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLSelectElement) return;
      revealControls();
      switch (e.key.toLowerCase()) {
        case ' ':
        case 'k':
          e.preventDefault();
          togglePlay();
          break;
        case 'arrowleft':
          e.preventDefault();
          seekTo(filmTime() - 10);
          break;
        case 'arrowright':
          e.preventDefault();
          seekTo(filmTime() + 10);
          break;
        case 'arrowup':
          if (isLive && sources.length > 1) {
            e.preventDefault();
            selectSource((sourceIndex - 1 + sources.length) % sources.length, false);
          }
          break;
        case 'arrowdown':
          if (isLive && sources.length > 1) {
            e.preventDefault();
            selectSource((sourceIndex + 1) % sources.length, false);
          }
          break;
        case 'f':
          toggleFullscreen();
          break;
        case 'p':
          togglePip();
          break;
        case 'm':
          toggleMute();
          break;
        case 'c':
          if (subtitles.length) chooseSubtitle(activeSub ? null : subtitles[0]);
          break;
        case 'g':
          if (activeSub) adjustSubtitleDelay(-0.5);
          break;
        case 'h':
          if (activeSub) adjustSubtitleDelay(0.5);
          break;
        case 'escape':
          if (openMenu) {
            setOpenMenu(null);
          } else if (!document.fullscreenElement) {
            onClose();
          }
          break;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [
    togglePlay, toggleFullscreen, togglePip, toggleMute, revealControls, onClose,
    subtitles, activeSub, openMenu,
    seekTo, chooseSubtitle, adjustSubtitleDelay, isLive, sources.length, sourceIndex, selectSource,
  ]);

  // --- Overlay status ---
  const videoFiles = (prepare?.files || []).filter((f) => VIDEO_FILE.test(f.name));
  const preparingPct = prepare && prepare.headTarget > 0 ? Math.min(99, Math.round((prepare.headBytes / prepare.headTarget) * 100)) : 0;
  const bitrate = prepare?.fileSize && duration > 0 ? prepare.fileSize / duration : 0;
  const swarmTooSlow = isTorrent && !prepare?.complete && bitrate > 0 && (prepare?.downloadSpeed || 0) < bitrate * 0.9;
  const peersLine = isTorrent && prepare && prepare.peers > 0 ? `${prepare.peers} peers · ${formatRate(prepare.downloadSpeed)}` : null;

  let statusText: string | null = null;
  if (phase === 'preparing') {
    statusText = !prepare || prepare.state === 'connecting' ? 'Connecting to peers…' : `Downloading the start of the video… ${preparingPct}%`;
  } else if (phase === 'probing') {
    statusText = 'Preparing video…';
  } else if (buffering) {
    statusText = isLive ? 'Connecting…' : `Buffering… ${Math.min(Math.floor(ahead), bufferTarget)} / ${bufferTarget}s`;
  }
  const canSkipSource = sources.length > sourceIndex + 1;
  const controlsVisible = showControls || !isPlaying || buffering || openMenu !== null;
  const hasFiles = isTorrent && videoFiles.length > 1;
  const hasSettings = !isLive || audioStreams.length > 1 || hasFiles;
  const conversion = reencoding ? 'Live Transcode' : 'Live Remux';
  const statusLabel =
    isTorrent && prepare
      ? prepare.complete
        ? '100% Downloaded'
        : `${Math.min(99, Math.round((prepare.progress ?? 0) * 100))}% on disk`
      : mode === 'transcode'
        ? conversion
        : 'Direct Play';
  const chooseFile = (index: number) => {
    setOpenMenu(null);
    if (index === fileIndex) return;
    resumeAt.current = 0;
    setFileOverride(index);
    resetPipeline(true);
  };
  const showSubButton = Boolean(subtitleQuery?.imdbId) || embeddedSubs.length > 0;

  useEffect(() => {
    if (!openMenu) return;
    const onDown = (e: PointerEvent) => {
      if (!(e.target as Element).closest('[data-player-menu]')) setOpenMenu(null);
    };
    document.addEventListener('pointerdown', onDown);
    return () => document.removeEventListener('pointerdown', onDown);
  }, [openMenu]);

  return (
    <div
      ref={containerRef}
      onMouseMove={revealControls}
      className={`fixed inset-0 z-[60] bg-black flex items-center justify-center select-none ${controlsVisible ? '' : 'cursor-none'}`}
    >
      <video
        ref={videoRef}
        className="w-full h-full object-contain"
        preload="auto"
        playsInline
        onPointerUp={handleVideoPointerUp}
        onDoubleClick={toggleFullscreen}
        onLoadedMetadata={(e) => {
          const v = e.currentTarget;
          if (isFinite(v.duration)) setMediaDuration(v.duration);
          if (!isLive) v.playbackRate = playbackRate;
          if (mode === 'direct' && resumeAt.current > 0) {
            v.currentTime = resumeAt.current;
            resumeAt.current = 0;
          }
        }}
        onDurationChange={(e) => {
          const d = e.currentTarget.duration;
          if (isFinite(d)) setMediaDuration(d);
        }}
        onTimeUpdate={(e) => {
          const v = e.currentTarget;
          const now = base + v.currentTime;
          setCurrentTime(now);
          if (v.buffered.length) {
            setBufferedEnd(base + v.buffered.end(v.buffered.length - 1));
            const ranges: [number, number][] = [];
            for (let i = 0; i < v.buffered.length; i++) {
              ranges.push([base + v.buffered.start(i), base + v.buffered.end(i)]);
            }
            setBufferedRanges(ranges);
          }
          if (!isLive && onWatchProgress && now > 5 && Date.now() - lastProgressReportAt.current >= 4000) {
            lastProgressReportAt.current = Date.now();
            onWatchProgress(now, duration);
          }
        }}
        onProgress={(e) => {
          const v = e.currentTarget;
          if (v.buffered.length) {
            setBufferedEnd(base + v.buffered.end(v.buffered.length - 1));
            const ranges: [number, number][] = [];
            for (let i = 0; i < v.buffered.length; i++) {
              ranges.push([base + v.buffered.start(i), base + v.buffered.end(i)]);
            }
            setBufferedRanges(ranges);
          }
        }}
        onWaiting={onWaiting}
        onSeeking={() => {
          justSeeked.current = true;
        }}
        onPlaying={() => {
          setIsPlaying(true);
          setBuffering(false);
          setError(null);
          revealControls();
        }}
        onPause={() => {
          if (autoPausing.current) {
            autoPausing.current = false;
          } else {
            setIsPlaying(false);
            if (!isLive && onWatchProgress) {
              const now = filmTime();
              if (now > 5) onWatchProgress(now, duration);
            }
          }
        }}
        onEnded={() => {
          setIsPlaying(false);
          if (!isLive && onWatchProgress && duration > 0) {
            onWatchProgress(duration, duration);
          }
          if (nextEpisode) {
            nextEpisode.onPlayNext();
          }
        }}
        onError={onVideoError}
      >
        {activeSub && (
          <track
            key={`${activeSub.id}-${effectiveSubOffset}`}
            kind="subtitles"
            src={subtitleSrc}
            srcLang={activeSub.lang}
            label={activeSub.label}
            default
          />
        )}
      </video>

      {/* Status overlay */}
      {statusText && !error && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-black/50">
          <div className="w-12 h-12 rounded-full border-4 border-white/15 border-t-rose-500 animate-spin" />
          <p className="text-white text-sm font-semibold">{statusText}</p>
          {peersLine && <p className="text-xs text-zinc-400">{peersLine}</p>}
          {phase === 'ready' && buffering && mode === 'transcode' && reencoding && (
            <p className="text-xs text-zinc-500">Converting this video for your browser</p>
          )}
          {swarmTooSlow && buffering && phase === 'ready' && (
            <p className="max-w-sm text-center text-xs text-amber-300/90 px-4">
              This source downloads slower than it plays ({formatRate(prepare?.downloadSpeed || 0)} vs {formatRate(bitrate)} needed).
            </p>
          )}
          {canSkipSource && (
            <button
              type="button"
              onClick={() => tryNextSource('Skipped.', phase === 'ready')}
              className="mt-1 h-9 px-3.5 rounded-lg inline-flex items-center justify-center gap-1.5 bg-white/10 hover:bg-white/20 text-xs font-semibold leading-none text-white transition-colors"
            >
              <SkipForward className="w-3.5 h-3.5 shrink-0" />
              <span className="text-trim">Try another source</span>
            </button>
          )}
          {sources.length > 1 && <p className="text-[11px] text-zinc-500">Source {sourceIndex + 1} of {sources.length}</p>}
        </div>
      )}

      {notice && !error && (
        <div className="absolute top-20 left-1/2 -translate-x-1/2 max-w-md px-4 py-2.5 rounded-xl bg-zinc-900/95 border border-zinc-700 text-xs text-zinc-200 shadow-xl text-center">
          {notice}
        </div>
      )}

      {error && (
        <div className="absolute inset-0 flex items-center justify-center p-6 bg-black/70">
          <div className="max-w-md bg-zinc-900 border border-zinc-800 rounded-2xl p-6 text-center space-y-4">
            <AlertCircle className="w-8 h-8 text-rose-500 mx-auto" />
            <p className="text-sm text-zinc-200">{error}</p>
            <div className="flex items-center justify-center gap-2">
              {sources.length > 1 && (
                <button
                  type="button"
                  onClick={() => selectSource((sourceIndex + 1) % sources.length, !isLive)}
                  className="h-10 px-4 rounded-xl inline-flex items-center justify-center bg-white/10 hover:bg-white/15 text-white text-sm font-semibold leading-none transition-colors"
                >
                  <span className="text-trim">{isLive ? 'Next channel' : 'Try next source'}</span>
                </button>
              )}
              <button
                type="button"
                onClick={onClose}
                className="h-10 px-5 rounded-xl inline-flex items-center justify-center bg-rose-600 hover:bg-rose-500 text-white text-sm font-semibold leading-none transition-colors"
              >
                <span className="text-trim">Back</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Top bar: title on the left, stream status and source on the right */}
      <div
        data-player-bar
        onPointerEnter={(e) => {
          mouseInBar.current = e.pointerType === 'mouse';
        }}
        onPointerLeave={() => {
          mouseInBar.current = false;
        }}
        className={`absolute top-0 inset-x-0 z-20 px-4 pt-4 pb-10 sm:px-6 sm:pt-6 flex items-center gap-3 bg-gradient-to-b from-black/80 to-transparent transition-opacity duration-300 ${
          controlsVisible ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
      >
        <button type="button" onClick={onClose} title="Close (Esc)" aria-label="Close player" className="player-btn">
          <X className="w-5 h-5" />
        </button>
        <h1 className="min-w-0 flex-1 text-base sm:text-lg font-semibold text-white truncate">{displayTitle}</h1>

        <div data-player-menu className="relative flex items-center gap-1 sm:gap-2 shrink-0">
          {!isLive && (
            <button
              type="button"
              onClick={() => toggleMenu('status')}
              title="View timeline colors & stream details"
              aria-label={`Stream status: ${statusLabel}`}
              aria-expanded={openMenu === 'status'}
              className="player-btn player-btn-text"
            >
              {isTorrent && prepare?.complete ? (
                <Check className="player-success w-4 h-4 shrink-0" />
              ) : (
                <span className={`w-2 h-2 rounded-full shrink-0 ${isTorrent ? 'player-dot-info' : 'bg-white/80'}`} />
              )}
              <span className="hidden sm:inline tabular-nums">{statusLabel}</span>
            </button>
          )}
          {sources.length > 1 && (
            <button
              type="button"
              onClick={() => toggleMenu('source')}
              title={isLive ? 'Switch channel (↑/↓)' : `Switch source (${sourceIndex + 1}/${sources.length})`}
              aria-label={isLive ? 'Switch channel' : 'Switch source'}
              aria-expanded={openMenu === 'source'}
              className="player-btn player-btn-text"
            >
              <Layers className="w-4 h-4 shrink-0" />
              <span className="hidden sm:inline tabular-nums">
                {sourceIndex + 1}/{sources.length}
              </span>
            </button>
          )}

          {openMenu === 'status' && !isLive && (
            <div className="player-panel top-full right-0 mt-2 w-72 p-3.5 text-xs text-zinc-200 space-y-3">
                  <div className="flex items-center justify-between border-b border-white/10 pb-2">
                    <span className="font-semibold text-white">Timeline & Stream Status</span>
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full bg-white/10 text-zinc-300 text-[10px] leading-none text-trim">
                      {mode === 'transcode' ? (reencoding ? 'FFmpeg H.264' : 'FFmpeg Remux') : 'Direct Play'}
                    </span>
                  </div>

                  {/* Color key */}
                  <div className="space-y-1.5 text-[11px]">
                    <div className="flex items-center gap-2">
                      <span className="w-3 h-1.5 rounded-full bg-gradient-to-r from-rose-500 to-orange-400 shrink-0" />
                      <span className="text-zinc-300">
                        <strong className="text-white">Played:</strong> Current playback position
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="w-3 h-1.5 rounded-full bg-white/60 shrink-0" />
                      <span className="text-zinc-300">
                        <strong className="text-white">In Memory:</strong> Decoded in browser (instant seek)
                      </span>
                    </div>
                    {isTorrent && (
                      <div className="flex items-center gap-2">
                        <span
                          className={`w-3 h-1.5 rounded-full shrink-0 ${
                            prepare?.complete ? 'bg-emerald-400/70' : 'bg-sky-400/60'
                          }`}
                        />
                        <span className="text-zinc-300">
                          <strong className="text-white">On Disk:</strong>{' '}
                          {prepare?.complete
                            ? 'Entire file downloaded (100% cached)'
                            : 'Torrent pieces saved locally'}
                        </span>
                      </div>
                    )}
                    <div className="flex items-center gap-2">
                      <span className="w-3 h-1.5 rounded-full bg-white/15 shrink-0" />
                      <span className="text-zinc-400">
                        <strong className="text-zinc-300">Unloaded:</strong> Streams on demand when seeked
                      </span>
                    </div>
                  </div>

                  {/* Live stats */}
                  <div className="pt-2 border-t border-white/10 grid grid-cols-2 gap-2 text-[11px]">
                    <div>
                      <span className="text-zinc-500 block">Browser buffer</span>
                      <span className="font-mono text-white">
                        +{Math.max(0, Math.round(bufferedEnd - currentTime))}s ahead
                      </span>
                    </div>
                    {isTorrent && prepare && (
                      <>
                        <div>
                          <span className="text-zinc-500 block">Downloaded</span>
                          <span className="font-mono text-white">
                            {prepare.complete ? '100% (Complete)' : `${Math.round((prepare.progress ?? 0) * 100)}%`}
                          </span>
                        </div>
                        <div>
                          <span className="text-zinc-500 block">Swarm speed</span>
                          <span className="font-mono text-white">
                            {prepare.complete ? 'Done' : formatRate(prepare.downloadSpeed)}
                          </span>
                        </div>
                        <div>
                          <span className="text-zinc-500 block">Active peers</span>
                          <span className="font-mono text-white">{prepare.peers} connected</span>
                        </div>
                      </>
                    )}
                  </div>
            </div>
          )}

          {openMenu === 'source' && sources.length > 1 && (
            <div className="player-panel top-full right-0 mt-2 w-72">
              <div className="player-panel-title">{isLive ? 'Channels' : 'Sources'}</div>
              {sources.map((s, idx) => (
                <button
                  key={idx}
                  type="button"
                  onClick={() => selectSource(idx, !isLive)}
                  aria-pressed={idx === sourceIndex}
                  className="player-item"
                >
                  <span className="truncate">{s.label || `Source ${idx + 1}`}</span>
                  {idx === sourceIndex && <Check className="player-accent w-4 h-4 shrink-0" />}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Up Next overlay in final 35s of an episode */}
      {nextEpisode && !isLive && duration > 60 && duration - currentTime <= 35 && duration - currentTime > 0 && !error && openMenu === null && (
        <div className="absolute bottom-32 right-4 sm:right-6 z-20 flex items-center gap-3 px-4 py-3 rounded-2xl bg-zinc-900/95 ring-1 ring-white/15 shadow-2xl backdrop-blur-2xl">
          <div className="text-left">
            <p className="text-[11px] font-semibold uppercase tracking-wider text-rose-400">
              Up next in {Math.max(1, Math.ceil(duration - currentTime))}s
            </p>
            <p className="text-sm font-semibold text-white">{nextEpisode.label}</p>
          </div>
          <button
            type="button"
            onClick={nextEpisode.onPlayNext}
            className="h-9 px-4 rounded-full inline-flex items-center justify-center gap-1.5 bg-white hover:bg-zinc-200 text-black text-xs font-semibold leading-none shadow-lg transition-all shrink-0"
          >
            <Play className="w-3.5 h-3.5 fill-current shrink-0" />
            <span className="text-trim">Play now</span>
          </button>
        </div>
      )}

      {/* Bottom controls */}
      <div
        data-player-bar
        onPointerEnter={(e) => {
          mouseInBar.current = e.pointerType === 'mouse';
        }}
        onPointerLeave={() => {
          mouseInBar.current = false;
        }}
        className={`absolute bottom-0 inset-x-0 z-20 px-4 pb-3 pt-12 sm:px-6 sm:pb-5 space-y-1 bg-gradient-to-t from-black/90 to-transparent transition-opacity duration-300 ${
          controlsVisible ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
      >
        {!isLive && (
          <div className="flex items-center gap-3">
            <span className="player-time text-right">{formatTime(currentTime)}</span>
            <div
              className="group/scrubber relative h-6 flex-1 min-w-0 flex items-center"
              onMouseMove={(e) => {
                if (!duration) return;
                const rect = e.currentTarget.getBoundingClientRect();
                const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / Math.max(1, rect.width)));
                setHoverSeek({ ratio, time: ratio * duration });
              }}
              onMouseLeave={() => setHoverSeek(null)}
            >
              {/* Hover timestamp + seek readiness tooltip */}
              {hoverSeek && duration > 0 && (() => {
                const t = hoverSeek.time;
                const delta = Math.round(t - currentTime);
                const deltaLabel =
                  Math.abs(delta) < 2
                    ? 'now'
                    : `${delta > 0 ? '+' : '−'}${formatTime(Math.abs(delta))}`;
                const inBrowserBuffer =
                  bufferedRanges.some(([start, end]) => t >= start - 0.5 && t <= end + 0.5) ||
                  (t >= base && t <= bufferedEnd);
                const inDiskCache =
                  prepare?.complete ||
                  Boolean(prepare?.downloadedRanges?.some(([s, e]) => hoverSeek.ratio >= s && hoverSeek.ratio <= e));
                const clampedPct = Math.max(6, Math.min(94, hoverSeek.ratio * 100));
                return (
                  <div
                    style={{ left: `${clampedPct}%` }}
                    className="pointer-events-none absolute -top-12 -translate-x-1/2 z-30 flex flex-col items-center"
                  >
                    <div className="px-2.5 py-1 rounded-lg bg-zinc-950/95 border border-white/15 shadow-xl backdrop-blur-md whitespace-nowrap text-center">
                      <div className="flex items-center justify-center gap-1.5 text-xs font-mono font-semibold text-white">
                        <span>{formatTime(t)}</span>
                        <span className="text-[10px] font-normal text-zinc-400">({deltaLabel})</span>
                      </div>
                      <div className="flex items-center justify-center gap-1 text-[10px] font-medium">
                        {inBrowserBuffer ? (
                          <>
                            <span className="w-1.5 h-1.5 rounded-full bg-white" />
                            <span className="text-zinc-200">In memory · instant seek</span>
                          </>
                        ) : inDiskCache ? (
                          <>
                            <span className={`w-1.5 h-1.5 rounded-full ${prepare?.complete ? 'bg-emerald-400' : 'bg-sky-400'}`} />
                            <span className={prepare?.complete ? 'text-emerald-300' : 'text-sky-300'}>
                              {prepare?.complete ? '100% downloaded on disk' : 'Downloaded on disk'}
                            </span>
                          </>
                        ) : (
                          <>
                            <span className="w-1.5 h-1.5 rounded-full bg-zinc-500" />
                            <span className="text-zinc-400">
                              {mode === 'transcode' ? 'Converts from here on seek' : 'Streams from here on seek'}
                            </span>
                          </>
                        )}
                      </div>
                    </div>
                  </div>
                );
              })()}

              {/* Visual 4-layer timeline track */}
              <div
                className={`pointer-events-none absolute inset-x-0 h-1.5 group-hover/scrubber:h-2.5 rounded-full overflow-hidden transition-all duration-150 ${
                  prepare?.complete ? 'bg-emerald-950/60 ring-1 ring-emerald-400/35' : 'bg-white/15'
                }`}
              >
                {/* Layer 1: Torrent pieces downloaded on disk (sky while downloading, emerald when 100% complete) */}
                {isTorrent &&
                  (prepare?.complete ? (
                    <div className="absolute inset-y-0 left-0 w-full bg-emerald-400/35 transition-all duration-300" />
                  ) : (
                    (prepare?.downloadedRanges || []).map(([startFrac, endFrac], idx) => (
                      <div
                        key={`dl-${idx}`}
                        className="absolute inset-y-0 bg-sky-400/30 transition-all duration-300"
                        style={{
                          left: `${Math.max(0, Math.min(100, startFrac * 100))}%`,
                          width: `${Math.max(0.5, Math.min(100, (endFrac - startFrac) * 100))}%`,
                        }}
                      />
                    ))
                  ))}

                {/* Layer 2: Browser decoded <video>.buffered memory ranges */}
                {duration > 0 &&
                  (bufferedRanges.length > 0 ? (
                    bufferedRanges.map(([startSec, endSec], idx) => {
                      const leftPct = Math.max(0, Math.min(100, (startSec / duration) * 100));
                      const widthPct = Math.max(0, Math.min(100 - leftPct, ((endSec - startSec) / duration) * 100));
                      return (
                        <div
                          key={`buf-${idx}`}
                          className="absolute inset-y-0 bg-white/55 transition-all duration-150"
                          style={{ left: `${leftPct}%`, width: `${widthPct}%` }}
                        />
                      );
                    })
                  ) : bufferedEnd > base ? (
                    <div
                      className="absolute inset-y-0 bg-white/55 transition-all duration-150"
                      style={{
                        left: `${(base / duration) * 100}%`,
                        width: `${Math.max(0, Math.min(100, ((bufferedEnd - base) / duration) * 100))}%`,
                      }}
                    />
                  ) : null)}

                {/* Hover ghost highlight */}
                {hoverSeek && duration > 0 && (
                  <div
                    className="absolute inset-y-0 w-0.5 bg-white/80"
                    style={{ left: `${hoverSeek.ratio * 100}%` }}
                  />
                )}

                {/* Layer 3: Played progress fill */}
                {duration > 0 && (
                  <div
                    className="absolute inset-y-0 left-0 bg-gradient-to-r from-rose-500 to-orange-400 rounded-l-full"
                    style={{ width: `${Math.max(0, Math.min(100, (currentTime / duration) * 100))}%` }}
                  />
                )}
              </div>

              {/* Accessible native range input */}
              <input
                type="range"
                min={0}
                max={duration || 0}
                step={1}
                value={Math.min(currentTime, duration || 0)}
                aria-valuetext={`${formatTime(currentTime)} of ${formatTime(duration)}`}
                onChange={(e) => seekTo(parseFloat(e.target.value))}
                onPointerUp={(e) => e.currentTarget.blur()}
                aria-label="Seek"
                className="lumen-scrubber relative z-10"
              />
            </div>
            <span className="player-time">{formatTime(duration)}</span>
          </div>
        )}

        <div className="flex items-center justify-between">
          {/* Transport and volume */}
          <div className="flex items-center sm:gap-1">
            <button
              type="button"
              onClick={togglePlay}
              title={isPlaying ? 'Pause (Space / K)' : 'Play (Space / K)'}
              aria-label={isPlaying ? 'Pause' : 'Play'}
              className="player-btn player-btn-primary"
            >
              {isPlaying ? <Pause className="w-5 h-5 fill-current" /> : <Play className="w-5 h-5 fill-current" />}
            </button>

            {isLive && sources.length > 1 && (
              <>
                <button
                  type="button"
                  onClick={() => selectSource((sourceIndex - 1 + sources.length) % sources.length, false)}
                  title="Previous channel (↑)"
                  aria-label="Previous channel"
                  className="player-btn"
                >
                  <SkipBack className="w-5 h-5" />
                </button>
                <button
                  type="button"
                  onClick={() => selectSource((sourceIndex + 1) % sources.length, false)}
                  title="Next channel (↓)"
                  aria-label="Next channel"
                  className="player-btn"
                >
                  <SkipForward className="w-5 h-5" />
                </button>
              </>
            )}

            {!isLive && (
              <>
                <button
                  type="button"
                  onClick={() => seekTo(currentTime - 10)}
                  title="Back 10 seconds (←)"
                  aria-label="Back 10 seconds"
                  className="player-btn max-[359px]:hidden"
                >
                  <RotateCcw className="w-5 h-5" />
                </button>
                <button
                  type="button"
                  onClick={() => seekTo(currentTime + 10)}
                  title="Forward 10 seconds (→)"
                  aria-label="Forward 10 seconds"
                  className="player-btn max-[359px]:hidden"
                >
                  <RotateCw className="w-5 h-5" />
                </button>
                {nextEpisode && (
                  <button
                    type="button"
                    onClick={nextEpisode.onPlayNext}
                    title={`Next episode: ${nextEpisode.label}`}
                    aria-label={`Next episode: ${nextEpisode.label}`}
                    className="player-btn"
                  >
                    <SkipForward className="w-5 h-5" />
                  </button>
                )}
              </>
            )}

            <div className="player-volume">
              <button
                type="button"
                onClick={toggleMute}
                title={muted ? 'Unmute (M)' : 'Mute (M)'}
                aria-label={muted ? 'Unmute' : 'Mute'}
                className="player-btn"
              >
                {muted || volume === 0 ? <VolumeX className="w-5 h-5" /> : <Volume2 className="w-5 h-5" />}
              </button>
              <input
                type="range"
                min={0}
                max={1}
                step={0.05}
                value={muted ? 0 : volume}
                aria-label="Volume"
                onChange={(e) => {
                  const v = parseFloat(e.target.value);
                  setVolume(v);
                  if (videoRef.current) {
                    videoRef.current.volume = v;
                    videoRef.current.muted = v === 0;
                    setMuted(v === 0);
                  }
                }}
                onPointerUp={(e) => e.currentTarget.blur()}
              />
            </div>
            {isLive && <span className="player-live">Live</span>}
          </div>

          {/* Subtitles, settings and window controls */}
          <div data-player-menu className="relative flex items-center sm:gap-1">
            {showSubButton && (
              <button
                type="button"
                onClick={() => toggleMenu('subtitles')}
                title="Subtitles (C)"
                aria-label="Subtitles"
                aria-expanded={openMenu === 'subtitles'}
                className={`player-btn ${activeSub ? 'player-btn-on' : ''}`}
              >
                <Captions className="w-5 h-5" />
              </button>
            )}
            {hasSettings && (
              <button
                type="button"
                onClick={() => toggleMenu('settings')}
                title="Playback settings"
                aria-label="Playback settings"
                aria-expanded={openMenu === 'settings'}
                className={`player-btn ${playbackRate !== 1 ? 'player-btn-on' : ''}`}
              >
                <Settings className="w-5 h-5" />
              </button>
            )}
            {pipSupported && (
              <button
                type="button"
                onClick={togglePip}
                title="Picture-in-Picture (P)"
                aria-label="Picture-in-Picture"
                className={`player-btn max-sm:hidden ${isPip ? 'player-btn-on' : ''}`}
              >
                <PictureInPicture2 className="w-5 h-5" />
              </button>
            )}
            <button
              type="button"
              onClick={toggleFullscreen}
              title={isFullscreen ? 'Exit fullscreen (F)' : 'Fullscreen (F)'}
              aria-label={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
              className="player-btn"
            >
              {isFullscreen ? <Minimize className="w-5 h-5" /> : <Maximize className="w-5 h-5" />}
            </button>

            {openMenu === 'subtitles' && showSubButton && (
              <div className="player-panel bottom-full right-0 mb-2 w-72">
                <div className="player-panel-title">Subtitles</div>
                    {activeSub && (
                      <div className="px-3.5 py-2 border-b border-white/[0.08] flex items-center justify-between gap-2">
                        <span className="text-[11px] text-zinc-400">
                          Sync ({subtitleDelay > 0 ? `+${subtitleDelay.toFixed(1)}s` : `${subtitleDelay.toFixed(1)}s`})
                        </span>
                        <div className="flex items-center gap-1">
                          <button
                            type="button"
                            onClick={() => adjustSubtitleDelay(-0.5)}
                            title="Advance subtitles 0.5s (G)"
                            className="h-6 px-2 rounded-md inline-flex items-center justify-center leading-none bg-zinc-800 hover:bg-zinc-700 text-xs text-zinc-200 transition-colors"
                          >
                            <span className="text-trim">−0.5s</span>
                          </button>
                          {subtitleDelay !== 0 && (
                            <button
                              type="button"
                              onClick={() => setSubtitleDelay(0)}
                              className="h-6 px-2 rounded-md inline-flex items-center justify-center leading-none bg-zinc-800 hover:bg-zinc-700 text-[11px] text-zinc-400 hover:text-white transition-colors"
                            >
                              <span className="text-trim">0s</span>
                            </button>
                          )}
                          <button
                            type="button"
                            onClick={() => adjustSubtitleDelay(0.5)}
                            title="Delay subtitles 0.5s (H)"
                            className="h-6 px-2 rounded-md inline-flex items-center justify-center leading-none bg-zinc-800 hover:bg-zinc-700 text-xs text-zinc-200 transition-colors"
                          >
                            <span className="text-trim">+0.5s</span>
                          </button>
                        </div>
                      </div>
                    )}
                    <button
                      type="button"
                      onClick={() => chooseSubtitle(null)}
                      aria-pressed={!activeSub}
                          className="player-item"
                    >
                      <span>Off</span>
                      {!activeSub && <Check className="player-accent w-4 h-4 shrink-0" />}
                    </button>
                    {subtitles.length === 0 && <p className="px-3.5 py-2 text-xs text-zinc-500">No subtitles found</p>}
                    {subtitles.map((s) => {
                      const active = activeSub?.id === s.id;
                      return (
                        <button
                          key={s.id}
                          type="button"
                          onClick={() => chooseSubtitle(s)}
                          aria-pressed={active}
                          className="player-item"
                        >
                          <span className="truncate pr-2">{s.label}</span>
                          {active && <Check className="player-accent w-4 h-4 shrink-0" />}
                        </button>
                      );
                    })}
              </div>
            )}

            {openMenu === 'settings' && hasSettings && (
              <div className="player-panel bottom-full right-0 mb-2 w-72">
                {!isLive && (
                  <>
                    <div className="player-panel-title">Speed</div>
                    <div className="grid grid-cols-6 gap-1 px-2 pb-2">
                      {PLAYBACK_RATES.map((r) => (
                        <button
                          key={r}
                          type="button"
                          onClick={() => setPlaybackRate(r)}
                          aria-pressed={playbackRate === r}
                          aria-label={`${r}x speed`}
                          className="player-chip"
                        >
                          {r}x
                        </button>
                      ))}
                    </div>
                  </>
                )}
                {audioStreams.length > 1 && (
                  <>
                    <div className="player-panel-title">Audio</div>
                    {audioStreams.map((a) => (
                      <button
                        key={a.index}
                        type="button"
                        onClick={() => chooseAudioTrack(a.index)}
                        aria-pressed={selectedAudioIndex === a.index}
                        className="player-item"
                      >
                        <span className="truncate">
                          {`${formatLang(a.language || 'und')}${a.title ? ` (${a.title})` : ''} · ${a.codec.toUpperCase()}`}
                        </span>
                        {selectedAudioIndex === a.index && <Check className="player-accent w-4 h-4 shrink-0" />}
                      </button>
                    ))}
                  </>
                )}
                {hasFiles && (
                  <>
                    <div className="player-panel-title">File</div>
                    {fileIndex === undefined && (
                      <button type="button" onClick={() => setOpenMenu(null)} aria-pressed className="player-item">
                        <span className="truncate">Auto (largest file)</span>
                        <Check className="player-accent w-4 h-4 shrink-0" />
                      </button>
                    )}
                    {videoFiles.map((f) => (
                      <button
                        key={f.index}
                        type="button"
                        onClick={() => chooseFile(f.index)}
                        aria-pressed={fileIndex === f.index}
                        title={f.name}
                        className="player-item"
                      >
                        <span className="truncate">{f.name.split('/').pop() || f.name}</span>
                        {fileIndex === f.index && <Check className="player-accent w-4 h-4 shrink-0" />}
                      </button>
                    ))}
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
