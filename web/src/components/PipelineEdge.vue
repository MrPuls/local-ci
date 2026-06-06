<script setup lang="ts">
import { computed } from 'vue';
import type { UiStatus } from '@/lib/types';

const props = defineProps<{ status: UiStatus }>();

const cls = computed(() => {
  if (props.status === 'running') return 'accent';
  if (props.status === 'passed') return '';
  return 'dim';
});
const broken = computed(() => props.status === 'failed' || props.status === 'skipped');
</script>

<template>
  <div class="edge" :data-test-id="`edge-${status}`">
    <span class="px-arrow" :class="[cls, { broken }]"></span>
  </div>
</template>

<style scoped>
.edge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 84px;
  flex-shrink: 0;
  padding: 0 10px;
}
</style>
