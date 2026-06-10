import { ref } from 'vue';
import { getHealth } from '@/lib/api';

// Backend version + reachability for the top bar. Shared singleton: fetched
// once, reused everywhere.

const version = ref<string>('');
const online = ref<boolean | null>(null); // null = not yet checked
let inflight: Promise<void> | null = null;

export function useHealth() {
  async function refresh(): Promise<void> {
    if (inflight) return inflight;
    inflight = (async () => {
      try {
        const h = await getHealth();
        version.value = h.version;
        online.value = true;
      } catch {
        online.value = false;
      } finally {
        inflight = null;
      }
    })();
    return inflight;
  }

  return { version, online, refresh };
}
