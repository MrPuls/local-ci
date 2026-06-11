<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import Icon from '@/components/Icon.vue';
import StatusTag from '@/components/StatusTag.vue';
import { useRuns } from '@/composables/useRuns';
import { useRunStatus } from '@/composables/useRunStatus';
import { useSystem } from '@/composables/useSystem';
import { useToast } from '@/composables/useToast';
import JobTrends from '@/components/JobTrends.vue';
import { fmtBytes, fmtDateTime, fmtDuration, gitRef, shortId } from '@/lib/format';
import type { Run } from '@/lib/types';

const router = useRouter();
const {
  runs,
  total,
  offset,
  pageSize,
  loading,
  error,
  hasPrev,
  hasNext,
  refresh,
  nextPage,
  prevPage,
  remove,
  cleanup,
} = useRuns();
const { reset } = useRunStatus();
const { db, refresh: refreshSystem } = useSystem();
const { push } = useToast();

onMounted(() => {
  reset(); // no run "on screen" while browsing history
  refresh();
  refreshSystem();
});

const rangeStart = computed(() => (total.value === 0 ? 0 : offset.value + 1));
const rangeEnd = computed(() => offset.value + runs.value.length);

function open(run: Run): void {
  router.push(`/runs/${run.id}`);
}

const msg = (e: unknown) => (e instanceof Error ? e.message : String(e));

// --- per-row delete (two-step inline confirm, keeps the CRT immersion) ------
const pendingDelete = ref<string | null>(null);
const cleanupPending = ref(false);

function askDelete(id: string): void {
  pendingDelete.value = id;
  cleanupPending.value = false;
}
async function confirmDelete(run: Run): Promise<void> {
  try {
    await remove(run.id);
    push(`> DELETED ${shortId(run.id)}_`, 'accent');
  } catch (e) {
    push(`ERROR: ${msg(e)}`, 'error');
  } finally {
    pendingDelete.value = null;
  }
}

// --- bulk cleanup: keep the most recent page, drop the rest -----------------
function askCleanup(): void {
  cleanupPending.value = true;
  pendingDelete.value = null;
}
async function confirmCleanup(): Promise<void> {
  try {
    const n = await cleanup(pageSize.value);
    push(n > 0 ? `> PURGED ${n} OLD RUN${n === 1 ? '' : 'S'}_` : '> NOTHING TO PURGE_', 'accent');
  } catch (e) {
    push(`ERROR: ${msg(e)}`, 'error');
  } finally {
    cleanupPending.value = false;
  }
}

function goPrev(): void {
  pendingDelete.value = null;
  prevPage();
}
function goNext(): void {
  pendingDelete.value = null;
  nextPage();
}
</script>

<template>
  <div class="col" data-test-id="history-view">
  <section class="panel">
    <div class="panel-hd">
      <span>RUN_HISTORY</span>
      <span class="dim" style="font-weight: normal">{{ total }} RUNS</span>
      <span style="flex: 1"></span>
      <button
        v-if="!cleanupPending"
        class="log-ctl"
        data-test-id="history-cleanup"
        title="Delete all but the most recent page"
        :disabled="total <= pageSize"
        @click="askCleanup"
      >
        <Icon name="cross" /> CLEANUP
      </button>
      <button
        v-else
        class="log-ctl error"
        data-test-id="history-cleanup-confirm"
        title="Confirm: keep only the most recent page"
        @click="confirmCleanup"
      >
        <Icon name="check" /> PURGE OLD · KEEP {{ pageSize }}?
      </button>
      <button class="log-ctl" data-test-id="history-refresh" @click="refresh()">
        <Icon name="refresh" /> REFRESH
      </button>
    </div>

    <div class="db-info dim" v-if="db" data-test-id="db-info">
      <Icon name="folder" /> DB: <span class="alt">{{ db.path }}</span>
      <span class="dim"> · </span>{{ fmtBytes(db.sizeBytes) }}
    </div>

    <div v-if="error" class="banner" data-test-id="history-error">
      <span class="error">ERROR: {{ error }}</span>
    </div>

    <div v-else-if="loading && runs.length === 0" class="empty">
      <span class="dim">&gt; LOADING_RUNS_</span><span class="blink accent">_</span>
    </div>

    <div v-else-if="total === 0" class="empty" data-test-id="history-empty">
      <div class="dim" style="font-size: 1.4rem">&gt; NO_DATA_FOUND</div>
      <div class="dim" style="margin-top: 6px">TRIGGER A RUN FROM THE PIPELINE VIEW_</div>
    </div>

    <template v-else>
      <table class="term" data-test-id="history-table">
        <thead>
          <tr>
            <th>STATUS</th>
            <th>RUN_ID</th>
            <th>MODE</th>
            <th>GIT</th>
            <th>STARTED</th>
            <th>DURATION</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="run in runs"
            :key="run.id"
            class="clickable"
            :data-test-id="`history-row-${run.id}`"
            @click="open(run)"
          >
            <td><StatusTag :status="run.status" /></td>
            <td><span class="link">{{ shortId(run.id) }}</span></td>
            <td class="alt">{{ run.mode.toUpperCase() }}</td>
            <td class="git-cell" :class="{ dim: !run.commit }" :title="run.commit ?? ''">
              {{ gitRef(run.commit, run.branch) }}
            </td>
            <td class="dim">{{ fmtDateTime(run.startedAt) }}</td>
            <td>{{ fmtDuration(run.durationMs) }}</td>
            <td class="del-cell" @click.stop>
              <button
                v-if="pendingDelete === run.id"
                class="del-btn error"
                :data-test-id="`history-delete-confirm-${run.id}`"
                title="Confirm delete"
                @click="confirmDelete(run)"
              >
                <Icon name="check" /> DELETE?
              </button>
              <button
                v-else
                class="del-btn"
                :data-test-id="`history-delete-${run.id}`"
                title="Delete this run"
                @click="askDelete(run.id)"
              >
                <Icon name="cross" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <div class="pager" data-test-id="history-pager">
        <button
          class="btn btn-sq"
          data-test-id="history-prev"
          :disabled="!hasPrev"
          title="Newer"
          @click="goPrev"
        >
          <Icon name="chevron-left" />
        </button>
        <span class="dim">{{ rangeStart }}–{{ rangeEnd }} OF {{ total }}</span>
        <button
          class="btn btn-sq"
          data-test-id="history-next"
          :disabled="!hasNext"
          title="Older"
          @click="goNext"
        >
          <Icon name="chevron-right" />
        </button>
      </div>
    </template>
  </section>

  <JobTrends />
  </div>
</template>

<style scoped>
.git-cell {
  text-transform: none; /* branch names are case-sensitive */
}
.db-info {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
  padding: 0.5rem 0;
  font-size: var(--fs-small);
  border-bottom: 1px dashed var(--term-dim);
  margin-bottom: 0.4rem;
}
.del-cell {
  text-align: right;
  width: 1%;
  white-space: nowrap;
}
.del-btn {
  background: transparent;
  border: 1px solid var(--term-dim);
  color: var(--term-dim);
  font-family: inherit;
  font-size: var(--fs-small);
  letter-spacing: 1px;
  padding: 0.1rem 0.45rem;
  cursor: pointer;
}
.del-btn:hover {
  color: var(--term-error);
  border-color: var(--term-error);
}
.del-btn.error {
  color: var(--term-error);
  border-color: var(--term-error);
  text-shadow: 0 0 6px var(--term-glow-error);
}
.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  padding: 0.7rem 0 0.2rem;
}
</style>
