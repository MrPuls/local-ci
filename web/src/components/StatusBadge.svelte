<script lang="ts">
  let { status }: { status: string } = $props();
</script>

<span class="badge {status}">{status}</span>

<style>
  .badge {
    display: inline-block;
    padding: 1px 8px;
    border-radius: 999px;
    font-size: 12px;
    font-weight: 600;
    border: 1px solid var(--border);
    color: var(--muted);
    text-transform: capitalize;
  }
  .badge.passed {
    color: #fff;
    background: color-mix(in srgb, var(--pass) 22%, transparent);
    border-color: var(--pass);
  }
  .badge.failed {
    color: #fff;
    background: color-mix(in srgb, var(--fail) 22%, transparent);
    border-color: var(--fail);
  }
  .badge.running {
    color: #fff;
    background: color-mix(in srgb, var(--run) 22%, transparent);
    border-color: var(--run);
  }
</style>
