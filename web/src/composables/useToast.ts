import { ref } from 'vue';

// A tiny shared toast queue. The design shows a single CRT toast top-right that
// auto-dismisses; we keep one active at a time, replacing it on a new push.

export type ToastKind = 'accent' | 'error';

export interface Toast {
  id: number;
  msg: string;
  kind: ToastKind;
}

const current = ref<Toast | null>(null);
let nextId = 1;
let timer: ReturnType<typeof setTimeout> | undefined;

export function useToast() {
  function push(msg: string, kind: ToastKind = 'accent', ttlMs = 2400): void {
    current.value = { id: nextId++, msg, kind };
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      current.value = null;
    }, ttlMs);
  }

  function dismiss(): void {
    if (timer) clearTimeout(timer);
    current.value = null;
  }

  return { current, push, dismiss };
}
