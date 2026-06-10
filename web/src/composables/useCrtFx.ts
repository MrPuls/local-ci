import { onMounted, onUnmounted, ref } from 'vue';

// Orchestrates the two stochastic CRT effects from the design:
//   glitch — every ~3s, 15% chance of a 110–550ms chromatic-split jitter
//   v-sync — every ~8s, 10% chance of a 400ms skew/hue-rotate burst
// Returns reactive flags App.vue binds as .glitch-active / .vsync-active on the
// app frame. Always on (the effects are part of the CRT look, not configurable).

export function useCrtFx() {
  const glitching = ref(false);
  const vsync = ref(false);

  let glitchTimer: ReturnType<typeof setInterval> | undefined;
  let glitchClear: ReturnType<typeof setTimeout> | undefined;
  let vsyncTimer: ReturnType<typeof setInterval> | undefined;
  let vsyncClear: ReturnType<typeof setTimeout> | undefined;

  onMounted(() => {
    glitchTimer = setInterval(() => {
      if (Math.random() < 0.15) {
        glitching.value = true;
        glitchClear = setTimeout(() => (glitching.value = false), 110 + Math.random() * 440);
      }
    }, 3000);
    vsyncTimer = setInterval(() => {
      if (Math.random() < 0.1) {
        vsync.value = true;
        vsyncClear = setTimeout(() => (vsync.value = false), 400);
      }
    }, 8000);
  });

  onUnmounted(() => {
    if (glitchTimer) clearInterval(glitchTimer);
    if (glitchClear) clearTimeout(glitchClear);
    if (vsyncTimer) clearInterval(vsyncTimer);
    if (vsyncClear) clearTimeout(vsyncClear);
  });

  return { glitching, vsync };
}
