import { ref } from 'vue';
import { getSystem } from '@/lib/api';
import type { DbInfo, EngineStatus } from '@/lib/types';

// Container-engine status (Docker/OrbStack ready?) + history DB location/size,
// polled from GET /api/system. Shared singleton: one poll loop feeds the top-bar
// engine chip and the history view's DB readout.

const engine = ref<EngineStatus | null>(null);
const db = ref<DbInfo | null>(null);
let inflight: Promise<void> | null = null;
let timer: ReturnType<typeof setInterval> | undefined;

const POLL_MS = 8000;

function refresh(): Promise<void> {
  if (inflight) return inflight;
  inflight = (async () => {
    try {
      const s = await getSystem();
      engine.value = s.engine;
      db.value = s.db;
    } catch {
      // Can't reach the server → can't accept jobs; reflect that, keep DB info.
      engine.value = engine.value ? { ...engine.value, ready: false } : null;
    } finally {
      inflight = null;
    }
  })();
  return inflight;
}

export function useSystem() {
  if (!timer) {
    refresh();
    timer = setInterval(refresh, POLL_MS);
  }
  return { engine, db, refresh };
}
