<script setup lang="ts">
import { useToast } from '@/composables/useToast';

const { current } = useToast();
</script>

<template>
  <Transition name="toast">
    <div
      v-if="current"
      class="toast"
      :class="{ 'toast-error': current.kind === 'error' }"
      data-test-id="toast"
    >
      <span :class="[current.kind === 'error' ? 'error' : 'accent', 'glow-strong']">{{
        current.msg
      }}</span>
    </div>
  </Transition>
</template>
