<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import Icon from './Icon.vue';
import StatusTag from './StatusTag.vue';
import type { LogLine } from '@/lib/events';
import type { UiStatus } from '@/lib/types';

const props = defineProps<{
  lines: LogLine[];
  openJobs: string[];
  activeJob: string | null;
  jobStatus: Record<string, UiStatus>;
  minimized: boolean;
}>();
const emit = defineEmits<{
  (e: 'focus', job: string): void;
  (e: 'close', job: string): void;
  (e: 'minimize'): void;
  (e: 'restore'): void;
}>();

const feedEl = ref<HTMLElement | null>(null);

// Backend log lines are raw chunks, not pre-split rows. Join all chunks for the
// active job, then split for per-line display + a light severity heuristic
// (there is no level metadata on the wire).
const activeText = computed(() =>
  props.activeJob === null
    ? ''
    : props.lines
        .filter((l) => l.job === props.activeJob)
        .map((l) => l.data)
        .join(''),
);

interface DisplayLine {
  text: string;
  cls: string;
}

const displayLines = computed<DisplayLine[]>(() =>
  activeText.value
    .replace(/\n$/, '')
    .split('\n')
    .map((text) => ({ text, cls: classify(text) })),
);

function classify(line: string): string {
  if (/\b(error|fail(ed|ure)?|panic|fatal)\b/i.test(line)) return 'error';
  if (/^\s*[$+#>]/.test(line)) return 'accent';
  return '';
}

const status = (job: string): UiStatus => props.jobStatus[job] ?? 'idle';
const isStreaming = computed(() => props.activeJob !== null && status(props.activeJob) === 'running');
const hasStreams = computed(() => props.openJobs.length > 0);
const label = (job: string) => job || 'PIPELINE';

// Tail: keep the feed pinned to the newest output while a stream is live.
watch(
  () => displayLines.value.length,
  async () => {
    if (!isStreaming.value) return;
    await nextTick();
    if (feedEl.value) feedEl.value.scrollTop = feedEl.value.scrollHeight;
  },
);
</script>

<template>
  <!-- Collapsed: a thin restore bar -->
  <section
    v-if="minimized"
    class="panel log-min"
    data-test-id="log-feed-min"
    title="RESTORE_LOG_FEED"
    @click="emit('restore')"
  >
    <span class="panel-hd" style="border-bottom: none; padding: 0; margin: 0"><span>LOG_FEED</span></span>
    <span class="dim"
      >[{{ openJobs.length }} STREAM{{ openJobs.length === 1 ? '' : 'S' }} ATTACHED]</span
    >
    <span class="line"></span>
    <button
      class="log-ctl"
      aria-label="Restore log feed"
      @click.stop="emit('restore')"
    >
      <Icon name="chevron-up" />
    </button>
  </section>

  <section
    v-else
    class="panel"
    style="display: flex; flex-direction: column; gap: 0.6rem"
    data-test-id="log-feed"
  >
    <div style="display: flex; align-items: center; gap: 0.8rem; flex-wrap: wrap">
      <span class="panel-hd" style="border-bottom: none; padding: 0; margin: 0"><span>LOG_FEED</span></span>
      <div class="tabs" style="flex: 1; border-bottom: none; flex-wrap: wrap; gap: 0.4rem">
        <span
          v-for="job in openJobs"
          :key="job"
          class="log-tab"
          :class="{ active: activeJob === job }"
          :data-test-id="`log-tab-${job}`"
        >
          <button class="log-tab-main" @click="emit('focus', job)">
            <StatusTag :status="status(job)" compact />&nbsp;{{ label(job) }}
          </button>
          <button
            class="log-tab-x"
            :aria-label="`Detach ${label(job)} log`"
            title="DETACH_STREAM"
            @click="emit('close', job)"
          >
            <Icon name="cross" />
          </button>
        </span>
      </div>
      <span v-if="isStreaming" class="accent glow-strong blink"><Icon name="dot" glow /> STREAMING_</span>
      <button class="log-ctl" aria-label="Minimize log feed" title="MINIMIZE_LOG_FEED" @click="emit('minimize')">
        <Icon name="chevron-down" />
      </button>
    </div>

    <div
      ref="feedEl"
      class="log-feed"
      style="border-top: 2px solid var(--term-dim); padding-top: 8px"
      data-test-id="log-feed-body"
    >
      <div v-if="!hasStreams" class="log-empty">
        <div class="dim" style="font-size: 1.2rem">&gt; NO_LOG_STREAMS_ATTACHED</div>
        <div class="dim" style="margin-top: 6px">
          SELECT A JOB IN THE GRAPH → <span class="accent">[ <Icon name="logs" /> CHECK_LOGS ]</span> TO TAIL ITS OUTPUT_
        </div>
        <div class="dim blink" style="margin-top: 6px">_</div>
      </div>

      <template v-else>
        <div v-for="(l, i) in displayLines" :key="i" :class="l.cls">{{ l.text || ' ' }}</div>
        <div v-if="isStreaming"><span class="log-cursor"></span></div>
      </template>
    </div>
  </section>
</template>
