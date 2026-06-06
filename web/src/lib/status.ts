import type { UiStatus } from './types';

// Status → ASCII glyph + color class + motion, ported from the design
// (parts.jsx STATUS_META). The system uses no icon font: status is a
// bracketed glyph in a phosphor color, with a soft pulse only for "running".

export type Motion = 'none' | 'pulse';

export interface StatusMeta {
  glyph: string;
  /** color helper class from tokens.css: '' (fg), 'accent', 'error', 'dim'. */
  cls: '' | 'accent' | 'error' | 'dim';
  label: string;
  motion: Motion;
}

export const STATUS_META: Record<UiStatus, StatusMeta> = {
  passed: { glyph: '[OK]', cls: 'accent', label: 'PASSED', motion: 'none' },
  failed: { glyph: '[XX]', cls: 'error', label: 'FAILED', motion: 'none' },
  running: { glyph: '[..]', cls: 'accent', label: 'RUNNING', motion: 'pulse' },
  queued: { glyph: '[//]', cls: 'dim', label: 'QUEUED', motion: 'none' },
  skipped: { glyph: '[--]', cls: 'dim', label: 'SKIPPED', motion: 'none' },
  idle: { glyph: '[  ]', cls: 'dim', label: 'IDLE', motion: 'none' },
};

export function statusMeta(status: UiStatus): StatusMeta {
  return STATUS_META[status] ?? STATUS_META.idle;
}

/** The CRT phosphor border/accent color var for a status (job cards, edges). */
export function statusColor(status: UiStatus): string {
  switch (status) {
    case 'failed':
      return 'var(--term-error)';
    case 'running':
      return 'var(--term-accent)';
    case 'passed':
      return 'var(--term-fg)';
    default:
      return 'var(--term-dim)';
  }
}

/** The matching low-alpha glow var for a status — always pair with the color. */
export function statusGlow(status: UiStatus): string {
  switch (status) {
    case 'failed':
      return 'var(--term-glow-error)';
    case 'running':
      return 'var(--term-glow-accent)';
    case 'passed':
      return 'var(--term-glow)';
    default:
      return 'transparent';
  }
}

/** The bar-fill modifier class for a status, '' meaning the default fg fill. */
export function barFillClass(status: UiStatus): '' | 'accent' | 'error' {
  if (status === 'failed') return 'error';
  if (status === 'running') return 'accent';
  return '';
}
