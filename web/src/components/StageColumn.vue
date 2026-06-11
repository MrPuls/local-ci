<script setup lang="ts">
import { computed } from 'vue';
import JobCard from './JobCard.vue';
import { statusMeta } from '@/lib/status';
import type { PipelineStage } from '@/lib/pipeline';
import type { UiStatus } from '@/lib/types';

const props = defineProps<{
  stage: PipelineStage;
  focusedJob: string | null;
  canRun?: boolean;
}>();
const emit = defineEmits<{
  (e: 'focus', name: string): void;
  (e: 'run-stage', stage: string): void;
}>();

const ORDER: UiStatus[] = ['passed', 'failed', 'running', 'queued', 'skipped', 'idle'];

// "2P 1R" style summary, only listing statuses present in this stage.
const summary = computed(() => {
  const counts: Partial<Record<UiStatus, number>> = {};
  for (const n of props.stage.nodes) counts[n.status] = (counts[n.status] ?? 0) + 1;
  return ORDER.filter((s) => counts[s])
    .map((s) => `${counts[s]}${statusMeta(s).label[0]}`)
    .join(' ');
});
</script>

<template>
  <div class="stage-col" :data-test-id="`stage-${stage.name}`">
    <div class="stage-rule">
      <span>&gt; {{ stage.name }}_</span>
      <span class="dim" style="font-size: 0.95rem">[{{ stage.nodes.length }} JOBS]</span>
      <button
        v-if="canRun"
        class="stage-run"
        :data-test-id="`run-stage-${stage.name}`"
        :title="`RUN ONLY STAGE ${stage.name.toUpperCase()}`"
        @click="emit('run-stage', stage.name)"
      >
        ▶
      </button>
      <span class="line"></span>
      <span class="dim" style="font-size: 0.95rem">{{ summary }}</span>
    </div>
    <div class="stage-jobs">
      <JobCard
        v-for="node in stage.nodes"
        :key="node.name"
        :node="node"
        :focused="focusedJob === node.name"
        @focus="emit('focus', node.name)"
      />
    </div>
  </div>
</template>

<style scoped>
.stage-run {
  background: transparent;
  border: 1px solid var(--term-dim);
  color: var(--term-dim);
  font-family: inherit;
  font-size: 0.8rem;
  line-height: 1.1;
  padding: 0 0.35rem;
  cursor: pointer;
}
.stage-run:hover {
  color: var(--term-accent);
  border-color: var(--term-accent);
  text-shadow: 0 0 8px var(--term-glow-accent);
}
.stage-col {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  flex-shrink: 0;
}
.stage-jobs {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}
</style>
