<script setup lang="ts">
import { computed } from 'vue';
import Icon from './Icon.vue';
import { useNow } from '@/composables/useNow';
import { statusMeta, statusColor, statusGlow, barFillClass } from '@/lib/status';
import { fmtSeconds } from '@/lib/format';
import type { PipelineNode } from '@/lib/pipeline';

const props = defineProps<{ node: PipelineNode; focused?: boolean }>();
const emit = defineEmits<{ (e: 'focus'): void }>();

const now = useNow();
const m = computed(() => statusMeta(props.node.status));

// Elapsed for a running job ticks live off the shared clock; finished jobs show
// their recorded duration; not-yet-run jobs show nothing.
const timeText = computed(() => {
  const n = props.node;
  if (n.status === 'running') {
    if (!n.startedAt) return '..';
    const elapsed = (now.value - new Date(n.startedAt).getTime()) / 1000;
    return elapsed >= 0 ? `${elapsed.toFixed(1)}S` : '..';
  }
  if (n.status === 'passed' || n.status === 'failed') return `${fmtSeconds(n.durationMs)}S`;
  return '--';
});

const flags = computed(() => {
  const f: string[] = [];
  if (props.node.variantCount > 1) f.push(`[×${props.node.variantCount}]`);
  if (props.node.parallel || props.node.execKind === 'detached') f.push('[PARA]');
  return f;
});

// The focus ring keeps the status color when it's meaningful (failed/running/
// passed) but falls back to the accent highlight for idle/queued/skipped — whose
// status color is --term-dim, identical to the unfocused border, so without the
// fallback a tapped tile would show no change at all.
const cardStyle = computed(() => {
  if (!props.focused) {
    return { border: '2px solid var(--term-dim)', boxShadow: 'none' };
  }
  const color = statusColor(props.node.status);
  const glow = statusGlow(props.node.status);
  const ringColor = color === 'var(--term-dim)' ? 'var(--term-accent)' : color;
  const ringGlow = glow === 'transparent' ? 'var(--term-glow-accent)' : glow;
  return {
    border: `2px solid ${ringColor}`,
    boxShadow: `0 0 20px ${ringGlow}`,
  };
});

const showBar = computed(() => ['running', 'passed', 'failed'].includes(props.node.status));
</script>

<template>
  <div
    class="job-card"
    :style="cardStyle"
    :data-test-id="`job-card-${node.name}`"
    :data-status="node.status"
    role="button"
    tabindex="0"
    @click="emit('focus')"
    @keydown.enter="emit('focus')"
    @keydown.space.prevent="emit('focus')"
  >
    <div class="job-card-hd">
      <span :class="m.cls" data-test-id="job-glyph">
        <Icon :name="m.icon" :spin="m.motion === 'pulse'" glow />
      </span>
      <span class="glow-strong job-card-title" data-test-id="job-name">{{ node.name }}</span>
      <span class="dim" data-test-id="job-time">{{ timeText }}</span>
    </div>

    <div class="job-card-row">
      <span class="dim">IMG:</span>
      <span class="job-card-img">{{ node.image || '—' }}</span>
    </div>

    <div class="job-card-row" style="flex-wrap: wrap">
      <span class="dim">STAGE:</span><span class="alt">{{ node.stage || '—' }}</span>
      <span class="accent" v-for="f in flags" :key="f">{{ f }}</span>
    </div>

    <div v-if="showBar" style="margin-top: 6px">
      <div class="bar">
        <div
          class="bar-fill"
          :class="[barFillClass(node.status), node.status === 'running' ? 'soft-pulse' : '']"
          style="width: 100%"
        ></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.job-card {
  padding: 0.5rem 0.7rem;
  background: rgba(0, 0, 0, 0.45);
  cursor: pointer;
  width: 260px;
}
.job-card:focus-visible {
  outline: 2px solid var(--term-accent);
  outline-offset: 2px;
}
.job-card-hd {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  border-bottom: 1px dashed var(--term-dim);
  padding-bottom: 4px;
  margin-bottom: 6px;
}
.job-card-title {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: 1.5px;
}
.job-card-row {
  display: flex;
  gap: 0.5rem;
  align-items: baseline;
  margin-bottom: 4px;
}
.job-card-img {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
</style>
