import { computed, ref, shallowRef } from 'vue';
import {
  listRunsPage,
  deleteRun as apiDeleteRun,
  cleanupRuns as apiCleanupRuns,
} from '@/lib/api';
import type { Run } from '@/lib/types';

// Paginated run history (GET /api/runs) plus delete + cleanup. A per-call
// composable instance so each view owns its own loading/paging lifecycle.

const PAGE_SIZE = 25;

export function useRuns() {
  const runs = shallowRef<Run[]>([]);
  const total = ref(0);
  const offset = ref(0);
  const pageSize = ref(PAGE_SIZE);
  const loading = ref(false);
  const error = ref<string | null>(null);

  const hasPrev = computed(() => offset.value > 0);
  const hasNext = computed(() => offset.value + pageSize.value < total.value);

  async function load(): Promise<void> {
    loading.value = true;
    error.value = null;
    try {
      const page = await listRunsPage(pageSize.value, offset.value);
      runs.value = page.runs;
      total.value = page.total;
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e);
    } finally {
      loading.value = false;
    }
  }

  function refresh(): Promise<void> {
    offset.value = 0;
    return load();
  }
  function nextPage(): Promise<void> {
    if (!hasNext.value) return Promise.resolve();
    offset.value += pageSize.value;
    return load();
  }
  function prevPage(): Promise<void> {
    if (!hasPrev.value) return Promise.resolve();
    offset.value = Math.max(0, offset.value - pageSize.value);
    return load();
  }

  async function remove(id: string): Promise<void> {
    await apiDeleteRun(id);
    // Stepping off a now-empty trailing page keeps the view from going blank.
    if (runs.value.length === 1 && offset.value > 0) {
      offset.value = Math.max(0, offset.value - pageSize.value);
    }
    await load();
  }

  async function cleanup(keep: number): Promise<number> {
    const n = await apiCleanupRuns(keep, true);
    offset.value = 0;
    await load();
    return n;
  }

  return {
    runs,
    total,
    offset,
    pageSize,
    loading,
    error,
    hasPrev,
    hasNext,
    refresh,
    load,
    nextPage,
    prevPage,
    remove,
    cleanup,
  };
}
