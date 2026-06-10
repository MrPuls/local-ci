import { computed, ref } from 'vue';
import { getRawConfig, saveRawConfig } from '@/lib/api';
import { useConfig } from '@/composables/useConfig';

// The YAML editor's buffer (GET/PUT /api/config/raw). Module singleton so an
// in-progress edit survives navigating between views; only an explicit load
// or save replaces it.

const text = ref('');
const savedText = ref('');
const loadedPath = ref<string | null>(null); // config path the buffer came from
const loading = ref(false);
const saving = ref(false);
const error = ref<string | null>(null);
const validation = ref<{ valid: boolean; errors: string[] } | null>(null);

export function useConfigRaw() {
  const dirty = computed(() => text.value !== savedText.value);
  const { config, refresh: refreshGraph } = useConfig();

  // The active config changed (selector) after the buffer was loaded from a
  // different file. The view offers an explicit reload instead of silently
  // discarding edits.
  const stale = computed(
    () => loadedPath.value !== null && !!config.value?.path && config.value.path !== loadedPath.value,
  );

  async function load(): Promise<void> {
    loading.value = true;
    error.value = null;
    try {
      const raw = await getRawConfig();
      text.value = raw;
      savedText.value = raw;
      loadedPath.value = config.value?.path ?? loadedPath.value;
      validation.value = null;
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e);
    } finally {
      loading.value = false;
    }
  }

  /** Load once (or after a source switch); keeps an existing dirty buffer. */
  async function ensureLoaded(): Promise<void> {
    if (loadedPath.value === null) await load();
    else if (stale.value && !dirty.value) await load();
  }

  async function save(): Promise<{ valid: boolean; errors: string[] }> {
    saving.value = true;
    error.value = null;
    try {
      const res = await saveRawConfig(text.value);
      savedText.value = text.value;
      loadedPath.value = res.path;
      validation.value = { valid: res.valid, errors: res.errors ?? [] };
      await refreshGraph(); // the visualizer + chrome follow the saved file
      return validation.value;
    } finally {
      saving.value = false;
    }
  }

  return { text, dirty, stale, loading, saving, error, validation, load, ensureLoaded, save };
}
