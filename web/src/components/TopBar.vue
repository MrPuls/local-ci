<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink } from 'vue-router';
import Icon from '@/components/Icon.vue';
import { useHealth } from '@/composables/useHealth';
import { useConfig } from '@/composables/useConfig';
import { useRunStatus } from '@/composables/useRunStatus';
import { useSettings, type Theme } from '@/composables/useSettings';
import { useSystem } from '@/composables/useSystem';
import { useConfigs } from '@/composables/useConfigs';
import { baseName } from '@/lib/format';

const { version, online } = useHealth();
const { config } = useConfig();
const { selectorOpen, refresh: refreshConfigs } = useConfigs();

// The FILE chip reopens the boot-time config selector with a fresh scan.
function openSelector(): void {
  refreshConfigs().then(() => {
    selectorOpen.value = true;
  });
}
const { summary } = useRunStatus();
const { settings, cycleTheme } = useSettings();
const { engine } = useSystem();

// Container-engine (Docker/OrbStack) readiness chip.
const engineLabel = computed(() => {
  if (!engine.value) return 'CHECKING';
  const name = engine.value.provider.toUpperCase().replace(/\s+/g, '_');
  return engine.value.ready ? name : `${name}_OFFLINE`;
});
const engineIcon = computed(() => (engine.value && !engine.value.ready ? 'cross' : 'dot'));
const engineClass = computed(() => {
  if (!engine.value) return 'dim';
  return engine.value.ready ? 'glow-strong' : 'error';
});
const engineTitle = computed(() => {
  if (!engine.value) return 'Checking container engine…';
  return engine.value.ready
    ? `${engine.value.provider} ${engine.value.version} — ready to run jobs`
    : `${engine.value.provider} not reachable — start it to run jobs`;
});

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
        <button
          class="file-chip"
          data-test-id="config-file"
          title="Switch config file"
          @click="openSelector"
        >
          {{ filename }}
        </button>
      </div>

      <nav class="nav" style="margin-left: 0.5rem">
        <RouterLink to="/" class="nav-link" exact-active-class="active">PIPELINE</RouterLink>
        <RouterLink to="/config" class="nav-link" active-class="active">CONFIG</RouterLink>
        <RouterLink to="/history" class="nav-link" active-class="active">HISTORY</RouterLink>
      </nav>

      <div style="flex: 1"></div>

      <div style="display: flex; align-items: baseline; gap: 0.6rem">
        <span class="dim">STATUS:</span>
        <span :class="[statusClass, 'glow-strong']" data-test-id="run-status-label"
          >{{ summary.label }}_</span
        >
      </div>

      <div class="dim">|</div>
      <div style="display: flex; align-items: baseline; gap: 0.5rem" data-test-id="engine-status" :title="engineTitle">
        <span class="dim">ENGINE:</span>
        <span :class="engineClass"><Icon :name="engineIcon" glow /> {{ engineLabel }}</span>
      </div>

      <div style="display: flex; gap: 0.5rem">
        <button class="btn btn-sq" data-test-id="cycle-theme" title="CYCLE PHOSPHOR" @click="cycleTheme()">
          <Icon name="circle" glow :style="{ color: themeColor }" />
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
/* FILE chip — text-styled button so the filename reads as a value but invites
   a click (reopens the config selector). */
.file-chip {
  background: transparent;
  border: none;
  border-bottom: 1px dashed var(--term-dim);
  color: var(--term-fg);
  font-family: inherit;
  font-size: inherit;
  letter-spacing: inherit;
  text-transform: uppercase;
  text-shadow: 0 0 5px var(--term-glow);
  padding: 0;
  cursor: pointer;
}
.file-chip:hover {
  color: var(--term-accent);
  border-bottom-color: var(--term-accent);
  text-shadow: 0 0 8px var(--term-glow-accent);
}
</style>
