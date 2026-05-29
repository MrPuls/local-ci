// These mirror the server's JSON exactly. Sources of truth:
//   internal/server/server.go   (runJSON, jobJSON)
//   internal/server/config.go   (configGraph, graphJob)
//   internal/engine/wire.go     (WireEvent)

export type RunStatus = 'running' | 'passed' | 'failed';

export interface Job {
  name: string;
  stage: string;
  execKind: string; // standalone | concurrent | detached
  groupLabel?: string;
  status: RunStatus;
  startedAt?: string; // RFC3339
  finishedAt?: string;
  durationMs: number;
  exitCode: number;
  error?: string;
}

export interface Run {
  id: string;
  projectPath: string;
  configPath: string;
  mode: string; // sequential | parallel | parallel-stages
  status: RunStatus;
  startedAt: string;
  finishedAt?: string;
  durationMs: number;
  error?: string;
  jobs?: Job[]; // present on GET /api/runs/{id}, absent in the list
}

export interface RunListResponse {
  runs: Run[];
}

// --- used by later slices (config DAG, live SSE) ---

export interface GraphJob {
  name: string;
  stage: string;
  image: string;
  parallel: boolean;
  variantCount: number;
}

export interface ConfigGraph {
  valid: boolean;
  errors?: string[];
  path?: string;
  stages?: string[];
  jobs?: GraphJob[];
  includes?: string[];
}

export interface WireEvent {
  seq: number;
  type: string;
  time: string;
  runId?: string;
  job?: string;
  stage?: string;
  exec?: string;
  groupId?: string;
  groupKind?: string;
  groupLabel?: string;
  mode?: string;
  hasMatrix?: boolean;
  hasDetached?: boolean;
  order?: string[];
  configPath?: string;
  projectPath?: string;
  exitCode?: number;
  durationMs?: number;
  err?: string;
  stream?: string;
  data?: string;
}
