<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import { useRunStatus } from '@/composables/useRunStatus';

// Bottom status ticker. The design's fixed "RUNTIME=ORBSTACK / DOCKER_x" line
// isn't available from the API, so we show what's real: run state, live
// per-status totals, and a wall clock.
const { summary } = useRunStatus();

const clock = ref(new Date().toLocaleTimeString([], { hour12: false }));
let timer: ReturnType<typeof setInterval> | undefined;

onMounted(() => {
  timer = setInterval(() => {
    clock.value = new Date().toLocaleTimeString([], { hour12: false });
  }, 1000);
});
onUnmounted(() => timer && clearInterval(timer));
</script>

<template>
  <footer
    style="
      display: flex;
      align-items: center;
      gap: 1.5rem;
      flex-wrap: wrap;
      border-top: 2px solid var(--term-dim);
      padding-top: 0.6rem;
      font-size: 1rem;
    "
    data-test-id="status-bar"
  >
    <span class="dim">SYS:</span>
    <span>STATE=<span class="accent">{{ summary.label }}</span></span>
    <span style="flex: 1"></span>
    <span class="accent" data-test-id="count-passed">{{ summary.counts.passed }}_OK</span>
    <span class="error" data-test-id="count-failed">{{ summary.counts.failed }}_ERR</span>
    <span class="accent" data-test-id="count-running">{{ summary.counts.running }}_RUN</span>
    <span class="dim" data-test-id="count-queued">{{ summary.counts.queued }}_QUE</span>
    <span class="dim" data-test-id="count-skipped">{{ summary.counts.skipped }}_SKP</span>
    <span class="dim" data-test-id="count-idle">{{ summary.counts.idle }}_IDL</span>
    <span class="dim">|</span>
    <span class="dim" data-test-id="clock">{{ clock }}</span>
    <span class="blink accent">_</span>
  </footer>
</template>
