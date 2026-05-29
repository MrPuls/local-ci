<script lang="ts">
  import { listRuns, triggerRun } from '../lib/api';
  import type { Run } from '../lib/types';
  import { fmtDuration, fmtTime, shortId } from '../lib/format';
  import { navigate } from '../lib/router.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  let runs = $state<Run[]>([]);
  let error = $state<string | null>(null);
  let loading = $state(true);
  let mode = $state('sequential');
  let triggering = $state(false);

  $effect(() => {
    load();
  });

  async function load() {
    loading = true;
    error = null;
    try {
      runs = await listRuns();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  async function run() {
    triggering = true;
    error = null;
    try {
      const id = await triggerRun({ mode });
      navigate(`#/runs/${encodeURIComponent(id)}`);
    } catch (e) {
      error = (e as Error).message;
    } finally {
      triggering = false;
    }
  }
</script>

<div class="toolbar">
  <label>
    Mode
    <select bind:value={mode}>
      <option value="sequential">sequential</option>
      <option value="parallel">parallel</option>
      <option value="parallel-stages">parallel-stages</option>
    </select>
  </label>
  <button onclick={run} disabled={triggering}>{triggering ? 'Starting…' : 'Run pipeline'}</button>
  <span class="spacer"></span>
  <button onclick={load}>Refresh</button>
</div>

{#if loading}
  <p class="muted">Loading…</p>
{:else if error}
  <p class="error">{error}</p>
{:else if runs.length === 0}
  <p class="muted">No runs recorded yet. Run a pipeline with <code>local-ci run</code>.</p>
{:else}
  <table>
    <thead>
      <tr>
        <th>Run</th><th>Status</th><th>Mode</th><th>Started</th><th>Duration</th><th>Project</th>
      </tr>
    </thead>
    <tbody>
      {#each runs as run (run.id)}
        <tr class="clickable" onclick={() => navigate(`#/runs/${encodeURIComponent(run.id)}`)}>
          <td class="mono">{shortId(run.id)}</td>
          <td><StatusBadge status={run.status} /></td>
          <td>{run.mode}</td>
          <td>{fmtTime(run.startedAt)}</td>
          <td>{fmtDuration(run.durationMs)}</td>
          <td class="muted">{run.projectPath}</td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }
  .toolbar label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
  }
  .toolbar .spacer {
    flex: 1;
  }
</style>
