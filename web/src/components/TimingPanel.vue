<script setup lang="ts">
import { computed } from 'vue';
import { fmtSeconds } from '@/lib/format';
import { barFillClass, statusMeta } from '@/lib/status';
import type { PipelineNode } from '@/lib/pipeline';

const props = defineProps<{ nodes: PipelineNode[]; focusedName: string | null }>();

const max = computed(() => Math.max(1, ...props.nodes.map((n) => n.durationMs)));

function rowClass(n: PipelineNode): string {
  return statusMeta(n.status).cls;
}
function width(n: PipelineNode): string {
  return `${n.durationMs > 0 ? (n.durationMs / max.value) * 100 : 0}%`;
}
</script>

<template>
  <div style="display: flex; flex-direction: column; gap: 0.6rem" data-test-id="inspector-timing">
    <div class="dim">// DURATION_PER_JOB · SCALED TO MAX</div>
    <table class="term">
      <thead>
        <tr>
          <th>JOB</th>
          <th>S</th>
          <th>BAR</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="n in nodes" :key="n.name">
          <td :class="focusedName === n.name ? 'glow-strong' : rowClass(n)">
            <span v-if="focusedName === n.name" class="accent">&gt; </span>{{ n.name }}
          </td>
          <td>{{ n.durationMs > 0 ? fmtSeconds(n.durationMs) : '--' }}</td>
          <td style="min-width: 140px">
            <div class="bar">
              <div class="bar-fill" :class="barFillClass(n.status)" :style="{ width: width(n) }"></div>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
