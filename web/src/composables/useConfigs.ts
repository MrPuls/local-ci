import { computed, ref, shallowRef } from 'vue';
import { listConfigs, selectConfig } from '@/lib/api';
import { useConfig } from '@/composables/useConfig';
import type { ConfigList } from '@/lib/types';

// Config-file discovery + selection (GET /api/configs, POST /api/configs/select).
// Shared singleton: the boot-time selector modal, the TopBar FILE chip and the
// editor view all look at the same list. `promptOnBoot` fires the selector once
// per page load — by design even with a single file, so the user always knows
// which config the session operates on.

const list = shallowRef<ConfigList | null>(null);
const loading = ref(false);
const error = ref<string | null>(null);
const selectorOpen = ref(false);
let prompted = false;

export function useConfigs() {
  const files = computed(() => list.value?.configs ?? []);
  const dir = computed(() => list.value?.dir ?? '');
  const active = computed(() => files.value.find((f) => f.active) ?? null);

  async function refresh(): Promise<void> {
    loading.value = true;
    error.value = null;
    try {
      list.value = await listConfigs();
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e);
    } finally {
      loading.value = false;
    }
  }

  /** Open the selector on app boot, once per page load. */
  async function promptOnBoot(): Promise<void> {
    if (prompted) return;
    prompted = true;
    await refresh();
    if (!error.value) selectorOpen.value = true;
  }

  /** Make `name` the active config and refresh the shared pipeline graph. */
  async function select(name: string): Promise<void> {
    const { config } = useConfig();
    config.value = await selectConfig(name); // the server returns the new graph
    await refresh();
  }

  return {
    files,
    dir,
    active,
    loading,
    error,
    selectorOpen,
    refresh,
    promptOnBoot,
    select,
  };
}
