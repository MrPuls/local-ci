import type { RunStatus, WireEvent } from './types';

// LiveState is reconstructed entirely from a run's SSE event stream (which
// replays from the start, then goes live), so it is the single source of truth
// for a run's view — whether the run is active or already finished.

export interface LiveJob {
  name: string;
  stage: string;
  execKind: string;
  groupLabel?: string;
  status: RunStatus;
  startedAt?: string;
  durationMs: number;
  exitCode: number;
  error?: string;
}

export interface LogLine {
  seq: number;
  job: string; // "" for run-level notices, "pipeline" for diagnostics
  data: string;
}

export interface LiveState {
  mode: string;
  status: RunStatus;
  startedAt?: string;
  durationMs: number;
  error?: string;
  configPath?: string;
  projectPath?: string;
  commit?: string;
  branch?: string;
  hasMatrix: boolean;
  hasDetached: boolean;
  /** Planned job order from run_started, when present. */
  order: string[];
  jobs: LiveJob[];
  log: LogLine[];
  finished: boolean;
}

export function newLiveState(): LiveState {
  return {
    mode: '',
    status: 'running',
    durationMs: 0,
    hasMatrix: false,
    hasDetached: false,
    order: [],
    jobs: [],
    log: [],
    finished: false,
  };
}

// applyEvent folds one wire event into the live state in place. Jobs are
// upserted by name so the replay (which re-sends job_started/finished) is
// idempotent. Returns the same state for convenience.
export function applyEvent(s: LiveState, e: WireEvent): LiveState {
  switch (e.type) {
    case 'run_started':
      if (e.mode) s.mode = e.mode;
      s.startedAt = e.time;
      s.configPath = e.configPath;
      s.projectPath = e.projectPath;
      s.commit = e.commit;
      s.branch = e.branch;
      s.hasMatrix = !!e.hasMatrix;
      s.hasDetached = !!e.hasDetached;
      if (e.order) s.order = e.order;
      break;
    case 'job_started': {
      const j = upsertJob(s, e.job ?? '');
      if (e.stage) j.stage = e.stage;
      if (e.exec) j.execKind = e.exec;
      j.startedAt = e.time;
      j.status = 'running';
      break;
    }
    case 'job_finished': {
      const j = upsertJob(s, e.job ?? '');
      if (e.stage) j.stage = e.stage;
      j.status = e.err ? 'failed' : 'passed';
      j.exitCode = e.exitCode ?? 0;
      j.durationMs = e.durationMs ?? 0;
      j.error = e.err;
      break;
    }
    case 'log_line':
      if (e.data) s.log.push({ seq: e.seq, job: e.job ?? '', data: e.data });
      break;
    case 'diagnostic':
      if (e.data) s.log.push({ seq: e.seq, job: 'pipeline', data: e.data });
      break;
    case 'run_finished':
      s.status = e.err ? 'failed' : 'passed';
      if (e.durationMs) s.durationMs = e.durationMs;
      s.error = e.err;
      s.finished = true;
      break;
    // group_started / group_finished carry barrier metadata the DAG slice may
    // use later; the pipeline view derives structure from /api/config instead.
  }
  return s;
}

function upsertJob(s: LiveState, name: string): LiveJob {
  let j = s.jobs.find((x) => x.name === name);
  if (!j) {
    j = { name, stage: '', execKind: '', status: 'running', durationMs: 0, exitCode: 0 };
    s.jobs.push(j);
  }
  return j;
}
