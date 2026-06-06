<script setup lang="ts">
import { computed } from 'vue';
import { statusMeta } from '@/lib/status';
import type { UiStatus } from '@/lib/types';

const props = defineProps<{ status: UiStatus }>();
const m = computed(() => statusMeta(props.status));
const fx = computed(() => (m.value.motion === 'pulse' ? 'soft-pulse' : ''));
</script>

<template>
  <span class="chip" :class="[m.cls, fx]" :data-test-id="`status-chip-${status}`">
    <span>{{ m.glyph }}</span>
    <span>{{ m.label }}</span>
  </span>
</template>
