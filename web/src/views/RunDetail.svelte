<script lang="ts">
  import { getRun, cancelRun } from '../lib/api';
  import type { Run, WireEvent } from '../lib/types';
  import { newLiveState, applyEvent, type LiveState } from '../lib/events';
  import { fmtDuration, fmtTime } from '../lib/format';
  import StatusBadge from '../components/StatusBadge.svelte';
  import JobLog from '../components/JobLog.svelte';
  import LiveLog from '../components/LiveLog.svelte';

  let { id }: { id: string } = $props();

  let run = $state<Run | null>(null);
  let error = $state<string | null>(null);
  let loading = $state(true);
  // Non-null while we drive the view from the live SSE stream (active runs).
  let live = $state<LiveState | null>(null);
  let selectedJob = $state<string | null>(null);
  let cancelling = $state(false);

  // Load the snapshot whenever the run id changes; go live if it's running.
  $effect(() => {
    void id;
    selectedJob = null;
    live = null;
    loadSnapshot();
  });

  async function loadSnapshot() {
    loading = true;
    error = null;
    try {
      const r = await getRun(id);
      run = r;
      if (r.status === 'running') {
        live = newLiveState();
      }
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  // Stream events while live; the stream replays from the start, so it fully
  // rebuilds job + log state. Closes on run_finished, id change, or unmount.
  $effect(() => {
    const liveState = live;
    if (!liveState) return;
    const es = new EventSource(`/api/runs/${encodeURIComponent(id)}/events`);
    es.onmessage = (ev) => {
      try {
        applyEvent(liveState, JSON.parse(ev.data) as WireEvent);
        if (liveState.finished) es.close();
      } catch {
        /* ignore a malformed frame */
      }
    };
    return () => es.close();
  });

  async function doCancel() {
    cancelling = true;
    try {
      await cancelRun(id);
    } catch (e) {
      error = (e as Error).message;
    } finally {
      cancelling = false;
    }
  }
</script>

<p><a href="#/">← All runs</a></p>

{#if loading}
  <p class="muted">Loading…</p>
{:else if error}
  <p class="error">{error}</p>
{:else if live}
  <!-- Live view: state driven entirely by the SSE stream. -->
  <h2 class="mono">{id}</h2>
  <dl class="meta">
    <dt>Status</dt>
    <dd><StatusBadge status={live.status} /></dd>
    <dt>Mode</dt>
    <dd>{live.mode || run?.mode || '–'}</dd>
    <dt>Duration</dt>
    <dd>{fmtDuration(live.durationMs)}</dd>
    {#if live.error}
      <dt>Error</dt>
      <dd class="error">{live.error}</dd>
    {/if}
  </dl>

  {#if !live.finished}
    <p><button onclick={doCancel} disabled={cancelling}>{cancelling ? 'Cancelling…' : 'Cancel run'}</button></p>
  {/if}

  {#if live.jobs.length > 0}
    <table>
      <thead>
        <tr><th>Job</th><th>Stage</th><th>Kind</th><th>Status</th><th>Duration</th></tr>
      </thead>
      <tbody>
        {#each live.jobs as job (job.name)}
          <tr>
            <td>{job.name}</td>
            <td>{job.stage}</td>
            <td class="muted">{job.execKind}</td>
            <td><StatusBadge status={job.status} /></td>
            <td>{fmtDuration(job.durationMs)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}

  <LiveLog lines={live.log} />
{:else if run}
  <!-- Finished run: cheap static view (snapshot + on-demand per-job log). -->
  <h2 class="mono">{run.id}</h2>
  <dl class="meta">
    <dt>Status</dt>
    <dd><StatusBadge status={run.status} /></dd>
    <dt>Mode</dt>
    <dd>{run.mode}</dd>
    <dt>Started</dt>
    <dd>{fmtTime(run.startedAt)}</dd>
    <dt>Duration</dt>
    <dd>{fmtDuration(run.durationMs)}</dd>
    <dt>Project</dt>
    <dd class="muted">{run.projectPath}</dd>
    <dt>Config</dt>
    <dd class="muted">{run.configPath}</dd>
    {#if run.error}
      <dt>Error</dt>
      <dd class="error">{run.error}</dd>
    {/if}
  </dl>

  {#if run.jobs && run.jobs.length > 0}
    <table>
      <thead>
        <tr><th>Job</th><th>Stage</th><th>Kind</th><th>Status</th><th>Duration</th><th></th></tr>
      </thead>
      <tbody>
        {#each run.jobs as job (job.name)}
          <tr>
            <td>{job.name}</td>
            <td>{job.stage}</td>
            <td class="muted">{job.execKind}</td>
            <td><StatusBadge status={job.status} /></td>
            <td>{fmtDuration(job.durationMs)}</td>
            <td><button onclick={() => (selectedJob = job.name)}>Log</button></td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <p class="muted">No jobs recorded.</p>
  {/if}

  <p><button onclick={() => (selectedJob = 'pipeline')}>Pipeline diagnostics</button></p>

  {#if selectedJob}
    <JobLog runId={run.id} job={selectedJob} />
  {/if}
{/if}
