<script setup lang="ts">
import { computed } from 'vue';

// Pixel-grid icons (16x16, fill="currentColor") from the design set. They are
// loaded eagerly as raw strings and inlined so the SVG inherits the active
// phosphor color via currentColor and scales like a glyph. Size/color come from
// CSS on the wrapper (the SVG's own width/height attrs are overridden), so an
// <Icon> sits inline in text at the surrounding font-size.
const RAW = import.meta.glob('@/assets/icons/*.svg', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>;

const BY_NAME: Record<string, string> = {};
for (const [path, raw] of Object.entries(RAW)) {
  const name = path.slice(path.lastIndexOf('/') + 1).replace(/\.svg$/, '');
  BY_NAME[name] = raw;
}

const props = withDefaults(
  defineProps<{
    /** Icon file name without extension, e.g. "play", "spinner". */
    name: string;
    /** Continuous rotation (mechanical step easing) — for the spinner. */
    spin?: boolean;
    /** Phosphor drop-shadow halo to match the CRT glow on text. */
    glow?: boolean;
  }>(),
  { spin: false, glow: false },
);

const svg = computed(() => BY_NAME[props.name] ?? '');

if (import.meta.env.DEV && !svg.value) {
  console.warn(`[Icon] unknown icon "${props.name}"`);
}
</script>

<template>
  <span class="icon" :class="{ spin, glow }" :data-icon="name" aria-hidden="true" v-html="svg" />
</template>

<style scoped>
.icon {
  display: inline-flex;
  width: 1em;
  height: 1em;
  vertical-align: -0.125em;
  line-height: 1;
  flex: none;
}
.icon :deep(svg) {
  width: 100%;
  height: 100%;
  display: block;
}
.icon.glow :deep(svg) {
  filter: drop-shadow(0 0 4px currentColor);
}
.icon.spin {
  animation: icon-spin 1s steps(8, end) infinite;
}
@keyframes icon-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .icon.spin {
    animation: none;
  }
}
</style>
