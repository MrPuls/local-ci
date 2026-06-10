// Minimal YAML syntax highlighter for the CRT config editor. Hand-rolled on
// purpose: a few regexes themed with the terminal palette beat shipping a
// full editor dependency for one file type. Classes (.y-*) live in app.css.

const ESC: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;' };
const esc = (s: string): string => s.replace(/[&<>]/g, (c) => ESC[c]);

// Scalar/flow-value tokens: quoted strings, $VAR / ${VAR} refs, booleans/null,
// bare numbers. Applied to already-escaped text (only &<> change, so quotes
// and word boundaries survive).
const VALUE_RE =
  /('[^']*'|"[^"]*")|(\$\{?[A-Za-z_]\w*\}?)|\b(true|false|null|yes|no|on|off)\b|(?<=^|[\s,[{:])(-?\d+(?:\.\d+)?)(?=$|[\s,\]}])/g;

function highlightValue(v: string): string {
  return esc(v).replace(VALUE_RE, (m, str, vari, bool, num) => {
    if (str) return `<span class="y-str">${m}</span>`;
    if (vari) return `<span class="y-var">${m}</span>`;
    if (bool) return `<span class="y-bool">${m}</span>`;
    if (num) return `<span class="y-num">${m}</span>`;
    return m;
  });
}

// Splits a raw line into code + trailing comment: the first '#' that is not
// inside quotes and sits at the start or after whitespace.
function splitComment(line: string): [string, string] {
  let inSingle = false;
  let inDouble = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === "'" && !inDouble) inSingle = !inSingle;
    else if (ch === '"' && !inSingle) inDouble = !inDouble;
    else if (ch === '#' && !inSingle && !inDouble && (i === 0 || /\s/.test(line[i - 1]))) {
      return [line.slice(0, i), line.slice(i)];
    }
  }
  return [line, ''];
}

export function highlightYamlLine(raw: string): string {
  const [code, comment] = splitComment(raw);
  const commentHtml = comment ? `<span class="y-comment">${esc(comment)}</span>` : '';

  // "indent [- ] key:" — key gets the accent, a list dash gets its own mark.
  const keyMatch = code.match(/^(\s*)(- +)?([^\s:][^:]*?)(:)(\s.*|)$/);
  if (keyMatch) {
    const [, indent, dash, key, colon, rest] = keyMatch;
    return (
      esc(indent) +
      (dash ? `<span class="y-dash">${esc(dash)}</span>` : '') +
      `<span class="y-key">${esc(key)}</span>` +
      `<span class="y-dash">${colon}</span>` +
      highlightValue(rest) +
      commentHtml
    );
  }

  // "- item" list entries and plain scalar lines.
  const dashMatch = code.match(/^(\s*)(- +)(.*)$/);
  if (dashMatch) {
    const [, indent, dash, rest] = dashMatch;
    return (
      esc(indent) + `<span class="y-dash">${esc(dash)}</span>` + highlightValue(rest) + commentHtml
    );
  }
  return highlightValue(code) + commentHtml;
}

export function highlightYaml(text: string): string {
  return text.split('\n').map(highlightYamlLine).join('\n');
}
