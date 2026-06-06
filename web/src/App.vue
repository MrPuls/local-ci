<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterView } from 'vue-router';
import CrtEffects from '@/components/CrtEffects.vue';
import TopBar from '@/components/TopBar.vue';
import StatusBar from '@/components/StatusBar.vue';
import ToastHost from '@/components/ToastHost.vue';
import TweaksPanel from '@/components/TweaksPanel.vue';
import { useSettings } from '@/composables/useSettings';
import { useCrtFx } from '@/composables/useCrtFx';
import { useHealth } from '@/composables/useHealth';
import { useConfig } from '@/composables/useConfig';

const { settings } = useSettings();
const { glitching, vsync } = useCrtFx();
const { refresh: refreshHealth } = useHealth();
const { refresh: refreshConfig } = useConfig();

const tweaksOpen = ref(false);

onMounted(() => {
  refreshHealth();
  refreshConfig();
});
</script>

<template>
  <CrtEffects />
  <!-- flicker on the outer frame, glitch/v-sync on the inner app, so both
       animations can run at once (each element owns one `animation`). -->
  <div :class="{ 'crt-flicker': settings.flicker }">
    <div class="app" :class="{ 'glitch-active': glitching, 'vsync-active': vsync }">
      <TopBar @open-settings="tweaksOpen = true" />
      <main class="grow" style="min-width: 0">
        <RouterView />
      </main>
      <StatusBar />
    </div>
  </div>
  <ToastHost />
  <TweaksPanel :open="tweaksOpen" @close="tweaksOpen = false" />
</template>
