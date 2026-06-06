<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink } from 'vue-router';
import { useHealth } from '@/composables/useHealth';
import { useConfig } from '@/composables/useConfig';
import { useRunStatus } from '@/composables/useRunStatus';
import { useSettings, type Theme } from '@/composables/useSettings';
import { useToast } from '@/composables/useToast';
import { baseName } from '@/lib/format';

const emit = defineEmits<{ (e: 'open-settings'): void }>();

const { version, online } = useHealth();
const { config } = useConfig();
const { summary } = useRunStatus();
const { settings, cycleTheme } = useSettings();
const { push } = useToast();

const filename = computed(() => baseName(config.value?.path));
const statusClass = computed(() =>
  summary.kind === 'error' ? 'error' : summary.kind === 'idle' ? 'dim' : 'accent',
);

const THEME_GLYPH: Record<Theme, string> = { amber: '🟠', cyan: '🔵', mono: '⚪️' };
const themeGlyph = computed(() => THEME_GLYPH[settings.theme]);

function onCycleTheme(): void {
  const next = cycleTheme();
  push(`> PHOSPHOR_${next.toUpperCase()}_OK_`, 'accent');
}
</script>

<template>
  <header class="panel" style="padding: 0.8rem 1rem" data-test-id="top-bar">
    <div style="display: flex; align-items: center; gap: 1.25rem; flex-wrap: wrap">
      <div style="display: flex; align-items: baseline; gap: 0.6rem">
        <span class="accent glow-strong" style="font-size: 1.8rem; letter-spacing: 3px"
          >LOCAL_CI</span
        >
        <span class="dim" v-if="version" data-test-id="version">V{{ version }}</span>
        <span class="error" v-else-if="online === false" data-test-id="offline">OFFLINE</span>
      </div>

      <div class="dim">|</div>
      <div style="display: flex; align-items: baseline; gap: 0.6rem">
        <span class="dim">FILE:</span>
        <span data-test-id="config-file">{{ filename }}</span>
      </div>

      <nav class="nav" style="margin-left: 0.5rem">
        <RouterLink to="/" class="nav-link" exact-active-class="active">PIPELINE</RouterLink>
        <RouterLink to="/history" class="nav-link" active-class="active">HISTORY</RouterLink>
      </nav>

      <div style="flex: 1"></div>

      <div style="display: flex; align-items: baseline; gap: 0.6rem">
        <span class="dim">STATUS:</span>
        <span :class="[statusClass, 'glow-strong']" data-test-id="run-status-label"
          >{{ summary.label }}_</span
        >
      </div>

      <div style="display: flex; gap: 0.5rem">
        <button class="btn btn-sq" data-test-id="cycle-theme" title="CYCLE PHOSPHOR" @click="onCycleTheme">
          <span style="filter: saturate(1.2)">{{ themeGlyph }}</span>
        </button>
        <button class="btn btn-sq" data-test-id="open-settings" title="TWEAKS" @click="emit('open-settings')">
          ⚙ TWEAKS
        </button>
      </div>
    </div>
  </header>
</template>
