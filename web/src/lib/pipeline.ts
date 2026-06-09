import type { ConfigGraph, RunStatus, UiStatus } from './types';

// mergePipeline overlays a run's job states onto the configured pipeline graph.
// The graph structure (stages, jobs, images, matrix fan-out) comes from
// GET /api/config; the per-job status/duration comes from a run (live SSE state
// or a persisted run detail). This is what replaces the design's hardcoded
// PIPELINE + RUN_STATES.

/** The minimal run-job shape both LiveJob (SSE) and Job (REST) satisfy. */
export interface RunJobLike {
  name: string;
  stage: string;
  status: RunStatus;
  startedAt?: string;
  durationMs: number;
  exitCode?: number;
  error?: string;
  execKind?: string;
}

/** Run context for the overlay. `null` means "no run" → all nodes idle. */
export interface RunContext {
  active: boolean; // a run is in progress
  finished: boolean; // a run exists and has finished
  jobs: RunJobLike[];
}

export interface PipelineNode {
  name: string;
  stage: string;
  image?: string;
  parallel: boolean;
  variantCount: number;
  status: UiStatus;
  startedAt?: string;
  durationMs: number;
  exitCode?: number;
  error?: string;
  execKind?: string;
  configured: boolean; // present in the config graph
  ran: boolean; // present in the run
}

export interface PipelineStage {
  name: string;
  nodes: PipelineNode[];
}

// A run job is a matrix variant of a config job when its name is the config
// name followed by a fan-out separator. The engine emits "<job>_<key>.<value>"
// (e.g. "test:matrix_GO.1.23"); we also accept the " ..." / "[...]" forms the
// GitLab-style designs used. Only consulted for jobs the config marks as
// fanning out (variantCount > 1) to avoid matching unrelated jobs that merely
// share a prefix.
function isVariantOf(runName: string, configName: string): boolean {
  if (runName === configName) return true;
  if (!runName.startsWith(configName)) return false;
  const next = runName.charAt(configName.length);
  return next === '_' || next === ' ' || next === '[';
}

export function mergePipeline(
  config: ConfigGraph | null,
  run: RunContext | null,
): PipelineStage[] {
  const configJobs = config?.jobs ?? [];
  const runJobs = run?.jobs ?? [];

  // Stage order: configured stages first, then any stage seen only in the run.
  const stageOrder: string[] = [...(config?.stages ?? [])];
  for (const j of [...configJobs, ...runJobs]) {
    if (j.stage && !stageOrder.includes(j.stage)) stageOrder.push(j.stage);
  }

  const runByName = new Map(runJobs.map((j) => [j.name, j]));
  const used = new Set<string>();

  const placeholderStatus = (): UiStatus =>
    run?.finished ? 'skipped' : run?.active ? 'queued' : 'idle';

  const fromRun = (r: RunJobLike, c?: (typeof configJobs)[number]): PipelineNode => ({
    name: r.name,
    stage: r.stage || c?.stage || '',
    image: c?.image,
    parallel: c?.parallel ?? r.execKind === 'detached',
    variantCount: c?.variantCount ?? 1,
    status: r.status,
    startedAt: r.startedAt,
    durationMs: r.durationMs,
    exitCode: r.exitCode,
    error: r.error,
    execKind: r.execKind,
    configured: !!c,
    ran: true,
  });

  const fromConfig = (c: (typeof configJobs)[number], status: UiStatus): PipelineNode => ({
    name: c.name,
    stage: c.stage,
    image: c.image,
    parallel: c.parallel,
    variantCount: c.variantCount,
    status,
    durationMs: 0,
    configured: true,
    ran: false,
  });

  const stages: PipelineStage[] = [];
  for (const stage of stageOrder) {
    const nodes: PipelineNode[] = [];

    // 1. Configured jobs in this stage, in config order.
    for (const c of configJobs.filter((j) => j.stage === stage)) {
      const exact = runByName.get(c.name);
      if (exact) {
        used.add(c.name);
        nodes.push(fromRun(exact, c));
        continue;
      }
      if (c.variantCount > 1) {
        const variants = runJobs.filter((r) => !used.has(r.name) && isVariantOf(r.name, c.name));
        if (variants.length > 0) {
          for (const v of variants) {
            used.add(v.name);
            nodes.push(fromRun(v, c));
          }
          continue;
        }
      }
      nodes.push(fromConfig(c, placeholderStatus()));
    }

    // 2. Run-only jobs in this stage (e.g. a config that changed since the run).
    for (const r of runJobs.filter((j) => j.stage === stage && !used.has(j.name))) {
      used.add(r.name);
      nodes.push(fromRun(r, undefined));
    }

    if (nodes.length > 0) stages.push({ name: stage, nodes });
  }
  return stages;
}

export function allNodes(stages: PipelineStage[]): PipelineNode[] {
  return stages.flatMap((s) => s.nodes);
}

export type StatusCounts = Record<UiStatus, number>;

export function countByStatus(stages: PipelineStage[]): StatusCounts {
  const counts: StatusCounts = {
    passed: 0,
    failed: 0,
    running: 0,
    queued: 0,
    skipped: 0,
    idle: 0,
  };
  for (const n of allNodes(stages)) counts[n.status] += 1;
  return counts;
}

/** The between-stage edge state: failed if upstream failed and all downstream
 *  skipped; else running if any downstream running; else passed/queued/dim. */
export function edgeStatus(from: PipelineStage, to: PipelineStage): UiStatus {
  const fs = from.nodes.map((n) => n.status);
  const ts = to.nodes.map((n) => n.status);
  if (fs.some((s) => s === 'failed') && ts.every((s) => s === 'skipped')) return 'failed';
  if (ts.some((s) => s === 'running')) return 'running';
  if (ts.length > 0 && ts.every((s) => s === 'passed')) return 'passed';
  if (ts.some((s) => s === 'queued' || s === 'idle')) return 'queued';
  return 'skipped';
}
