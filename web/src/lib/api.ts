import type {
  ConfigGraph,
  Health,
  Run,
  RunListResponse,
  RunMode,
  SystemInfo,
} from './types';

// Base is empty in browser dev (same-origin via the Vite proxy) and in Tauri;
// the proxy/shell supplies the bearer token, so it never lives here.
const BASE = import.meta.env.VITE_LCI_BASE ?? '';

async function safeText(res: Response): Promise<string> {
  try {
    return await res.text();
  } catch {
    return '';
  }
}

/** Thrown for any non-2xx response; carries the HTTP status for callers. */
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(BASE + path, init);
  if (!res.ok) {
    throw new ApiError(res.status, `${res.status} ${res.statusText}: ${await safeText(res)}`);
  }
  return res;
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await request(path, { headers: { Accept: 'application/json' } });
  return (await res.json()) as T;
}

export function getHealth(): Promise<Health> {
  return getJSON<Health>('/api/health');
}

export function getConfig(): Promise<ConfigGraph> {
  return getJSON<ConfigGraph>('/api/config');
}

export function getSystem(): Promise<SystemInfo> {
  return getJSON<SystemInfo>('/api/system');
}

export interface RunPage {
  runs: Run[];
  total: number;
}

/** One page of run history (newest first), with the total count for paging. */
export async function listRunsPage(limit = 25, offset = 0): Promise<RunPage> {
  const data = await getJSON<RunListResponse>(
    `/api/runs?all=true&limit=${limit}&offset=${offset}`,
  );
  return { runs: data.runs ?? [], total: data.total ?? 0 };
}

/** Delete a single finished run (row, jobs, and log files). */
export async function deleteRun(id: string): Promise<void> {
  await request(`/api/runs/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

/** Delete all but the `keep` most recent runs. Returns how many were removed. */
export async function cleanupRuns(keep: number, all = true): Promise<number> {
  const res = await request('/api/runs/cleanup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keep, all }),
  });
  const data = (await res.json()) as { deleted: number };
  return data.deleted ?? 0;
}

export function getRun(id: string): Promise<Run> {
  return getJSON<Run>(`/api/runs/${encodeURIComponent(id)}`);
}

export async function jobLog(runId: string, job: string): Promise<string> {
  const res = await request(
    `/api/runs/${encodeURIComponent(runId)}/log?job=${encodeURIComponent(job)}`,
  );
  return res.text();
}

export interface TriggerRequest {
  mode?: RunMode;
  jobs?: string[];
  stages?: string[];
  env?: string[];
}

export async function triggerRun(req: TriggerRequest = {}): Promise<string> {
  const res = await request('/api/runs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  const data = (await res.json()) as { id: string };
  return data.id;
}

export function cancelRun(id: string): Promise<Response> {
  return request(`/api/runs/${encodeURIComponent(id)}/cancel`, { method: 'POST' });
}

/** SSE URL for a run's live + replayed event stream. Same-origin/relative so
 *  the dev proxy (or Tauri shell) attaches the bearer token. */
export function eventStreamUrl(runId: string): string {
  return `${BASE}/api/runs/${encodeURIComponent(runId)}/events`;
}
