<script setup lang="ts">
import { computed } from 'vue';
import Icon from './Icon.vue';
import { statusMeta } from '@/lib/status';
import type { UiStatus } from '@/lib/types';

const props = withDefaults(defineProps<{ status: UiStatus; compact?: boolean }>(), {
  compact: false,
});

const m = computed(() => statusMeta(props.status));
</script>

<template>
  <span class="status-tag" :class="m.cls" :data-test-id="`status-tag-${status}`">
    <Icon :name="m.icon" :spin="m.motion === 'pulse'" glow />
    <span v-if="!compact">{{ m.label }}</span>
  </span>
</template>

<style scoped>
.status-tag {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}
</style>
