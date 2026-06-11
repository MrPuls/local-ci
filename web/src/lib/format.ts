// Formatting helpers. The brand voice is tabular + uppercase: durations are
// "12.8S" (no space, uppercase unit), times are clock-style, ids are short.

/** "12.8S" / "1M03S" / "420MS" — uppercase, no space, matching the design. */
export function fmtDuration(ms: number): string {
  if (!ms || ms <= 0) return '--';
  if (ms < 1000) return `${Math.round(ms)}MS`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}S`;
  const m = Math.floor(s / 60);
  const rem = Math.round(s % 60);
  return `${m}M${String(rem).padStart(2, '0')}S`;
}

/** Seconds as a bare number for table cells, e.g. "12.8". */
export function fmtSeconds(ms: number): string {
  if (!ms || ms <= 0) return '--';
  return (ms / 1000).toFixed(1);
}

/** Local wall-clock time "14:33:47". */
export function fmtClock(iso?: string): string {
  if (!iso) return '--';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '--';
  return d.toLocaleTimeString([], { hour12: false });
}

/** Local date + time for history rows. */
export function fmtDateTime(iso?: string): string {
  if (!iso) return '--';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '--';
  return d.toLocaleString([], { hour12: false });
}

/** A short, readable handle for a run id like "20260529T170019Z-1386d9". */
export function shortId(id: string): string {
  return id.length > 20 ? id.slice(0, 20) + '…' : id;
}

/** Byte count as "0 B" / "12.4 KB" / "1.2 MB" (binary units). */
export function fmtBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let n = bytes;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${i === 0 ? n : n.toFixed(1)} ${units[i]}`;
}

/** Final path component, e.g. "/proj/.local-ci.yaml" -> ".LOCAL-CI.YAML". */
export function baseName(path?: string): string {
  if (!path) return '--';
  const parts = path.split(/[/\\]/);
  return parts[parts.length - 1] || path;
}

/** Git context as "branch@1a2b3c4" — '--' when the run wasn't in a git repo. */
export function gitRef(commit?: string, branch?: string): string {
  if (!commit) return '--';
  const sha = commit.slice(0, 7);
  return branch ? `${branch}@${sha}` : sha;
}
