// Minimal ANSI SGR renderer for the log feed: turns escape-coded tool output
// into styled spans (classes defined in app.css, tinted to the CRT palette).
// SGR (`...m`) sequences become styling; every other escape sequence and
// carriage-return trickery is stripped so logs read clean.

export interface AnsiSpan {
  text: string;
  /** CSS classes, e.g. ['a-fg31', 'a-bold']. Empty for plain text. */
  classes: string[];
}

interface SgrState {
  fg: number | null; // 30-37 / 90-97
  bold: boolean;
  dim: boolean;
  underline: boolean;
}

const freshState = (): SgrState => ({ fg: null, bold: false, dim: false, underline: false });

function classesOf(s: SgrState): string[] {
  const out: string[] = [];
  if (s.fg !== null) out.push(`a-fg${s.fg}`);
  if (s.bold) out.push('a-bold');
  if (s.dim) out.push('a-dim');
  if (s.underline) out.push('a-underline');
  return out;
}

function applySgr(state: SgrState, params: string): void {
  const codes = params === '' ? [0] : params.split(';').map((p) => parseInt(p, 10) || 0);
  for (let i = 0; i < codes.length; i++) {
    const c = codes[i];
    if (c === 0) Object.assign(state, freshState());
    else if (c === 1) state.bold = true;
    else if (c === 2) state.dim = true;
    else if (c === 4) state.underline = true;
    else if (c === 22) (state.bold = false), (state.dim = false);
    else if (c === 24) state.underline = false;
    else if ((c >= 30 && c <= 37) || (c >= 90 && c <= 97)) state.fg = c;
    else if (c === 39) state.fg = null;
    else if (c === 38 || c === 48) {
      // 256/truecolor: consume the arguments, approximate 256-color fg to the
      // base palette, ignore backgrounds (illegible over the CRT scanlines).
      const mode = codes[i + 1];
      if (mode === 5) {
        if (c === 38) state.fg = approx256(codes[i + 2] ?? 0);
        i += 2;
      } else if (mode === 2) {
        i += 4;
      }
    }
    // backgrounds (40-47, 100-107) and the rest: intentionally ignored
  }
}

/** Maps a 256-color index onto the base 30-37/90-97 range. */
function approx256(n: number): number {
  if (n < 8) return 30 + n;
  if (n < 16) return 90 + (n - 8);
  if (n >= 232) return n < 244 ? 90 : 97; // grayscale ramp
  // 6x6x6 cube: pick the dominant channel
  const idx = n - 16;
  const r = Math.floor(idx / 36);
  const g = Math.floor((idx % 36) / 6);
  const b = idx % 6;
  if (r >= g && r >= b) return g >= 3 ? 33 : 31; // reddish / yellowish
  if (g >= r && g >= b) return b >= 3 ? 36 : 32; // greenish / cyanish
  return b >= 2 ? 34 : 35;
}

// CSI (\x1b[...X), OSC (\x1b]...BEL/ST) and other stray escapes.
const ESCAPE_RE = /\x1b\[([0-9;]*)m|\x1b\[[0-9;?]*[A-Za-ln-z]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?|\x1b./g;

/**
 * Renders one log line into styled spans, carrying SGR state across calls via
 * the caller-owned `state` (pass the previous line's returned state).
 * A carriage return keeps only the text after the last \r — the terminal
 * behavior progress bars rely on.
 */
export function ansiLine(raw: string, state: SgrState): AnsiSpan[] {
  // Emulate \r overwrite: keep the final segment (ignoring a trailing \r).
  let line = raw;
  if (line.includes('\r')) {
    const segments = line.replace(/\r+$/, '').split('\r');
    line = segments[segments.length - 1] ?? '';
  }

  const spans: AnsiSpan[] = [];
  let last = 0;
  ESCAPE_RE.lastIndex = 0;
  for (let m = ESCAPE_RE.exec(line); m !== null; m = ESCAPE_RE.exec(line)) {
    if (m.index > last) {
      spans.push({ text: line.slice(last, m.index), classes: classesOf(state) });
    }
    if (m[1] !== undefined) applySgr(state, m[1]); // SGR; others just stripped
    last = m.index + m[0].length;
  }
  if (last < line.length) {
    spans.push({ text: line.slice(last), classes: classesOf(state) });
  }
  return spans;
}

export function newSgrState(): SgrState {
  return freshState();
}

/** True when the line contains any escape byte (cheap pre-check). */
export function hasAnsi(line: string): boolean {
  return line.includes('\x1b') || line.includes('\r');
}
