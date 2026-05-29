import type { Run, RunListResponse } from './types';

// Base is empty in browser dev (same-origin via the Vite proxy) and in Tauri;
// the proxy/shell supplies the bearer token, so it never lives here.
const BASE = import.meta.env.VITE_LCI_BASE ?? '';

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}: ${await safeText(res)}`);
  }
  return (await res.json()) as T;
}

async function safeText(res: Response): Promise<string> {
  try {
    return await res.text();
  } catch {
    return '';
  }
}

export async function listRuns(): Promise<Run[]> {
  const data = await getJSON<RunListResponse>('/api/runs?all=true');
  return data.runs ?? [];
}

export function getRun(id: string): Promise<Run> {
  return getJSON<Run>(`/api/runs/${encodeURIComponent(id)}`);
}

export async function jobLog(runId: string, job: string): Promise<string> {
  const url = `${BASE}/api/runs/${encodeURIComponent(runId)}/log?job=${encodeURIComponent(job)}`;
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}: ${await safeText(res)}`);
  }
  return res.text();
}

export interface TriggerRequest {
  mode?: string; // sequential | parallel | parallel-stages
  jobs?: string[];
  stages?: string[];
  env?: string[];
}

export async function triggerRun(req: TriggerRequest = {}): Promise<string> {
  const res = await fetch(`${BASE}/api/runs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}: ${await safeText(res)}`);
  }
  const data = (await res.json()) as { id: string };
  return data.id;
}

export async function cancelRun(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/runs/${encodeURIComponent(id)}/cancel`, { method: 'POST' });
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}: ${await safeText(res)}`);
  }
}
