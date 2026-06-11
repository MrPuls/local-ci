<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import Icon from "@/components/Icon.vue";
import PipelineGraph from "@/components/PipelineGraph.vue";
import Inspector from "@/components/Inspector.vue";
import LogFeed from "@/components/LogFeed.vue";
import { useConfig } from "@/composables/useConfig";
import { useLiveRun } from "@/composables/useLiveRun";
import { useRunStatus, type RunKind } from "@/composables/useRunStatus";
import { useSettings } from "@/composables/useSettings";
import { useToast } from "@/composables/useToast";
import { allNodes, mergePipeline, type RunContext } from "@/lib/pipeline";
import { cancelRun, triggerRun } from "@/lib/api";
import { gitRef, shortId } from "@/lib/format";
import type { RunMode, UiStatus } from "@/lib/types";

const route = useRoute();
const router = useRouter();
const {
    config,
    loading: configLoading,
    error: configError,
    refresh: refreshConfig,
} = useConfig();
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
const failedCount = computed(
    () => nodes.value.filter((n) => n.status === "failed").length,
);
const label = computed<{ text: string; kind: RunKind }>(() => {
    if (!runId.value) return { text: "STANDBY", kind: "idle" };
    if (!live.finished) return { text: "EXECUTING", kind: "running" };
    return failedCount.value > 0
        ? { text: `HALTED · ${failedCount.value}_FAILED`, kind: "error" }
        : { text: "OK", kind: "done" };
});
watch(label, (l) => setStatus(l.text, l.kind), { immediate: true });
watch(stages, (s) => setCounts(s), { immediate: true });
onUnmounted(resetStatus);

// Desktop notification when a run finishes while the tab is hidden (opt-in
// via the bell in the top bar). Pairs with the future Tauri shell.
const { settings } = useSettings();
watch(
    () => live.finished,
    (finished, was) => {
        if (!finished || was || !runId.value) return;
        if (!settings.notify || !document.hidden) return;
        if (!("Notification" in window) || Notification.permission !== "granted")
            return;
        const failed = failedCount.value;
        new Notification(
            failed > 0 ? "LOCAL_CI: PIPELINE FAILED" : "LOCAL_CI: PIPELINE PASSED",
            {
                body:
                    failed > 0
                        ? `${failed} job${failed === 1 ? "" : "s"} failed · run ${shortId(runId.value)}`
                        : `All jobs passed · run ${shortId(runId.value)}`,
                tag: runId.value, // replaying a run never re-notifies
            },
        );
    },
);

// --- run control --------------------------------------------------------
const mode = ref<RunMode>("sequential");
const MODES: RunMode[] = ["sequential", "parallel", "parallel-stages"];
function cycleMode(): void {
    mode.value = MODES[(MODES.indexOf(mode.value) + 1) % MODES.length];
}
const canRun = computed(() => !runContext.value?.active);
const canCancel = computed(() => !!runContext.value?.active);
const busy = ref(false);

// --- env vars passed to triggered runs, persisted across reloads ---------
const ENV_KEY = "local-ci.run-env";
const envText = ref(localStorage.getItem(ENV_KEY) ?? "");
watch(envText, (v) => {
    try {
        localStorage.setItem(ENV_KEY, v);
    } catch {
        // storage may be blocked; the field still works for this session
    }
});

/** "K=V K2=V2" / comma-separated → ["K=V", ...]; null on a malformed entry. */
function parseEnv(): string[] | null {
    const entries = envText.value.split(/[\s,]+/).filter(Boolean);
    for (const e of entries) {
        if (!/^[A-Za-z_][A-Za-z0-9_]*=.*$/.test(e)) return null;
    }
    return entries;
}

async function trigger(req: {
    jobs?: string[];
    stages?: string[];
}): Promise<void> {
    if (busy.value) return;
    const env = parseEnv();
    if (env === null) {
        push("ERROR: ENV MUST BE KEY=VALUE PAIRS_", "error");
        return;
    }
    busy.value = true;
    try {
        const id = await triggerRun({ mode: mode.value, env, ...req });
        const what = req.jobs?.length
            ? `RERUNNING ${req.jobs.join(", ").toUpperCase()}`
            : req.stages?.length
              ? `RUNNING STAGE ${req.stages.join(", ").toUpperCase()}`
              : "PIPELINE_STARTED";
        push(`> ${what}_`, "accent");
        router.push(`/runs/${id}`);
    } catch (e) {
        push(`ERROR: ${e instanceof Error ? e.message : String(e)}`, "error");
    } finally {
        busy.value = false;
    }
}

