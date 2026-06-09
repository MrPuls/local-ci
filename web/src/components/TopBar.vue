<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink } from 'vue-router';
import Icon from '@/components/Icon.vue';
import { useHealth } from '@/composables/useHealth';
import { useConfig } from '@/composables/useConfig';
import { useRunStatus } from '@/composables/useRunStatus';
import { useSettings, type Theme } from '@/composables/useSettings';
import { baseName } from '@/lib/format';

const { version, online } = useHealth();
const { config } = useConfig();
const { summary } = useRunStatus();
const { settings, cycleTheme } = useSettings();

const filename = computed(() => baseName(config.value?.path));
const statusClass = computed(() =>
  summary.kind === 'error' ? 'error' : summary.kind === 'idle' ? 'dim' : 'accent',
);

// Theme swatch: a filled circle tinted to each phosphor's foreground color (the
// active theme is always shown in its own color, regardless of the page theme).
const THEME_COLOR: Record<Theme, string> = { amber: '#33ff00', cyan: '#00e5ff', mono: '#d4d4d4' };
const themeColor = computed(() => THEME_COLOR[settings.theme]);
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
        <button class="btn btn-sq" data-test-id="cycle-theme" title="CYCLE PHOSPHOR" @click="cycleTheme()">
          <Icon name="circle" glow :style="{ color: themeColor }" />
        </button>
      </div>
    </div>
  </header>
</template>
