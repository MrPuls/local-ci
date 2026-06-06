import { onUnmounted, ref } from 'vue';

// A shared 1s-ticking clock (epoch ms). Components read it to compute live
// elapsed times for running jobs without each spinning its own timer. The
// interval runs only while at least one consumer is mounted.

const now = ref(Date.now());
let consumers = 0;
let timer: ReturnType<typeof setInterval> | undefined;

export function useNow() {
  consumers += 1;
  if (!timer) {
    timer = setInterval(() => (now.value = Date.now()), 1000);
  }
  onUnmounted(() => {
    consumers -= 1;
    if (consumers <= 0 && timer) {
      clearInterval(timer);
      timer = undefined;
    }
  });
  return now;
}
