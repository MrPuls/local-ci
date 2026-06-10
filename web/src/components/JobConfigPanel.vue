<script setup lang="ts">
import { computed } from 'vue';
import { fmtSeconds } from '@/lib/format';
import type { PipelineNode } from '@/lib/pipeline';

const props = defineProps<{ node: PipelineNode }>();

const execKind = computed(() => (props.node.execKind || 'standalone').toUpperCase());
</script>

<template>
  <div style="display: flex; flex-direction: column; gap: 0.9rem" data-test-id="inspector-config">
    <div>
      <div class="dim" style="margin-bottom: 4px">&gt; IDENTITY_</div>
      <div class="kv">
        <div class="k">JOB</div>
        <div class="v">{{ node.name }}</div>
        <div class="k">STAGE</div>
        <div class="v alt">{{ node.stage || '—' }}</div>
        <div class="k">IMAGE</div>
        <div class="v">{{ node.image || '—' }}</div>
      </div>
    </div>

    <div>
      <div class="dim" style="margin-bottom: 4px">&gt; EXECUTION_</div>
      <div class="kv">
        <div class="k">EXEC_KIND</div>
        <div class="v">{{ execKind }}</div>
        <div class="k">PARALLEL</div>
        <div class="v">{{ node.parallel ? 'YES' : 'NO' }}</div>
        <div class="k">VARIANTS</div>
        <div class="v">{{ node.variantCount > 1 ? `${node.variantCount} (MATRIX)` : '1' }}</div>
      </div>
    </div>

    <div v-if="node.ran">
      <div class="dim" style="margin-bottom: 4px">&gt; RESULT_</div>
      <div class="kv">
        <div class="k">EXIT_CODE</div>
        <div class="v" :class="{ error: node.exitCode }">{{ node.exitCode ?? 0 }}</div>
        <div class="k">DURATION</div>
        <div class="v">{{ fmtSeconds(node.durationMs) }}S</div>
        <template v-if="node.error">
          <div class="k">ERROR</div>
          <div class="v error">{{ node.error }}</div>
        </template>
      </div>
    </div>

    <div class="dim" style="font-size: 1rem; line-height: 1.3">
      // SCRIPT, VARIABLES AND RULES ARE NOT EXPOSED BY THE SERVER API_
      <br />
      // VIEW THEM IN {{ node.stage ? '.LOCAL-CI.YAML' : 'THE CONFIG FILE' }}
    </div>
  </div>
</template>
