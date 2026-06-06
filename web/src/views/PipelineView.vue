<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import PipelineGraph from '@/components/PipelineGraph.vue';
import Inspector from '@/components/Inspector.vue';
import LogFeed from '@/components/LogFeed.vue';
import { useConfig } from '@/composables/useConfig';
import { useLiveRun } from '@/composables/useLiveRun';
import { useRunStatus, type RunKind } from '@/composables/useRunStatus';
import { useToast } from '@/composables/useToast';
import { allNodes, mergePipeline, type RunContext } from '@/lib/pipeline';
import { cancelRun, triggerRun } from '@/lib/api';
import { shortId } from '@/lib/format';
import type { RunMode, UiStatus } from '@/lib/types';

const route = useRoute();
const router = useRouter();
const { config, loading: configLoading, error: configError, refresh: refreshConfig } = useConfig();
const { state: live, error: liveError, connect, disconnect } = useLiveRun();
const { set: setStatus, setCounts, reset: resetStatus } = useRunStatus();
const { push } = useToast();

// --- current run, driven by the optional :id route param ----------------
const runId = computed(() => (route.params.id as string | undefined) ?? null);

watch(
  runId,
  (id) => {
    if (id) connect(id);
    else disconnect();
  },
  { immediate: true },
);

const runContext = computed<RunContext | null>(() => {
  if (!runId.value) return null;
  return { active: !live.finished, finished: live.finished, jobs: live.jobs };
});

const stages = computed(() => mergePipeline(config.value, runContext.value));
const nodes = computed(() => allNodes(stages.value));

// --- run label + counts published to the global chrome ------------------
const failedCount = computed(() => nodes.value.filter((n) => n.status === 'failed').length);
const label = computed<{ text: string; kind: RunKind }>(() => {
  if (!runId.value) return { text: 'STANDBY', kind: 'idle' };
  if (!live.finished) return { text: 'EXECUTING', kind: 'running' };
  return failedCount.value > 0
    ? { text: `HALTED · ${failedCount.value}_FAILED`, kind: 'error' }
    : { text: 'OK', kind: 'done' };
});
watch(label, (l) => setStatus(l.text, l.kind), { immediate: true });
watch(stages, (s) => setCounts(s), { immediate: true });
onUnmounted(resetStatus);

// --- run control --------------------------------------------------------
const mode = ref<RunMode>('sequential');
const MODES: RunMode[] = ['sequential', 'parallel', 'parallel-stages'];
const canRun = computed(() => !runContext.value?.active);
const canCancel = computed(() => !!runContext.value?.active);
const busy = ref(false);

async function onRun(): Promise<void> {
  if (busy.value) return;
  busy.value = true;
  try {
    const id = await triggerRun({ mode: mode.value });
    push('> PIPELINE_STARTED_', 'accent');
    router.push(`/runs/${id}`);
  } catch (e) {
    push(`ERROR: ${e instanceof Error ? e.message : String(e)}`, 'error');
  } finally {
    busy.value = false;
  }
}

async function onCancel(): Promise<void> {
  if (!runId.value) return;
  try {
    await cancelRun(runId.value);
    push('ERROR: HALTED_BY_USER', 'error');
  } catch (e) {
    push(`ERROR: ${e instanceof Error ? e.message : String(e)}`, 'error');
  }
}

// --- inspector + log feed UI state --------------------------------------
const focusedJob = ref<string | null>(null);
const inspectorOpen = ref(false);
const openLogs = ref<string[]>([]);
const activeLog = ref<string | null>(null);
const logsMin = ref(false);

const focusedNode = computed(() => nodes.value.find((n) => n.name === focusedJob.value) ?? null);
const logAttached = computed(() => !!focusedJob.value && openLogs.value.includes(focusedJob.value));

const jobStatus = computed<Record<string, UiStatus>>(() => {
  const m: Record<string, UiStatus> = {};
  for (const n of nodes.value) m[n.name] = n.status;
  return m;
});

function onFocus(name: string): void {
  focusedJob.value = name;
  inspectorOpen.value = true;
}
function checkLogs(name: string): void {
  if (!openLogs.value.includes(name)) openLogs.value = [...openLogs.value, name];
  activeLog.value = name;
  logsMin.value = false;
  push(`> TAILING ${name}_`, 'accent');
}
function closeLog(name: string): void {
  const next = openLogs.value.filter((j) => j !== name);
  openLogs.value = next;
  if (activeLog.value === name) activeLog.value = next[next.length - 1] ?? null;
}
</script>

<template>
  <div class="col" data-test-id="pipeline-view">
    <!-- run control -->
    <div class="panel controls" data-test-id="run-controls">
      <label for="mode-select">MODE:</label>
      <select id="mode-select" v-model="mode" class="term" data-test-id="mode-select">
        <option v-for="m in MODES" :key="m" :value="m">{{ m.toUpperCase() }}</option>
      </select>
      <button
        class="btn btn-accent"
        data-test-id="run-pipeline"
        :disabled="!canRun || busy"
        @click="onRun"
      >
        {{ busy ? 'STARTING...' : '▶ RUN_PIPELINE' }}
      </button>
      <button class="btn btn-error" data-test-id="cancel-run" :disabled="!canCancel" @click="onCancel">
        ■ STOP
      </button>
      <span class="grow"></span>
      <span v-if="runId" class="dim" data-test-id="current-run-id">RUN: {{ shortId(runId) }}</span>
      <button class="btn btn-sq" title="RELOAD_CONFIG" data-test-id="reload-config" @click="refreshConfig()">
        ↻ CONFIG
      </button>
    </div>

    <div v-if="liveError" class="banner" data-test-id="live-error">
      <span class="error">ERROR: {{ liveError }}</span>
    </div>

    <!-- work area: graph + inspector -->
    <div class="work" :class="{ 'work-full': !inspectorOpen }">
      <PipelineGraph
        :stages="stages"
        :focused-job="focusedJob"
        :loading="configLoading"
        :error="configError"
        @focus="onFocus"
      />
      <Inspector
        v-if="inspectorOpen"
        :node="focusedNode"
        :nodes="nodes"
        :log-attached="logAttached"
        @close="inspectorOpen = false"
        @check-logs="checkLogs"
      />
    </div>

    <!-- log feed -->
    <LogFeed
      :lines="live.log"
      :open-jobs="openLogs"
      :active-job="activeLog"
      :job-status="jobStatus"
      :minimized="logsMin"
      @focus="activeLog = $event"
      @close="closeLog"
      @minimize="logsMin = true"
      @restore="logsMin = false"
    />
  </div>
</template>
