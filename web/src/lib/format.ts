export function fmtDuration(ms: number): string {
  if (!ms || ms <= 0) return '–';
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  const rem = Math.round(s % 60);
  return `${m}m${rem}s`;
}

export function fmtTime(iso?: string): string {
  if (!iso) return '–';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '–';
  return d.toLocaleString();
}

// A short, readable handle for a run id like "20260529T170019Z-1386d9".
export function shortId(id: string): string {
  return id.length > 16 ? id.slice(0, 16) + '…' : id;
}
