import { onUnmounted, reactive, ref } from 'vue';
import { eventStreamUrl } from '@/lib/api';
import { applyEvent, newLiveState, type LiveState } from '@/lib/events';
import type { WireEvent } from '@/lib/types';

// Drives a run's view from its SSE stream. The endpoint replays the whole
// event file from the start and then (if the run is still active) continues
// live, so this works identically for active and finished runs: connect, fold
// every event into LiveState, and close once run_finished arrives.

// Cap the in-memory log so a long/replayed run can't grow unbounded. The
// pipeline log feed tails recent output; full per-job logs come from the
// /log endpoint in the history view.
const MAX_LOG = 5000;

export function useLiveRun() {
  const state = reactive<LiveState>(newLiveState());
  const connected = ref(false);
  const error = ref<string | null>(null);

  let es: EventSource | null = null;

  function disconnect(): void {
    if (es) {
      es.close();
      es = null;
    }
    connected.value = false;
  }

  function connect(id: string): void {
    disconnect();
    Object.assign(state, newLiveState());
    error.value = null;

    es = new EventSource(eventStreamUrl(id));

    es.onopen = () => {
      connected.value = true;
    };

    es.onmessage = (ev: MessageEvent<string>) => {
      let wire: WireEvent;
      try {
        wire = JSON.parse(ev.data) as WireEvent;
      } catch {
        return; // ignore non-JSON keepalives
      }
      applyEvent(state, wire);
      if (state.log.length > MAX_LOG) {
        state.log.splice(0, state.log.length - MAX_LOG);
      }
      if (wire.type === 'run_finished') {
        disconnect(); // finished: don't let EventSource re-replay the file
      }
    };

    es.onerror = () => {
      // EventSource auto-reconnects on transient drops (readyState CONNECTING)
      // and the server resumes via Last-Event-ID. Treat only a terminal close
      // (e.g. 404 / auth) — before we ever saw events — as a surfaced error.
      connected.value = false;
      if (es && es.readyState === EventSource.CLOSED) {
        if (!state.startedAt && !state.finished) {
          error.value = 'event stream closed before any data (run not found or unauthorized)';
        }
        disconnect();
      }
    };
  }

  // Close the stream when the owning view unmounts (e.g. navigating to the
  // history list) so we don't leak an open EventSource.
  onUnmounted(disconnect);

  return { state, connected, error, connect, disconnect };
}
