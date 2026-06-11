// Client-side config lint: cheap, line-based checks that catch the classic
// mistakes before the YAML is even saved (the server stays the source of
// truth at save time). Deliberately conservative — line-oriented parsing of
// the config subset, with all cross-checks skipped when `include:` is present
// (stages/jobs may then come from other files we can't see).

export interface LintIssue {
  line: number; // 1-based
  message: string;
}

const RESERVED = new Set([
  'stages',
  'variables',
  'bootstrap',
  'cleanup',
  'remote_provider',
  'include',
]);

interface JobBlock {
  name: string;
  line: number;
  stage?: { value: string; line: number };
  needs: { value: string; line: number }[];
  hasImage: boolean;
  hasScript: boolean;
  hasExtends: boolean;
}

export function lintConfig(text: string): LintIssue[] {
  const lines = text.split('\n');
  const stages: string[] = [];
  const jobs: JobBlock[] = [];
  let hasInclude = false;
  let hasStagesKey = false;

  let section: 'stages' | 'include' | 'job' | 'needs' | null = null;
  let job: JobBlock | null = null;

  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i];
    const line = raw.replace(/#.*$/, ''); // good enough: configs rarely quote '#'
    if (line.trim() === '') continue;

    // Top-level key?
    const top = line.match(/^([^\s:][^:]*):\s*(.*)$/);
    if (top) {
      const key = top[1].trim();
      const rest = top[2].trim();
      job = null;
      section = null;
      if (key === 'stages') {
        hasStagesKey = true;
        section = 'stages';
        const inline = rest.match(/^\[(.*)\]$/);
        if (inline) {
          for (const s of inline[1].split(',')) {
            const v = s.trim().replace(/^['"]|['"]$/g, '');
            if (v) stages.push(v);
          }
          section = null;
        }
        continue;
      }
      if (key === 'include') {
        hasInclude = true;
        continue;
      }
      if (RESERVED.has(key)) continue;
      job = { name: key, line: i + 1, needs: [], hasImage: false, hasScript: false, hasExtends: false };
      jobs.push(job);
      section = 'job';
      continue;
    }

    // Indented content.
    if (section === 'stages') {
      const item = line.match(/^\s+-\s+(.+?)\s*$/);
      if (item) stages.push(item[1].replace(/^['"]|['"]$/g, ''));
      continue;
    }
    if (job === null) continue;

    const field = line.match(/^\s{1,8}(\w+):\s*(.*)$/);
    if (field && /^\s{1,8}\w/.test(line) && line.match(/^(\s+)/)![1].length <= 4) {
      const key = field[1];
      const value = field[2].trim();
      section = 'job';
      switch (key) {
        case 'stage':
          job.stage = { value: value.replace(/^['"]|['"]$/g, ''), line: i + 1 };
          break;
        case 'image':
          job.hasImage = value !== '';
          break;
        case 'script':
          job.hasScript = true;
          break;
        case 'extends':
          job.hasExtends = true;
          break;
        case 'needs': {
          const inline = value.match(/^\[(.*)\]$/);
          if (inline) {
            for (const n of inline[1].split(',')) {
              const v = n.trim().replace(/^['"]|['"]$/g, '');
              if (v) job.needs.push({ value: v, line: i + 1 });
            }
          } else if (value !== '') {
            job.needs.push({ value: value.replace(/^['"]|['"]$/g, ''), line: i + 1 });
          } else {
            section = 'needs';
          }
          break;
        }
      }
      continue;
    }
    if (section === 'needs') {
      const item = line.match(/^\s+-\s+(.+?)\s*$/);
      if (item) job.needs.push({ value: item[1].replace(/^['"]|['"]$/g, ''), line: i + 1 });
      else section = 'job';
    }
  }

  // --- checks ---------------------------------------------------------
  const issues: LintIssue[] = [];
  const jobNames = new Set(jobs.map((j) => j.name));
  const isTemplate = (name: string) => name.startsWith('.');

  for (const j of jobs) {
    if (isTemplate(j.name)) continue;
    if (!hasInclude) {
      if (!j.stage && !j.hasExtends) {
        issues.push({ line: j.line, message: `job "${j.name}" has no stage` });
      }
      if (j.stage && hasStagesKey && stages.length > 0 && !stages.includes(j.stage.value)) {
        issues.push({
          line: j.stage.line,
          message: `stage "${j.stage.value}" is not in stages [${stages.join(', ')}]`,
        });
      }
      if (!j.hasImage && !j.hasExtends) {
        issues.push({ line: j.line, message: `job "${j.name}" has no image` });
      }
      if (!j.hasScript && !j.hasExtends) {
        issues.push({ line: j.line, message: `job "${j.name}" has no script` });
      }
    }
    for (const n of j.needs) {
      if (n.value === j.name) {
        issues.push({ line: n.line, message: `job "${j.name}" needs itself` });
      } else if (!hasInclude && !jobNames.has(n.value)) {
        issues.push({ line: n.line, message: `needs "${n.value}" — no such job` });
      }
    }
  }
  return issues;
}
