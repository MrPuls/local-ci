import { ref, shallowRef } from 'vue';
import { listRuns } from '@/lib/api';
import type { Run } from '@/lib/types';

// Run history (GET /api/runs). Used by the history browser. Kept as a per-call
// composable instance (not a singleton) so each view owns its own loading
// lifecycle.

export function useRuns() {
  const runs = shallowRef<Run[]>([]);
  const loading = ref(false);
  const error = ref<string | null>(null);

  async function refresh(limit = 50): Promise<void> {
    loading.value = true;
    error.value = null;
    try {
      runs.value = await listRuns(limit);
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e);
    } finally {
      loading.value = false;
    }
  }

  return { runs, loading, error, refresh };
}
