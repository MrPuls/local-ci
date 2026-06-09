import type { UiStatus } from './types';

// Status → pixel-grid icon + color class + motion. Each status maps to a crisp
// SVG from the design set (rendered via <Icon>, inheriting the phosphor color
// through currentColor); "running" spins. The icon name indexes
// web/src/assets/icons/<name>.svg.

export type Motion = 'none' | 'pulse';

export interface StatusMeta {
  /** Icon file name (without extension) under src/assets/icons. */
  icon: string;
  /** color helper class from tokens.css: '' (fg), 'accent', 'error', 'dim'. */
  cls: '' | 'accent' | 'error' | 'dim';
  label: string;
  motion: Motion;
}

export const STATUS_META: Record<UiStatus, StatusMeta> = {
  passed: { icon: 'check', cls: 'accent', label: 'PASSED', motion: 'none' },
  failed: { icon: 'cross', cls: 'error', label: 'FAILED', motion: 'none' },
  running: { icon: 'spinner', cls: 'accent', label: 'RUNNING', motion: 'pulse' },
  queued: { icon: 'hourglass', cls: 'dim', label: 'QUEUED', motion: 'none' },
  skipped: { icon: 'skip', cls: 'dim', label: 'SKIPPED', motion: 'none' },
  idle: { icon: 'dot', cls: 'dim', label: 'IDLE', motion: 'none' },
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
