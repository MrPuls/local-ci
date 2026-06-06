<script setup lang="ts">
import { onMounted } from 'vue';
import { useRouter } from 'vue-router';
import StatusTag from '@/components/StatusTag.vue';
import { useRuns } from '@/composables/useRuns';
import { useRunStatus } from '@/composables/useRunStatus';
import { fmtDateTime, fmtDuration, shortId } from '@/lib/format';
import type { Run } from '@/lib/types';

const router = useRouter();
const { runs, loading, error, refresh } = useRuns();
const { reset } = useRunStatus();

onMounted(() => {
  reset(); // no run "on screen" while browsing history
  refresh();
});

function open(run: Run): void {
  router.push(`/runs/${run.id}`);
}
</script>

<template>
  <section class="panel" data-test-id="history-view">
    <div class="panel-hd">
      <span>RUN_HISTORY</span>
      <span class="dim" style="font-weight: normal">{{ runs.length }} RUNS</span>
      <button class="log-ctl" style="margin-left: auto" data-test-id="history-refresh" @click="refresh()">
        ↻ REFRESH
      </button>
    </div>

    <div v-if="error" class="banner" data-test-id="history-error">
      <span class="error">ERROR: {{ error }}</span>
    </div>

    <div v-else-if="loading && runs.length === 0" class="empty">
      <span class="dim">&gt; LOADING_RUNS_</span><span class="blink accent">_</span>
    </div>

    <div v-else-if="runs.length === 0" class="empty" data-test-id="history-empty">
      <div class="dim" style="font-size: 1.4rem">&gt; NO_DATA_FOUND</div>
      <div class="dim" style="margin-top: 6px">TRIGGER A RUN FROM THE PIPELINE VIEW_</div>
    </div>

    <table v-else class="term" data-test-id="history-table">
      <thead>
        <tr>
          <th>STATUS</th>
          <th>RUN_ID</th>
          <th>MODE</th>
          <th>STARTED</th>
          <th>DURATION</th>
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
          <td class="dim">{{ fmtDateTime(run.startedAt) }}</td>
          <td>{{ fmtDuration(run.durationMs) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
