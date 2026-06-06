<script setup lang="ts">
import { computed, ref } from 'vue';
import StatusChip from './StatusChip.vue';
import JobConfigPanel from './JobConfigPanel.vue';
import TimingPanel from './TimingPanel.vue';
import { useNow } from '@/composables/useNow';
import { statusMeta } from '@/lib/status';
import { fmtSeconds } from '@/lib/format';
import type { PipelineNode } from '@/lib/pipeline';

const props = defineProps<{
  node: PipelineNode | null;
  nodes: PipelineNode[];
  logAttached: boolean;
}>();
const emit = defineEmits<{ (e: 'close'): void; (e: 'check-logs', name: string): void }>();

type Tab = 'config' | 'timing';
const tab = ref<Tab>('config');
const TABS: { id: Tab; label: string }[] = [
  { id: 'config', label: 'CONFIG' },
  { id: 'timing', label: 'TIMING' },
];

const now = useNow();
const m = computed(() => statusMeta(props.node?.status ?? 'idle'));
const fx = computed(() => (m.value.motion === 'pulse' ? 'soft-pulse' : ''));

const elapsedText = computed(() => {
  const n = props.node;
  if (!n || !n.startedAt || n.status !== 'running') return null;
  const e = (now.value - new Date(n.startedAt).getTime()) / 1000;
  return e >= 0 ? `${e.toFixed(1)}S_ELAPSED` : null;
});
</script>

<template>
  <section
    class="panel"
    style="display: flex; flex-direction: column; gap: 0.8rem; min-width: 0"
    data-test-id="inspector"
  >
    <div class="panel-hd">
      <span>JOB_INSPECTOR</span>
      <button
        class="inspector-close"
        data-test-id="inspector-close"
        title="CLOSE_INSPECTOR"
        aria-label="Close inspector"
        @click="emit('close')"
      >
        [ X ]
      </button>
    </div>

    <template v-if="node">
      <!-- identity -->
      <div>
        <div style="display: flex; align-items: baseline; gap: 0.6rem; flex-wrap: wrap">
          <span :class="['accent', 'glow-strong', fx]" style="font-size: 1.6rem">{{ m.glyph }}</span>
          <span class="glow-strong" style="font-size: 1.55rem; letter-spacing: 2px" data-test-id="inspector-job-name">{{
            node.name
          }}</span>
        </div>
        <div class="dim" style="margin-top: 4px">
          STAGE={{ (node.stage || '—').toUpperCase() }} · IMG={{ node.image || '—' }}
        </div>
        <div style="margin-top: 6px; display: flex; gap: 0.6rem; flex-wrap: wrap; align-items: center">
          <StatusChip :status="node.status" />
          <span v-if="elapsedText" class="chip accent">[ ⏱ ] {{ elapsedText }}</span>
          <span v-else-if="node.ran && node.durationMs > 0" class="chip dim"
            >[ ⏱ ] {{ fmtSeconds(node.durationMs) }}S</span
          >
          <button
            class="btn btn-accent btn-sq"
            style="margin-left: auto"
            data-test-id="check-logs"
            :disabled="!node.ran"
            title="ATTACH_LOG_STREAM_TO_FEED"
            @click="emit('check-logs', node.name)"
          >
            {{ logAttached ? '⊳ TAILING' : '⊳ CHECK_LOGS' }}
          </button>
        </div>
      </div>

      <!-- tabs -->
      <div class="tabs">
        <button
          v-for="t in TABS"
          :key="t.id"
          class="tab"
          :class="{ active: tab === t.id }"
          :data-test-id="`inspector-tab-${t.id}`"
          @click="tab = t.id"
        >
          {{ t.label }}
        </button>
      </div>

      <!-- body -->
      <div style="min-height: 280px">
        <JobConfigPanel v-if="tab === 'config'" :node="node" />
        <TimingPanel v-else :nodes="nodes" :focused-name="node.name" />
      </div>
    </template>

    <div v-else class="empty" data-test-id="inspector-empty">
      <div class="dim" style="font-size: 1.4rem">&gt; NO_JOB_SELECTED</div>
      <div class="dim" style="margin-top: 6px">SELECT A JOB IN THE GRAPH TO INSPECT_</div>
      <div class="dim blink" style="margin-top: 6px">_</div>
    </div>
  </section>
</template>
