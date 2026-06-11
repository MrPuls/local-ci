<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import Icon from './Icon.vue';
import StatusTag from './StatusTag.vue';
import { ansiLine, newSgrState, type AnsiSpan } from '@/lib/ansi';
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
const query = ref('');

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

interface DisplaySpan extends AnsiSpan {
  match?: boolean;
}

interface DisplayLine {
  spans: DisplaySpan[];
  cls: string;
}

// ANSI-render every line, carrying SGR state across lines (a color set on one
// line styles the next until reset — terminal semantics).
const renderedLines = computed<DisplayLine[]>(() => {
  const state = newSgrState();
  return activeText.value
    .replace(/\n$/, '')
    .split('\n')
    .map((text) => {
      const spans = ansiLine(text, state);
      return { spans, cls: classify(plain(spans)) };
    });
});

const plain = (spans: AnsiSpan[]): string => spans.map((s) => s.text).join('');

// Search: filter to matching lines and highlight the matched substrings.
const displayLines = computed<DisplayLine[]>(() => {
  const q = query.value.toLowerCase();
  if (!q) return renderedLines.value;
  return renderedLines.value
    .filter((l) => plain(l.spans).toLowerCase().includes(q))
    .map((l) => ({ ...l, spans: l.spans.flatMap((s) => markMatches(s, q)) }));
});

const matchCount = computed(() => {
  const q = query.value.toLowerCase();
  if (!q) return 0;
  return renderedLines.value.filter((l) => plain(l.spans).toLowerCase().includes(q)).length;
});

// Splits a span so the query substring gets its own marked span.
function markMatches(span: DisplaySpan, q: string): DisplaySpan[] {
  const lower = span.text.toLowerCase();
  const out: DisplaySpan[] = [];
  let pos = 0;
  for (let hit = lower.indexOf(q, pos); hit >= 0; hit = lower.indexOf(q, pos)) {
    if (hit > pos) out.push({ ...span, text: span.text.slice(pos, hit) });
    out.push({ ...span, text: span.text.slice(hit, hit + q.length), match: true });
    pos = hit + q.length;
  }
  if (pos < span.text.length) out.push({ ...span, text: span.text.slice(pos) });
  return out;
}

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
      <span class="search-box" :class="{ active: query }">
        <Icon name="search" />
        <input
          v-model="query"
          type="text"
          class="search-input"
          data-test-id="log-search"
          placeholder="GREP_"
          spellcheck="false"
        />
        <span v-if="query" class="dim" data-test-id="log-search-count">{{ matchCount }}</span>
        <button v-if="query" class="log-tab-x" aria-label="Clear search" @click="query = ''">
          <Icon name="cross" />
        </button>
      </span>
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
        <div v-if="query && displayLines.length === 0" class="dim">
          &gt; NO_MATCHES FOR "{{ query }}"_
        </div>
        <div v-for="(l, i) in displayLines" :key="i" :class="l.cls">
          <template v-if="l.spans.length === 0">&nbsp;</template>
          <span
            v-for="(s, j) in l.spans"
            :key="j"
            :class="[...s.classes, s.match ? 'log-match' : '']"
            >{{ s.text }}</span
          >
        </div>
        <div v-if="isStreaming && !query"><span class="log-cursor"></span></div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.search-box {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 2px solid var(--term-dim);
  padding: 0.05rem 0.4rem;
  color: var(--term-dim);
}
.search-box.active {
  border-color: var(--term-accent);
  color: var(--term-accent);
}
.search-input {
  background: transparent;
  border: none;
  outline: none;
  color: var(--term-fg);
  font-family: inherit;
  font-size: 0.95rem;
  letter-spacing: 1px;
  width: 9rem;
  text-transform: none; /* search terms are case-preserved (matching is case-insensitive) */
}
</style>
