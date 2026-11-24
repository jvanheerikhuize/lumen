'use client';

import React from 'react';
import '@/vendor/design-system/ui-button.js';
import '@/vendor/design-system/ui-chip.js';
import '@/vendor/design-system/ui-switch.js';
import '@/vendor/design-system/ui-badge.js';
import '@/vendor/design-system/ui-input.js';

// React wrappers around the design system atoms. The atoms keep their own state; the wrappers put
// them back in step with props after each event, so React state stays the only source of truth.

type HostProps = React.HTMLAttributes<HTMLElement> & { ref?: React.Ref<HTMLElement>; [attribute: string]: unknown };

declare module 'react' {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace JSX {
    interface IntrinsicElements {
      'ui-button': HostProps;
      'ui-chip': HostProps;
      'ui-switch': HostProps;
      'ui-badge': HostProps;
      'ui-input': HostProps;
    }
  }
}

interface ButtonProps {
  onPress: () => void;
  children: React.ReactNode;
  variant?: 'primary' | 'secondary' | 'ghost' | 'surface' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  busy?: boolean;
  /** Makes this a toggle button. */
  pressed?: boolean;
  title?: string;
  className?: string;
  'aria-label'?: string;
}

export function Button({ onPress, variant = 'secondary', size = 'md', disabled = false, busy = false, pressed, ...rest }: ButtonProps) {
  return (
    <ui-button
      {...rest}
      variant={variant}
      size={size}
      disabled={disabled}
      busy={busy}
      aria-pressed={pressed === undefined ? undefined : pressed ? 'true' : 'false'}
      onui-press={onPress}
    />
  );
}

interface ChipProps {
  pressed: boolean;
  onPress: () => void;
  children: React.ReactNode;
  accent?: boolean;
}

export function Chip({ pressed, onPress, children, accent = false }: ChipProps) {
  return (
    <ui-chip
      pressed={pressed}
      variant={accent ? 'accent' : undefined}
      onui-toggle={(e: CustomEvent) => {
        (e.currentTarget as HTMLElement & { pressed: boolean }).pressed = pressed;
        onPress();
      }}
    >
      {children}
    </ui-chip>
  );
}

interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  children?: React.ReactNode;
}

export function Switch({ checked, onChange, label, children }: SwitchProps) {
  return (
    <ui-switch
      checked={checked}
      aria-label={label}
      onui-change={(e: CustomEvent<{ checked: boolean }>) => {
        (e.currentTarget as HTMLElement & { checked: boolean }).checked = checked;
        onChange(e.detail.checked);
      }}
    >
      {children}
    </ui-switch>
  );
}

export function Badge({ variant, children }: { variant: 'neutral' | 'success' | 'accent'; children: React.ReactNode }) {
  return <ui-badge variant={variant}>{children}</ui-badge>;
}

interface InputProps {
  value: string;
  onChange: (value: string) => void;
  'aria-label': string;
  type?: 'text' | 'password';
  placeholder?: string;
  maxLength?: number;
  invalid?: boolean;
  onEnter?: () => void;
  className?: string;
}

export function Input({ value, onChange, type = 'text', maxLength, invalid = false, onEnter, ...rest }: InputProps) {
  return (
    <ui-input
      {...rest}
      type={type}
      maxlength={maxLength}
      value={value}
      invalid={invalid}
      onui-input={(e: CustomEvent<{ value: string }>) => onChange(e.detail.value)}
      onKeyDown={(e: React.KeyboardEvent) => {
        if (e.key === 'Enter' && onEnter) {
          e.preventDefault();
          onEnter();
        }
      }}
    />
  );
}
