import { reactive } from 'vue';
import { countByStatus, type PipelineStage, type StatusCounts } from '@/lib/pipeline';

// A small shared summary of the run currently on screen. The PipelineView owns
// the live run and publishes here; the global TopBar (STATUS) and StatusBar
// (totals) subscribe. This keeps run control in the run view while the chrome
// stays global.

export type RunKind = 'idle' | 'running' | 'done' | 'error';

interface RunSummary {
  label: string;
  kind: RunKind;
  counts: StatusCounts;
}

const empty = (): StatusCounts => ({
  passed: 0,
  failed: 0,
  running: 0,
  queued: 0,
  skipped: 0,
  idle: 0,
});

const summary = reactive<RunSummary>({ label: 'STANDBY', kind: 'idle', counts: empty() });

export function useRunStatus() {
  function set(label: string, kind: RunKind): void {
    summary.label = label;
    summary.kind = kind;
  }

  function setCounts(stages: PipelineStage[]): void {
    summary.counts = countByStatus(stages);
  }

  function reset(): void {
    summary.label = 'STANDBY';
    summary.kind = 'idle';
    summary.counts = empty();
  }

  return { summary, set, setCounts, reset };
}
