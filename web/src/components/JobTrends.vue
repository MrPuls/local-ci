<script setup lang="ts">
import { onMounted, ref } from 'vue';
import Icon from './Icon.vue';
import { getJobStats } from '@/lib/api';
import { fmtDuration } from '@/lib/format';
import type { JobSample, JobStat } from '@/lib/types';

// JOB_TRENDS — per-job duration sparklines + health signals over the recent
// runs of this project. The sparkline is built from block characters (one per
// run, height = duration relative to the job's max, color = status): the most
// CRT-native chart there is.

const stats = ref<JobStat[]>([]);
const windowSize = ref(20);
const loading = ref(false);
const error = ref<string | null>(null);

async function refresh(): Promise<void> {
  loading.value = true;
  error.value = null;
  try {
    const res = await getJobStats(windowSize.value);
    stats.value = res.jobs;
    windowSize.value = res.window;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}
onMounted(refresh);
defineExpose({ refresh });

const BLOCKS = ['▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'];

function bar(sample: JobSample, stat: JobStat): string {
  if (sample.status === 'running') return '·';
  if (stat.maxMs <= 0) return BLOCKS[0];
  const level = Math.min(7, Math.round((sample.durationMs / stat.maxMs) * 7));
  return BLOCKS[level];
}

function barClass(sample: JobSample): string {
  if (sample.status === 'failed') return 'error';
  if (sample.status === 'running') return 'dim';
  return '';
}

function barTitle(sample: JobSample): string {
  return `${sample.status.toUpperCase()} · ${fmtDuration(sample.durationMs)} · run ${sample.runId}`;
}

const pct = (rate: number): string => `${Math.round(rate * 100)}%`;
</script>

<template>
  <section class="panel" data-test-id="job-trends">
    <div class="panel-hd">
      <span>JOB_TRENDS</span>
      <span class="dim" style="font-weight: normal">LAST {{ windowSize }} RUNS · THIS PROJECT</span>
      <span style="flex: 1"></span>
      <button class="log-ctl" data-test-id="trends-refresh" @click="refresh()">
        <Icon name="refresh" /> REFRESH
      </button>
    </div>

    <div v-if="error" class="banner"><span class="error">ERROR: {{ error }}</span></div>

    <div v-else-if="loading && stats.length === 0" class="empty">
      <span class="dim">&gt; CRUNCHING_NUMBERS_</span><span class="blink accent">_</span>
    </div>

    <div v-else-if="stats.length === 0" class="empty" data-test-id="trends-empty">
      <div class="dim" style="font-size: 1.3rem">&gt; NO_TREND_DATA</div>
      <div class="dim" style="margin-top: 6px">RUN THE PIPELINE A FEW TIMES TO BUILD HISTORY_</div>
    </div>

    <table v-else class="term" data-test-id="trends-table">
      <thead>
        <tr>
          <th>JOB</th>
          <th>DURATION_TREND</th>
          <th>AVG</th>
          <th>PASS_RATE</th>
          <th>HEALTH</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="stat in stats" :key="stat.name" :data-test-id="`trend-${stat.name}`">
          <td class="job-name">{{ stat.name }}</td>
          <td class="spark">
            <span
              v-for="(sample, i) in stat.samples"
              :key="i"
              :class="barClass(sample)"
              :title="barTitle(sample)"
              >{{ bar(sample, stat) }}</span
            >
          </td>
          <td>{{ fmtDuration(stat.avgMs) }}</td>
          <td :class="{ error: stat.passRate < 1, alt: stat.passRate === 1 }">
            {{ pct(stat.passRate) }}
          </td>
          <td>
            <span v-if="stat.flaky" class="chip error soft-pulse" title="Both passes and failures in the window">
              <Icon name="warning" /> INTERMITTENT
            </span>
            <span v-else-if="stat.passRate === 0" class="chip error">BROKEN</span>
            <span v-else class="chip dim">STABLE</span>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
.job-name {
  text-transform: none; /* job names are case-sensitive config keys */
}
.spark {
  font-size: 1.25rem;
  letter-spacing: 2px;
  line-height: 1;
  white-space: nowrap;
}
.spark span {
  cursor: help;
}
</style>
