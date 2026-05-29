<script lang="ts">
  import { router, startRouter } from './lib/router.svelte';
  import RunList from './views/RunList.svelte';
  import RunDetail from './views/RunDetail.svelte';

  startRouter();

  let route = $derived(router.route);
</script>

<header class="app-header">
  <h1><a href="#/">local-ci</a></h1>
  <span class="crumb">{route.name === 'detail' ? 'run detail' : 'runs'}</span>
</header>

<main>
  {#if route.name === 'detail'}
    <RunDetail id={route.id} />
  {:else}
    <RunList />
  {/if}
</main>
