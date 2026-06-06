import { onUnmounted, ref, watch } from 'vue';
import { useSettings } from './useSettings';

// Orchestrates the two stochastic CRT effects from the design:
//   glitch — every ~3s, 15% chance of a 110–550ms chromatic-split jitter
//   v-sync — every ~8s, 10% chance of a 400ms skew/hue-rotate burst
// Returns reactive flags App.vue binds as .glitch-active / .vsync-active on the
// app frame. Timers start/stop with the corresponding setting.

export function useCrtFx() {
  const { settings } = useSettings();
  const glitching = ref(false);
  const vsync = ref(false);

  let glitchTimer: ReturnType<typeof setInterval> | undefined;
  let glitchClear: ReturnType<typeof setTimeout> | undefined;
  let vsyncTimer: ReturnType<typeof setInterval> | undefined;
  let vsyncClear: ReturnType<typeof setTimeout> | undefined;

  function startGlitch() {
    stopGlitch();
    glitchTimer = setInterval(() => {
      if (Math.random() < 0.15) {
        glitching.value = true;
        glitchClear = setTimeout(() => (glitching.value = false), 110 + Math.random() * 440);
      }
    }, 3000);
  }
  function stopGlitch() {
    if (glitchTimer) clearInterval(glitchTimer);
    if (glitchClear) clearTimeout(glitchClear);
    glitchTimer = glitchClear = undefined;
    glitching.value = false;
  }

  function startVsync() {
    stopVsync();
    vsyncTimer = setInterval(() => {
      if (Math.random() < 0.1) {
        vsync.value = true;
        vsyncClear = setTimeout(() => (vsync.value = false), 400);
      }
    }, 8000);
  }
  function stopVsync() {
    if (vsyncTimer) clearInterval(vsyncTimer);
    if (vsyncClear) clearTimeout(vsyncClear);
    vsyncTimer = vsyncClear = undefined;
    vsync.value = false;
  }

  watch(
    () => settings.glitch,
    (on) => (on ? startGlitch() : stopGlitch()),
    { immediate: true },
  );
  watch(
    () => settings.vsync,
    (on) => (on ? startVsync() : stopVsync()),
    { immediate: true },
  );

  onUnmounted(() => {
    stopGlitch();
    stopVsync();
  });

  return { glitching, vsync };
}
