<script setup lang="ts">
import { computed } from 'vue';
import Icon from './Icon.vue';
import { statusMeta } from '@/lib/status';
import type { UiStatus } from '@/lib/types';

const props = defineProps<{ status: UiStatus }>();
const m = computed(() => statusMeta(props.status));
</script>

<template>
  <span class="chip" :class="m.cls" :data-test-id="`status-chip-${status}`">
    <Icon :name="m.icon" :spin="m.motion === 'pulse'" glow />
    <span>{{ m.label }}</span>
  </span>
</template>
