// These mirror the server's JSON exactly. Sources of truth:
//   internal/server/server.go   (runJSON, jobJSON)
//   internal/server/config.go   (configGraph, graphJob)
//   internal/engine/wire.go     (WireEvent)

// The statuses the backend actually persists/streams for a run or job.
export type RunStatus = 'running' | 'passed' | 'failed';

// The wider set the UI renders. 'idle' (configured but no run yet), 'queued'
// (awaiting prior stage in a live run) and 'skipped' are presentation-only
// states the pipeline view derives — the API never emits them.
export type UiStatus = RunStatus | 'idle' | 'queued' | 'skipped';

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
  commit?: string; // HEAD SHA at run start (absent outside a git repo)
  branch?: string;
  jobs?: Job[]; // present on GET /api/runs/{id}, absent in the list
}

export interface RunListResponse {
  runs: Run[];
  total: number;
}

export interface Health {
  status: string;
  version: string;
}

// GET /api/system — container-engine status + history DB location/size.
// Mirrors internal/server/server.go (systemJSON) and internal/docker (Status).
export interface EngineStatus {
  ready: boolean;
  provider: string; // Docker | Docker Desktop | OrbStack | …
  version: string;
}

export interface DbInfo {
  path: string;
  sizeBytes: number;
}

export interface SystemInfo {
  engine: EngineStatus;
  db: DbInfo;
}

export interface GraphService {
  alias: string;
  image: string;
}

export interface GraphJob {
  name: string;
  stage: string;
  image: string;
  parallel: boolean;
  variantCount: number; // >1 when the job fans out via matrix
  timeout?: string; // per-attempt limit, e.g. "10m"
  retry?: number; // extra attempts on failure
  needs?: string[]; // DAG dependencies
  services?: GraphService[]; // sidecar containers
  artifacts?: string[]; // produced paths
}

export interface ConfigGraph {
  valid: boolean;
  errors?: string[];
  path?: string;
  stages?: string[];
  jobs?: GraphJob[];
  includes?: string[];
}

// GET /api/configs — config files discovered in the project directory.
// Mirrors internal/server/config.go (configListJSON, configFileJSON).
export interface ConfigFile {
  name: string;
  path: string;
  active: boolean;
  exists: boolean;
}

export interface ConfigList {
  dir: string;
  configs: ConfigFile[];
}

// PUT /api/config/raw — save result. Invalid YAML still saves; the errors
// travel back so the editor can surface them.
export interface SaveConfigResult {
  saved: boolean;
  path: string;
  valid: boolean;
  errors?: string[];
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
  commit?: string;
  branch?: string;
  exitCode?: number;
  durationMs?: number;
  err?: string;
  stream?: string;
  data?: string;
}

export type RunMode = 'sequential' | 'parallel' | 'parallel-stages';

// GET /api/jobs/stats — per-job trend rows across the recent-runs window.
// Mirrors internal/server/stats.go.
export interface JobSample {
  runId: string;
  status: string; // running | passed | failed
  durationMs: number;
}

export interface JobStat {
  name: string;
  samples: JobSample[]; // oldest first
  avgMs: number;
  maxMs: number;
  passRate: number; // 0..1 over finished samples
  flaky: boolean; // both passes and failures in the window
}

export interface JobStatsResponse {
  window: number;
  jobs: JobStat[];
}
