<script setup lang="ts">
import { computed } from 'vue';
import StageColumn from './StageColumn.vue';
import PipelineEdge from './PipelineEdge.vue';
import { allNodes, edgeStatus, type PipelineStage } from '@/lib/pipeline';

const props = defineProps<{
  stages: PipelineStage[];
  focusedJob: string | null;
  loading?: boolean;
  error?: string | null;
}>();
const emit = defineEmits<{ (e: 'focus', name: string): void }>();

const jobCount = computed(() => allNodes(props.stages).length);
</script>

<template>
  <section class="panel" style="overflow: hidden" data-test-id="pipeline-graph">
    <div class="panel-hd">
      <span>PIPELINE_GRAPH</span>
      <span class="dim" style="font-weight: normal">
        {{ jobCount }} JOBS · {{ stages.length }} STAGES
      </span>
    </div>

    <div v-if="error" class="banner" data-test-id="graph-error">
      <span class="error">ERROR: {{ error }}</span>
    </div>

    <div v-else-if="loading && stages.length === 0" class="empty">
      <span class="dim">&gt; LOADING_CONFIG_</span><span class="blink accent">_</span>
    </div>

    <div v-else-if="stages.length === 0" class="empty" data-test-id="graph-empty">
      <div class="dim" style="font-size: 1.4rem">&gt; NO_PIPELINE_FOUND</div>
      <div class="dim">CHECK .LOCAL-CI.YAML EXISTS AND IS VALID_</div>
    </div>

    <div v-else style="overflow-x: auto; padding-bottom: 4px">
      <div style="display: flex; align-items: stretch; gap: 0">
        <template v-for="(stage, i) in stages" :key="stage.name">
          <StageColumn :stage="stage" :focused-job="focusedJob" @focus="emit('focus', $event)" />
          <div v-if="i < stages.length - 1" style="display: flex; align-items: center">
            <PipelineEdge :status="edgeStatus(stage, stages[i + 1])" />
          </div>
        </template>
      </div>
    </div>
  </section>
</template>
