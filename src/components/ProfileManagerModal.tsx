'use client';

import React, { useMemo, useState } from 'react';
import { Check, Plus, Trash2, Users } from 'lucide-react';
import { Modal } from '@/components/Modal';
import { HouseholdMember, getMemberDisplayName } from '@/lib/watchProgress';
import { PROFILE_BADGE_CLASSES } from '@/components/Navbar';
import { Badge, Button, Input, Switch } from '@/components/ds';

interface ProfileManagerModalProps {
  profiles: HouseholdMember[];
  activeProfileId: string;
  onSaveProfiles: (nextProfiles: HouseholdMember[], deletedIds: string[]) => void;
  onSwitchProfile: (profileId: string) => void;
  onClose: () => void;
}

const COLOR_OPTIONS: { id: string; label: string; dotClass: string }[] = [
  { id: 'rose', label: 'Rose', dotClass: 'bg-rose-500' },
  { id: 'sky', label: 'Sky', dotClass: 'bg-sky-500' },
  { id: 'emerald', label: 'Emerald', dotClass: 'bg-emerald-500' },
  { id: 'amber', label: 'Amber', dotClass: 'bg-amber-500' },
  { id: 'purple', label: 'Purple', dotClass: 'bg-purple-500' },
];

export const ProfileManagerModal: React.FC<ProfileManagerModalProps> = ({
  profiles,
  activeProfileId,
  onSaveProfiles,
  onSwitchProfile,
  onClose,
}) => {
  const [drafts, setDrafts] = useState<HouseholdMember[]>(() => profiles.map((p) => ({ ...p })));
  const [deletedIds, setDeletedIds] = useState<string[]>([]);
  const [newName, setNewName] = useState('');
  const [newNickname, setNewNickname] = useState('');
  const [newColor, setNewColor] = useState('emerald');
  const [newIsKids, setNewIsKids] = useState(false);

  const dirty = useMemo(() => JSON.stringify(drafts) !== JSON.stringify(profiles), [drafts, profiles]);

  const updateDraft = (id: string, patch: Partial<HouseholdMember>) => {
    setDrafts((prev) => prev.map((p) => (p.id === id ? { ...p, ...patch } : p)));
  };

  const handleDelete = (id: string) => {
    if (drafts.length <= 1) return;
    setDrafts((prev) => prev.filter((p) => p.id !== id));
    setDeletedIds((prev) => (prev.includes(id) ? prev : [...prev, id]));
  };

  const normalizeDrafts = (list: HouseholdMember[]): HouseholdMember[] =>
    list
      .map((p) => {
        const name = p.name.trim() || 'Member';
        const nickname = p.nickname?.trim() || '';
        return {
          id: p.id,
          name,
          ...(nickname ? { nickname } : {}),
          color: p.color || 'rose',
          ...(p.isKids ? { isKids: true } : {}),
        };
      })
      .filter((p) => Boolean(p.id));

  const handleAddMember = () => {
    const trimmedName = newName.trim();
    if (!trimmedName) return;
    const baseSlug = trimmedName
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 18);
    const id = `${baseSlug || 'member'}-${Date.now().toString(36).slice(-4)}`;
    const trimmedNick = newNickname.trim();
    const created: HouseholdMember = {
      id,
      name: trimmedName,
      ...(trimmedNick ? { nickname: trimmedNick } : {}),
      color: newColor,
      ...(newIsKids ? { isKids: true } : {}),
    };
    const nextDrafts = normalizeDrafts([...drafts, created]);
    onSaveProfiles(nextDrafts, deletedIds);
    setDrafts(nextDrafts);
    setDeletedIds([]);
    setNewName('');
    setNewNickname('');
    setNewIsKids(false);
  };

  const handleCommit = () => {
    const cleaned = normalizeDrafts(drafts);
    if (cleaned.length === 0) return;
    onSaveProfiles(cleaned, deletedIds);
    setDeletedIds([]);
    onClose();
  };

  return (
    <Modal onClose={onClose} maxWidth="max-w-2xl" label="Manage household profiles">
      <div className="p-6 sm:p-8 space-y-6">
        <div className="pr-10">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-rose-500/15 ring-1 ring-rose-500/35 inline-flex items-center justify-center shrink-0">
              <Users className="w-4 h-4 text-rose-400" />
            </div>
            <div>
              <h2 className="font-display text-xl sm:text-2xl font-bold text-white leading-tight">
                Household Profiles
              </h2>
              <p className="text-xs sm:text-sm text-zinc-400 mt-0.5">
                Set nicknames, pick colors, or turn any profile into a Kids-safe account.
              </p>
            </div>
          </div>
        </div>

        <div className="space-y-3">
          {drafts.map((member) => {
            const isActive = member.id === activeProfileId;
            const badgeClass = PROFILE_BADGE_CLASSES[member.color] || PROFILE_BADGE_CLASSES.rose;
            const displayName = getMemberDisplayName(member);

            return (
              <div
                key={member.id}
                className={`p-4 rounded-2xl ring-1 ring-inset transition-colors ${
                  isActive ? 'bg-rose-500/[0.06] ring-rose-500/35' : 'bg-white/[0.03] ring-white/[0.08]'
                }`}
              >
                <div className="flex flex-wrap items-center justify-between gap-3 mb-3">
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className={`w-9 h-9 rounded-full ring-1 inline-flex items-center justify-center text-sm font-bold uppercase leading-none shrink-0 ${badgeClass}`}
                    >
                      <span className="text-trim">{displayName.slice(0, 1)}</span>
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-semibold text-white text-sm truncate">{displayName}</span>
                        {isActive && (
                          <Badge variant="accent">Active</Badge>
                        )}
                        {member.isKids && (
                          <Badge variant="success">Kids safe</Badge>
                        )}
                      </div>
                      <p className="text-[11px] text-zinc-400 truncate mt-0.5">
                        Profile: {member.name}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {!isActive && (
                      <Button
                        size="sm"
                        onPress={() => {
                          const cleaned = normalizeDrafts(drafts);
                          onSaveProfiles(cleaned, deletedIds);
                          setDeletedIds([]);
                          onSwitchProfile(member.id);
                          onClose();
                        }}
                      >
                        Switch to
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      onPress={() => handleDelete(member.id)}
                      disabled={drafts.length <= 1}
                      title={drafts.length <= 1 ? 'At least one profile must remain' : `Delete ${displayName}`}
                      aria-label={`Delete ${displayName}`}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </Button>
                  </div>
                </div>

                <div className="grid sm:grid-cols-2 gap-3">
                  <div>
                    <p className="text-[11px] font-medium text-zinc-400 mb-1">Nickname (shown in bar)</p>
                    <Input
                      aria-label={`Nickname for ${member.name}`}
                      value={member.nickname || ''}
                      maxLength={24}
                      placeholder={member.name}
                      onChange={(nickname) => updateDraft(member.id, { nickname })}
                    />
                  </div>
                  <div>
                    <p className="text-[11px] font-medium text-zinc-400 mb-1">Profile name</p>
                    <Input
                      aria-label={`Profile name for ${displayName}`}
                      value={member.name}
                      maxLength={24}
                      onChange={(name) => updateDraft(member.id, { name })}
                    />
                  </div>
                </div>

                <div className="mt-3 pt-3 border-t border-white/[0.06] flex flex-wrap items-center justify-between gap-3">
                  <div className="flex items-center gap-2">
                    <span className="text-[11px] font-medium text-zinc-400">Color:</span>
                    <div className="flex items-center gap-1.5">
                      {COLOR_OPTIONS.map((c) => {
                        const selected = member.color === c.id;
                        return (
                          <button
                            key={c.id}
                            type="button"
                            onClick={() => updateDraft(member.id, { color: c.id })}
                            aria-label={`${c.label} avatar color for ${displayName}`}
                            aria-pressed={selected}
                            className={`w-6 h-6 rounded-full inline-flex items-center justify-center transition-transform ${
                              c.dotClass
                            } ${selected ? 'ring-2 ring-white ring-offset-2 ring-offset-zinc-900 scale-105' : 'opacity-70 hover:opacity-100'}`}
                          >
                            {selected && <Check className="w-3 h-3 text-white stroke-[3]" />}
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <Switch
                    label={`Kids account for ${displayName}`}
                    checked={Boolean(member.isKids)}
                    onChange={(isKids) => updateDraft(member.id, { isKids })}
                  >
                    Kids account
                  </Switch>
                </div>
              </div>
            );
          })}
        </div>

        <div className="p-4 rounded-2xl bg-white/[0.03] ring-1 ring-inset ring-white/[0.08] space-y-3">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-semibold text-white">Add another profile</h3>
            <Switch label="Kids account for new profile" checked={newIsKids} onChange={setNewIsKids}>
              Kids account
            </Switch>
          </div>
          <div className="grid sm:grid-cols-2 gap-2.5">
            <Input
              value={newName}
              maxLength={24}
              placeholder={newIsKids ? 'Name (e.g. Kids, Leo)' : 'Name (e.g. Alex)'}
              aria-label="New profile name"
              onChange={setNewName}
              onEnter={handleAddMember}
            />
            <Input
              value={newNickname}
              maxLength={24}
              placeholder="Nickname (optional)"
              aria-label="New profile nickname"
              onChange={setNewNickname}
              onEnter={handleAddMember}
            />
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-1.5">
              {COLOR_OPTIONS.map((c) => {
                const selected = newColor === c.id;
                return (
                  <button
                    key={c.id}
                    type="button"
                    onClick={() => setNewColor(c.id)}
                    aria-label={`${c.label} color for new profile`}
                    aria-pressed={selected}
                    className={`w-6 h-6 rounded-full inline-flex items-center justify-center transition-transform ${c.dotClass} ${
                      selected ? 'ring-2 ring-white ring-offset-2 ring-offset-zinc-900 scale-105' : 'opacity-70 hover:opacity-100'
                    }`}
                  >
                    {selected && <Check className="w-3 h-3 text-white stroke-[3]" />}
                  </button>
                );
              })}
            </div>
            <Button onPress={handleAddMember} disabled={!newName.trim()}>
              <Plus className="w-3.5 h-3.5 shrink-0" />
              Add profile
            </Button>
          </div>
        </div>

        <div className="pt-2 border-t border-white/[0.08] flex flex-wrap items-center justify-between gap-3">
          <span className="text-xs text-zinc-400">
            {dirty || deletedIds.length > 0 ? 'You have unsaved profile changes' : 'All changes saved'}
          </span>
          <div className="flex items-center gap-2">
            <Button variant="ghost" onPress={onClose}>Close</Button>
            <Button variant="primary" onPress={handleCommit} disabled={!dirty && deletedIds.length === 0}>
              Save changes
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
};
