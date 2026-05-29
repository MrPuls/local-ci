<script lang="ts">
  import { jobLog } from '../lib/api';

  let { runId, job }: { runId: string; job: string } = $props();

  let text = $state('');
  let error = $state<string | null>(null);
  let loading = $state(true);

  // Reload whenever the selected run or job changes.
  $effect(() => {
    void runId;
    void job;
    load();
  });

  async function load() {
    loading = true;
    error = null;
    try {
      text = await jobLog(runId, job);
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="joblog">
  <div class="joblog-head"><strong>{job}</strong> log</div>
  {#if loading}
    <p class="muted">Loading…</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if text.trim() === ''}
    <p class="muted">(empty)</p>
  {:else}
    <pre class="log">{text}</pre>
  {/if}
</div>

<style>
  .joblog-head {
    margin: 16px 0 6px;
  }
</style>
