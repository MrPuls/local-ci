<script lang="ts">
  import type { LogLine } from '../lib/events';

  let { lines }: { lines: LogLine[] } = $props();

  let el = $state<HTMLPreElement | null>(null);

  // Container output chunks aren't line-aligned, so we present a single
  // interleaved console (job/diagnostic streams as the engine emits them).
  let text = $derived(lines.map((l) => l.data).join(''));

  // Auto-scroll to the bottom as new output arrives.
  $effect(() => {
    void text;
    if (el) el.scrollTop = el.scrollHeight;
  });
</script>

<div class="livelog-head">live output</div>
{#if text === ''}
  <p class="muted">Waiting for output…</p>
{:else}
  <pre class="log" bind:this={el}>{text}</pre>
{/if}

<style>
  .livelog-head {
    margin: 16px 0 6px;
    color: var(--muted);
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
</style>
