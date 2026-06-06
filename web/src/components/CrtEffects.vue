<script setup lang="ts">
import { useSettings } from '@/composables/useSettings';

// The fixed full-viewport CRT overlays. The vignette/bezel ("field") is always
// present; the scanline interlace + crawling scanbar are gated by the
// SCANLINES setting. Flicker/glitch/v-sync are applied to the app frame by
// App.vue (they need a wrapped element, not an overlay).
const { settings } = useSettings();
</script>

<template>
  <div class="crt-field" aria-hidden="true" data-test-id="crt-field"></div>
  <template v-if="settings.scanlines">
    <div class="crt-scanbar" aria-hidden="true" data-test-id="crt-scanbar"></div>
    <div class="crt-scanlines" aria-hidden="true" data-test-id="crt-scanlines"></div>
  </template>
</template>
