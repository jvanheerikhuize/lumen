'use client';

import React, { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Check, ChevronDown, Search, X } from 'lucide-react';

export interface SelectOption {
  value: string;
  label: string;
  badge?: string | number;
}

interface SelectDropdownProps {
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  ariaLabel: string;
  icon?: React.ReactNode;
  searchable?: boolean;
  searchPlaceholder?: string;
  align?: 'left' | 'right';
  placement?: 'auto' | 'top' | 'bottom';
  variant?: 'pill' | 'field';
  compact?: boolean;
  className?: string;
  menuWidthClass?: string;
  onOpenChange?: (open: boolean) => void;
}

export const SelectDropdown: React.FC<SelectDropdownProps> = ({
  value,
  onChange,
  options,
  ariaLabel,
  icon,
  searchable = false,
  searchPlaceholder = 'Filter…',
  align = 'right',
  placement = 'auto',
  variant = 'pill',
  compact = false,
  className = '',
  menuWidthClass = 'w-64',
  onOpenChange,
}) => {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState('');
  const [resolvedPlacement, setResolvedPlacement] = useState<'top' | 'bottom'>(placement === 'top' ? 'top' : 'bottom');
  const [resolvedAlign, setResolvedAlign] = useState<'left' | 'right'>(align);
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const optionRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const listboxId = useId();

  const selected = useMemo(
    () => options.find((o) => o.value === value) || options[0],
    [options, value]
  );

  const filteredOptions = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return options;
    return options.filter((o) => o.label.toLowerCase().includes(q));
  }, [options, filter]);

  const onOpenChangeRef = useRef(onOpenChange);
  useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  const setOpenState = (next: boolean) => {
    setOpen(next);
    onOpenChangeRef.current?.(next);
  };

  const closeMenu = (restoreFocus = false) => {
    setOpenState(false);
    setFilter('');
    if (restoreFocus) {
      requestAnimationFrame(() => triggerRef.current?.focus());
    }
  };

  useLayoutEffect(() => {
    if (!open) return;
    const measure = () => {
      const trigger = triggerRef.current;
      if (!trigger) return;
      const rect = trigger.getBoundingClientRect();
      const dialog = trigger.closest('dialog');
      const bounds = dialog ? dialog.getBoundingClientRect() : null;
      const topLimit = bounds ? Math.max(0, bounds.top) : 0;
      const bottomLimit = bounds ? Math.min(window.innerHeight, bounds.bottom) : window.innerHeight;
      const spaceBelow = bottomLimit - rect.bottom;
      const spaceAbove = rect.top - topLimit;

      if (placement === 'top' || placement === 'bottom') {
        setResolvedPlacement(placement);
      } else {
        setResolvedPlacement(spaceBelow < 240 && spaceAbove > spaceBelow ? 'top' : 'bottom');
      }

      if (variant === 'field') {
        setResolvedAlign('left');
      } else {
        const estimatedWidth = 260;
        if (align === 'right' && rect.right < estimatedWidth && window.innerWidth - rect.left >= rect.right) {
          setResolvedAlign('left');
        } else if (align === 'left' && window.innerWidth - rect.left < estimatedWidth && rect.right > window.innerWidth - rect.left) {
          setResolvedAlign('right');
        } else {
          setResolvedAlign(align);
        }
      }
    };
    measure();
    window.addEventListener('resize', measure, { passive: true });
    return () => window.removeEventListener('resize', measure);
  }, [open, placement, align, variant]);

  useEffect(() => {
    if (!open) return;
    const selectedIdx = filteredOptions.findIndex((o) => o.value === value);
    setHighlightedIndex(selectedIdx >= 0 ? selectedIdx : 0);
  }, [open, filter, value, filteredOptions]);

  useEffect(() => {
    if (!open) return;
    const onMouseDown = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        closeMenu(false);
      }
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        closeMenu(true);
        return;
      }
      if (filteredOptions.length === 0) return;
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setHighlightedIndex((prev) => {
          const next = (prev + 1) % filteredOptions.length;
          optionRefs.current[next]?.scrollIntoView({ block: 'nearest' });
          return next;
        });
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setHighlightedIndex((prev) => {
          const next = (prev - 1 + filteredOptions.length) % filteredOptions.length;
          optionRefs.current[next]?.scrollIntoView({ block: 'nearest' });
          return next;
        });
      } else if (e.key === 'Home') {
        e.preventDefault();
        setHighlightedIndex(0);
        optionRefs.current[0]?.scrollIntoView({ block: 'nearest' });
      } else if (e.key === 'End') {
        e.preventDefault();
        const last = filteredOptions.length - 1;
        setHighlightedIndex(last);
        optionRefs.current[last]?.scrollIntoView({ block: 'nearest' });
      } else if (e.key === 'Enter') {
        const chosen = filteredOptions[highlightedIndex];
        if (chosen) {
          e.preventDefault();
          onChange(chosen.value);
          closeMenu(true);
        }
      }
    };
    document.addEventListener('mousedown', onMouseDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onMouseDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open, filteredOptions, highlightedIndex, onChange]);

  useEffect(() => {
    if (open && searchable) {
      requestAnimationFrame(() => searchInputRef.current?.focus());
    }
  }, [open, searchable]);

  const isField = variant === 'field';

  return (
    <div ref={containerRef} className={`relative inline-flex ${isField ? 'w-full' : ''} ${className}`}>
      <button
        ref={triggerRef}
        type="button"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        onClick={() => {
          if (open) closeMenu(false);
          else setOpenState(true);
        }}
        className={
          isField
            ? `w-full h-10 inline-flex items-center justify-between gap-2.5 rounded-xl bg-zinc-900/90 hover:bg-zinc-800/90 px-3.5 text-sm font-medium text-white ring-1 ring-inset transition-all outline-none ${
                open ? 'ring-rose-500/60 bg-zinc-900' : 'ring-white/12 hover:ring-white/25'
              }`
            : compact
              ? `max-w-full min-w-0 h-8 inline-flex items-center justify-between gap-1.5 rounded-full bg-zinc-900/85 hover:bg-zinc-800/95 backdrop-blur-md px-2.5 text-xs font-medium text-zinc-100 hover:text-white ring-1 ring-inset shadow-sm transition-all outline-none ${
                  open ? 'ring-rose-500/60 bg-zinc-900 text-white' : 'ring-white/15 hover:ring-white/30'
                }`
              : `max-w-full min-w-0 h-9 inline-flex items-center justify-between gap-2 rounded-full bg-zinc-900/85 hover:bg-zinc-800/95 backdrop-blur-md px-3.5 text-xs sm:text-sm font-medium text-zinc-100 hover:text-white ring-1 ring-inset shadow-sm transition-all outline-none ${
                  open ? 'ring-rose-500/60 bg-zinc-900 text-white' : 'ring-white/15 hover:ring-white/30'
                }`
        }
      >
        <span className="inline-flex items-center gap-2 min-w-0 leading-snug">
          {icon && <span className="text-zinc-400 shrink-0 inline-flex items-center justify-center">{icon}</span>}
          <span className="truncate leading-snug">{selected?.label ?? ''}</span>
          {selected?.badge !== undefined && (
            <span className="px-1.5 py-0.5 rounded-full bg-white/[0.08] text-[10px] font-semibold text-zinc-300 leading-none text-trim shrink-0">
              {selected.badge}
            </span>
          )}
        </span>
        <ChevronDown
          className={`w-3.5 h-3.5 text-zinc-400 shrink-0 transition-transform duration-200 ${
            open ? 'rotate-180 text-rose-400' : ''
          }`}
        />
      </button>

      {open && (
        <div
          id={listboxId}
          role="listbox"
          aria-label={ariaLabel}
          className={`absolute ${
            resolvedPlacement === 'top' ? 'bottom-full mb-2' : 'top-full mt-2'
          } z-[70] ${
            isField
              ? 'left-0 right-0 w-full'
              : `${resolvedAlign === 'right' ? 'right-0' : 'left-0'} ${menuWidthClass} max-w-[calc(100vw-2rem)]`
          } rounded-2xl bg-zinc-900/95 backdrop-blur-2xl ring-1 ring-white/15 shadow-2xl shadow-black/80 overflow-hidden`}
        >
          {searchable && (
            <div className="p-2 border-b border-white/[0.08]">
              <div className="relative flex items-center">
                <Search className="absolute left-2.5 w-3.5 h-3.5 text-zinc-400 pointer-events-none" />
                <input
                  ref={searchInputRef}
                  type="text"
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                  placeholder={searchPlaceholder}
                  aria-label={searchPlaceholder}
                  className="w-full h-8 bg-white/[0.06] ring-1 ring-inset ring-white/10 focus:ring-rose-500/50 rounded-xl pl-8 pr-7 text-xs text-white placeholder-zinc-500 outline-none"
                />
                {filter && (
                  <button
                    type="button"
                    onClick={() => setFilter('')}
                    aria-label="Clear filter"
                    className="absolute right-2 w-5 h-5 inline-flex items-center justify-center rounded-full text-zinc-400 hover:text-white"
                  >
                    <X className="w-3 h-3" />
                  </button>
                )}
              </div>
            </div>
          )}

          <div className="max-h-64 overflow-y-auto p-1.5 space-y-0.5">
            {filteredOptions.length === 0 ? (
              <p className="py-4 px-3 text-center text-xs text-zinc-500">No matches</p>
            ) : (
              filteredOptions.map((opt, idx) => {
                const active = opt.value === value;
                const highlighted = idx === highlightedIndex;
                return (
                  <button
                    key={opt.value}
                    ref={(el) => {
                      optionRefs.current[idx] = el;
                    }}
                    type="button"
                    role="option"
                    aria-selected={active}
                    onMouseEnter={() => setHighlightedIndex(idx)}
                    onClick={() => {
                      onChange(opt.value);
                      closeMenu(true);
                    }}
                    className={`w-full inline-flex items-center justify-between gap-2.5 px-3 py-2 rounded-xl text-left text-xs sm:text-sm transition-colors ${
                      active
                        ? 'bg-rose-500/15 text-white font-semibold ring-1 ring-inset ring-rose-500/30'
                        : highlighted
                          ? 'bg-white/[0.09] text-white font-medium'
                          : 'text-zinc-300 hover:bg-white/[0.07] hover:text-white font-medium'
                    }`}
                  >
                    <span className="truncate leading-snug">{opt.label}</span>
                    <span className="inline-flex items-center gap-2 shrink-0">
                      {opt.badge !== undefined && (
                        <span
                          className={`px-1.5 py-0.5 rounded-md text-[10px] font-semibold leading-none text-trim ${
                            active ? 'bg-rose-500/25 text-rose-200' : 'bg-white/[0.06] text-zinc-400'
                          }`}
                        >
                          {opt.badge}
                        </span>
                      )}
                      {active && <Check className="w-3.5 h-3.5 text-rose-400 shrink-0" />}
                    </span>
                  </button>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
};