const onRun = (): Promise<void> => trigger({});
const runStage = (stage: string): Promise<void> => trigger({ stages: [stage] });

// --- re-run failed jobs (uses the trigger API's jobs:[] selector) --------
const failedConfigNames = computed(() => [
    ...new Set(
        nodes.value
            .filter((n) => n.status === "failed")
            .map((n) => n.configName),
    ),
]);
const canRerun = computed(() => canRun.value && !busy.value);

const rerunJobs = (names: string[]): Promise<void> =>
    names.length === 0 ? Promise.resolve() : trigger({ jobs: names });

async function onCancel(): Promise<void> {
    if (!runId.value) return;
    try {
        await cancelRun(runId.value);
        push("ERROR: HALTED_BY_USER", "error");
    } catch (e) {
        push(`ERROR: ${e instanceof Error ? e.message : String(e)}`, "error");
    }
}

// --- inspector + log feed UI state --------------------------------------
const focusedJob = ref<string | null>(null);
const inspectorOpen = ref(false);
const openLogs = ref<string[]>([]);
const activeLog = ref<string | null>(null);
const logsMin = ref(false);

const focusedNode = computed(
    () => nodes.value.find((n) => n.name === focusedJob.value) ?? null,
);
const logAttached = computed(
    () => !!focusedJob.value && openLogs.value.includes(focusedJob.value),
);

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
    if (!openLogs.value.includes(name))
        openLogs.value = [...openLogs.value, name];
    activeLog.value = name;
    logsMin.value = false;
    push(`> TAILING ${name}_`, "accent");
}
function closeLog(name: string): void {
    const next = openLogs.value.filter((j) => j !== name);
    openLogs.value = next;
    if (activeLog.value === name)
        activeLog.value = next[next.length - 1] ?? null;
}
</script>

<template>
    <div class="col" data-test-id="pipeline-view">
        <!-- run control -->
        <div class="panel controls" data-test-id="run-controls">
            <span class="dim">MODE:</span>
            <button
                class="btn mode-btn"
                style="min-width: 11rem; text-align: center"
                data-test-id="mode-select"
                title="CYCLE_RUN_MODE"
                @click="cycleMode"
            >
                {{ mode.toUpperCase() }}
            </button>
            <button
                class="btn btn-accent"
                data-test-id="run-pipeline"
                :disabled="!canRun || busy"
                @click="onRun"
            >
                <Icon :name="busy ? 'spinner' : 'play'" :spin="busy" />
                {{ busy ? "STARTING..." : "RUN_PIPELINE" }}
            </button>
            <button
                class="btn btn-error"
                data-test-id="cancel-run"
                :disabled="!canCancel"
                @click="onCancel"
            >
                <Icon name="stop" /> STOP
            </button>
            <button
                v-if="failedConfigNames.length > 0 && canRun"
                class="btn btn-error"
                data-test-id="rerun-failed"
                :disabled="busy"
                :title="`RERUN: ${failedConfigNames.join(', ')}`"
                @click="rerunJobs(failedConfigNames)"
            >
                <Icon name="retry" /> RERUN_FAILED ({{
                    failedConfigNames.length
                }})
            </button>
            <span class="dim">ENV:</span>
            <input
                v-model="envText"
                type="text"
                class="env-input"
                data-test-id="env-input"
                placeholder="KEY=VALUE KEY2=VALUE2"
                spellcheck="false"
                title="Extra environment variables for triggered runs"
            />
            <span class="grow"></span>
            <span
                v-if="live.commit"
                class="dim git-ref"
                data-test-id="run-git-ref"
                :title="live.commit"
                ><Icon name="branch" /> {{ gitRef(live.commit, live.branch) }}</span
            >
            <span v-if="runId" class="dim" data-test-id="current-run-id"
                >RUN: {{ shortId(runId) }}</span
            >
            <button
                class="btn btn-sq"
                title="RELOAD_CONFIG"
                data-test-id="reload-config"
                @click="refreshConfig()"
            >
                <Icon name="refresh" /> RELOAD_CONFIG
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
                :can-run="canRerun"
                @focus="onFocus"
                @run-stage="runStage"
            />
            <Inspector
                v-if="inspectorOpen"
                :node="focusedNode"
                :nodes="nodes"
                :log-attached="logAttached"
                :can-rerun="canRerun"
                @close="inspectorOpen = false"
                @check-logs="checkLogs"
                @rerun="rerunJobs([$event])"
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

<style scoped>
.env-input {
    min-width: 16rem;
    text-transform: none; /* env values are case-sensitive */
    font-size: 1rem;
}
.git-ref {
    text-transform: none; /* branch names are case-sensitive */
}
</style>
