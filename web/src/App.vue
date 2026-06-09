<script setup lang="ts">
import { onMounted } from 'vue';
import { RouterView } from 'vue-router';
import CrtEffects from '@/components/CrtEffects.vue';
import TopBar from '@/components/TopBar.vue';
import StatusBar from '@/components/StatusBar.vue';
import ToastHost from '@/components/ToastHost.vue';
import { useCrtFx } from '@/composables/useCrtFx';
import { useHealth } from '@/composables/useHealth';
import { useConfig } from '@/composables/useConfig';

const { glitching, vsync } = useCrtFx();
const { refresh: refreshHealth } = useHealth();
const { refresh: refreshConfig } = useConfig();

onMounted(() => {
  refreshHealth();
  refreshConfig();
});
</script>

<template>
  <CrtEffects />
  <!-- flicker on the outer frame, glitch/v-sync on the inner app, so both
       animations can run at once (each element owns one `animation`). -->
  <div class="crt-flicker">
    <div class="app" :class="{ 'glitch-active': glitching, 'vsync-active': vsync }">
      <TopBar />
      <main class="grow" style="min-width: 0">
        <RouterView />
      </main>
      <StatusBar />
    </div>
  </div>
  <ToastHost />
</template>
