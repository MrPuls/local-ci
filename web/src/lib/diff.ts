// Minimal line diff (LCS, O(n·m)) for the editor's discard-confirm preview.
// Configs are small files; cap the quadratic table and fall back to a blunt
// "everything changed" view beyond it.

export interface DiffLine {
  kind: 'same' | 'add' | 'del';
  text: string;
}

const MAX_CELLS = 4_000_000; // ~2000×2000 lines

export function diffLines(a: string, b: string): DiffLine[] {
  const A = a.split('\n');
  const B = b.split('\n');
  if (A.length * B.length > MAX_CELLS) {
    return [
      ...A.map((text): DiffLine => ({ kind: 'del', text })),
      ...B.map((text): DiffLine => ({ kind: 'add', text })),
    ];
  }

  // LCS table.
  const n = A.length;
  const m = B.length;
  const dp: Uint32Array[] = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = A[i] === B[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }

  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (A[i] === B[j]) {
      out.push({ kind: 'same', text: A[i] });
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      out.push({ kind: 'del', text: A[i++] });
    } else {
      out.push({ kind: 'add', text: B[j++] });
    }
  }
  while (i < n) out.push({ kind: 'del', text: A[i++] });
  while (j < m) out.push({ kind: 'add', text: B[j++] });
  return out;
}

/** Collapses unchanged stretches to context lines around changes. */
export function compactDiff(diff: DiffLine[], context = 2): (DiffLine | { kind: 'gap'; count: number })[] {
  const keep = new Array<boolean>(diff.length).fill(false);
  diff.forEach((d, i) => {
    if (d.kind === 'same') return;
    for (let k = Math.max(0, i - context); k <= Math.min(diff.length - 1, i + context); k++) {
      keep[k] = true;
    }
  });
  const out: (DiffLine | { kind: 'gap'; count: number })[] = [];
  let gap = 0;
  diff.forEach((d, i) => {
    if (keep[i]) {
      if (gap > 0) {
        out.push({ kind: 'gap', count: gap });
        gap = 0;
      }
      out.push(d);
    } else {
      gap++;
    }
  });
  if (gap > 0) out.push({ kind: 'gap', count: gap });
  return out;
}
