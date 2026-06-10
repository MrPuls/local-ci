import { ref, shallowRef } from 'vue';
import { getConfig } from '@/lib/api';
import type { ConfigGraph } from '@/lib/types';

// The configured pipeline graph (GET /api/config). Shared singleton so the
// pipeline view and run-control share one source. Refresh re-fetches (the YAML
// may have changed on disk between runs).

const config = shallowRef<ConfigGraph | null>(null);
const loading = ref(false);
const error = ref<string | null>(null);

export function useConfig() {
  async function refresh(): Promise<void> {
    loading.value = true;
    error.value = null;
    try {
      config.value = await getConfig();
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e);
    } finally {
      loading.value = false;
    }
  }

  return { config, loading, error, refresh };
}
