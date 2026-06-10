<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import Icon from '@/components/Icon.vue';
import { useConfigs } from '@/composables/useConfigs';
import { useToast } from '@/composables/useToast';

// Boot-time config source picker: "N CONFIG FILES FOUND — SELECT SOURCE".
// Always shown, even for a single file, so the session's config is an explicit
// choice. Esc keeps the current selection.

const { files, dir, selectorOpen, select } = useConfigs();
const { push } = useToast();

const cursor = ref(0);
const busy = ref(false);

watch(selectorOpen, (open) => {
  if (!open) return;
  const activeIdx = files.value.findIndex((f) => f.active);
  cursor.value = activeIdx >= 0 ? activeIdx : 0;
});

const count = computed(() => files.value.length);

async function choose(idx: number): Promise<void> {
  const file = files.value[idx];
  if (!file || busy.value) return;
  busy.value = true;
  try {
    await select(file.name);
    push(`> CONFIG_LOADED: ${file.name.toUpperCase()}_`, 'accent');
    selectorOpen.value = false;
  } catch (e) {
    push(`ERROR: ${e instanceof Error ? e.message : String(e)}`, 'error');
  } finally {
    busy.value = false;
  }
}

function onKey(e: KeyboardEvent): void {
  if (!selectorOpen.value) return;
  if (e.key === 'ArrowDown') {
    cursor.value = (cursor.value + 1) % count.value;
  } else if (e.key === 'ArrowUp') {
    cursor.value = (cursor.value - 1 + count.value) % count.value;
  } else if (e.key === 'Enter') {
    void choose(cursor.value);
  } else if (e.key === 'Escape') {
    selectorOpen.value = false;
  } else if (/^[1-9]$/.test(e.key) && Number(e.key) <= count.value) {
    void choose(Number(e.key) - 1);
  } else {
    return;
  }
  e.preventDefault();
}

onMounted(() => window.addEventListener('keydown', onKey));
onUnmounted(() => window.removeEventListener('keydown', onKey));
</script>

<template>
  <div v-if="selectorOpen" class="modal-backdrop" data-test-id="config-select-modal">
    <section class="panel panel-accent modal">
      <div class="panel-hd">
        <span>CONFIG_SOURCE</span>
        <button class="inspector-close" data-test-id="config-select-close" title="Keep current config" @click="selectorOpen = false">
          [ESC]
        </button>
      </div>

      <div class="modal-lead">
        <span class="accent glow-strong">{{ count }}</span> CONFIG FILE{{ count === 1 ? '' : 'S' }}
        FOUND — SELECT SOURCE:
      </div>
      <div class="dim modal-dir"><Icon name="folder" /> {{ dir }}</div>

      <ul class="file-list">
        <li v-for="(f, i) in files" :key="f.path">
          <button
            class="file-row"
            :class="{ focused: i === cursor }"
            :data-test-id="`config-option-${f.name}`"
            :disabled="busy"
            @mouseenter="cursor = i"
            @click="choose(i)"
          >
            <span class="dim idx">[{{ i + 1 }}]</span>
            <Icon name="file" />
            <span class="name">{{ f.name }}</span>
            <span class="grow"></span>
            <span v-if="f.active" class="accent marker">&lt; ACTIVE</span>
            <span v-if="!f.exists" class="error marker">MISSING</span>
          </button>
        </li>
      </ul>

      <div class="dim modal-hint">
        ↑/↓ SELECT · ENTER LOAD · 1–{{ Math.min(count, 9) }} QUICK PICK · ESC KEEP CURRENT
      </div>
    </section>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 9980; /* under the CRT overlays (9997+), above the app */
  background: rgba(0, 0, 0, 0.75);
  display: grid;
  place-items: center;
}
.modal {
  width: min(620px, calc(100vw - 3rem));
  background: var(--term-page);
}
.modal-lead {
  margin-bottom: 0.2rem;
}
.modal-dir {
  font-size: var(--fs-small);
  text-transform: none;
  margin-bottom: 0.8rem;
  word-break: break-all;
}
.file-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  max-height: 50vh;
  overflow-y: auto;
}
.file-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  background: transparent;
  border: 2px solid var(--term-dim);
  color: var(--term-fg);
  font-family: inherit;
  font-size: var(--fs-body);
  /* real filenames, case preserved — the global uppercase would lie here */
  text-transform: none;
  letter-spacing: 1px;
  padding: 0.3rem 0.7rem;
  cursor: pointer;
  text-align: left;
}
.file-row.focused {
  border-color: var(--term-accent);
  box-shadow: 0 0 12px var(--term-glow-accent);
}
.file-row.focused .name {
  color: var(--term-accent);
  text-shadow: 0 0 8px var(--term-glow-accent);
}
.file-row:disabled {
  opacity: 0.5;
  cursor: wait;
}
.idx {
  min-width: 2.2rem;
}
.marker {
  font-size: var(--fs-small);
  letter-spacing: 2px;
}
.modal-hint {
  margin-top: 0.9rem;
  font-size: var(--fs-small);
  letter-spacing: 1px;
}
</style>
