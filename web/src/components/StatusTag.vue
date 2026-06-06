<script setup lang="ts">
import { computed } from 'vue';
import { statusMeta } from '@/lib/status';
import type { UiStatus } from '@/lib/types';

const props = withDefaults(defineProps<{ status: UiStatus; compact?: boolean }>(), {
  compact: false,
});

const m = computed(() => statusMeta(props.status));
const fx = computed(() => (m.value.motion === 'pulse' ? 'soft-pulse' : ''));
const text = computed(() => (props.compact ? m.value.glyph : `${m.value.glyph} ${m.value.label}`));
</script>

<template>
  <span :class="[m.cls, fx]" :data-test-id="`status-tag-${status}`">{{ text }}</span>
</template>
