<script setup lang="ts">
// The fixed full-viewport CRT overlays: the vignette/bezel ("field"), the
// scanline interlace, and the crawling scanbar. Flicker/glitch/v-sync are
// applied to the app frame by App.vue (they need a wrapped element, not an
// overlay).
</script>

<template>
  <div class="crt-field" aria-hidden="true" data-test-id="crt-field"></div>
  <div class="crt-scanbar" aria-hidden="true" data-test-id="crt-scanbar"></div>
  <div class="crt-scanlines" aria-hidden="true" data-test-id="crt-scanlines"></div>
</template>
