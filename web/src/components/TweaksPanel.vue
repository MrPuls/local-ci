<script setup lang="ts">
import { useSettings, THEMES, type Theme } from '@/composables/useSettings';

defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: 'close'): void }>();

const { settings, setTheme, toggle } = useSettings();

const EFFECTS: { key: 'scanlines' | 'flicker' | 'glitch' | 'vsync'; label: string }[] = [
  { key: 'scanlines', label: 'SCANLINES' },
  { key: 'flicker', label: 'FLICKER' },
  { key: 'glitch', label: 'GLITCH' },
  { key: 'vsync', label: 'V-SYNC_TEAR' },
];
const themeLabel = (t: Theme) => t.toUpperCase();
</script>

<template>
  <aside v-if="open" class="tweaks" data-test-id="tweaks-panel">
    <div class="panel-hd" style="margin: 0">
      <span>TWEAKS</span>
      <button
        class="inspector-close"
        data-test-id="tweaks-close"
        aria-label="Close tweaks"
        @click="emit('close')"
      >
        [ X ]
      </button>
    </div>

    <div class="tweak-section">
      <span class="tweak-section-label">&gt; PHOSPHOR_</span>
      <div class="seg" role="radiogroup" aria-label="Theme">
        <button
          v-for="t in THEMES"
          :key="t"
          role="radio"
          :aria-checked="settings.theme === t"
          :class="{ active: settings.theme === t }"
          :data-test-id="`theme-${t}`"
          @click="setTheme(t)"
        >
          {{ themeLabel(t) }}
        </button>
      </div>
    </div>

    <div class="tweak-section">
      <span class="tweak-section-label">&gt; CRT_FX_</span>
      <div v-for="fx in EFFECTS" :key="fx.key" class="tweak-row">
        <span>{{ fx.label }}</span>
        <button
          class="toggle"
          role="switch"
          :aria-checked="settings[fx.key]"
          :data-on="settings[fx.key]"
          :data-test-id="`toggle-${fx.key}`"
          @click="toggle(fx.key)"
        >
          {{ settings[fx.key] ? '[ ON ]' : '[ OFF ]' }}
        </button>
      </div>
    </div>
  </aside>
</template>
