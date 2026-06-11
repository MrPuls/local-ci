import { describe, expect, it } from 'vitest';
import { ansiLine, hasAnsi, newSgrState } from '@/lib/ansi';
import { diffLines } from '@/lib/diff';
import { lintConfig } from '@/lib/lint';

describe('ansi', () => {
  it('splits SGR-colored text into classed spans', () => {
    const spans = ansiLine('\x1b[32mGREEN\x1b[0m plain \x1b[1;31mBOLDRED\x1b[0m', newSgrState());
    expect(spans).toEqual([
      { text: 'GREEN', classes: ['a-fg32'] },
      { text: ' plain ', classes: [] },
      { text: 'BOLDRED', classes: ['a-fg31', 'a-bold'] },
    ]);
  });

  it('carries state across lines until reset', () => {
    const state = newSgrState();
    ansiLine('\x1b[33mstart', state);
    const next = ansiLine('still yellow', state);
    expect(next[0].classes).toContain('a-fg33');
  });

  it('strips non-SGR escapes and emulates \\r overwrite', () => {
    expect(ansiLine('\x1b[2K\x1b[1Gclean', newSgrState())).toEqual([
      { text: 'clean', classes: [] },
    ]);
    expect(ansiLine('progress 10%\rprogress 99%', newSgrState())).toEqual([
      { text: 'progress 99%', classes: [] },
    ]);
  });

  it('hasAnsi pre-check', () => {
    expect(hasAnsi('plain')).toBe(false);
    expect(hasAnsi('a\x1b[1mb')).toBe(true);
  });
});

describe('diffLines', () => {
  it('marks additions, deletions, and unchanged lines', () => {
    const d = diffLines('a\nb\nc', 'a\nB\nc\nd');
    expect(d).toEqual([
      { kind: 'same', text: 'a' },
      { kind: 'del', text: 'b' },
      { kind: 'add', text: 'B' },
      { kind: 'same', text: 'c' },
      { kind: 'add', text: 'd' },
    ]);
  });
});

describe('lintConfig', () => {
  const base = 'stages:\n  - build\n  - test\n';

  it('accepts a clean config', () => {
    const issues = lintConfig(
      base + 'Build:\n  stage: build\n  image: alpine\n  script:\n    - true\n',
    );
    expect(issues).toEqual([]);
  });

  it('flags an undefined stage with the offending line', () => {
    const text = base + 'Build:\n  stage: nope\n  image: alpine\n  script:\n    - true\n';
    const issues = lintConfig(text);
    expect(issues).toHaveLength(1);
    expect(issues[0].message).toContain('"nope"');
    expect(issues[0].line).toBe(5); // the `stage: nope` line
  });

  it('flags missing image/script and unknown needs targets', () => {
    const text = base + 'Build:\n  stage: build\n  needs: [Ghost]\n';
    const messages = lintConfig(text).map((i) => i.message);
    expect(messages.some((m) => m.includes('no image'))).toBe(true);
    expect(messages.some((m) => m.includes('no script'))).toBe(true);
    expect(messages.some((m) => m.includes('Ghost'))).toBe(true);
  });

  it('stays quiet for templates, extends, and include-based configs', () => {
    expect(
      lintConfig(base + '.tmpl:\n  image: alpine\nBuild:\n  stage: build\n  extends: .tmpl\n'),
    ).toEqual([]);
    expect(lintConfig('include:\n  - other.yaml\nBuild:\n  stage: ext\n')).toEqual([]);
  });
});
