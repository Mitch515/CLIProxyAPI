<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { tokenStore } from '$lib/stores/token.svelte';
  import { eventsStore } from '$lib/stores/events.svelte';
  import { accountsStore } from '$lib/stores/accounts.svelte';
  import Toast from '$lib/components/Toast.svelte';
  import StatusDot from '$lib/components/StatusDot.svelte';

  let { children } = $props();

  let isTokenScreen = $derived(page.url.pathname.endsWith('/token'));

  onMount(() => {
    if (!tokenStore.hasToken && !isTokenScreen) {
      goto(`${base}/token`, { replaceState: true });
      return;
    }
    if (tokenStore.hasToken) {
      void accountsStore.refresh();
      void eventsStore.start();
    }
    return () => eventsStore.stop();
  });
</script>

{#if isTokenScreen}
  {@render children()}
{:else}
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="logo">●</span>
        <span class="name">CLIProxy</span>
      </div>
      <nav>
        <a href="{base}" class:active={page.url.pathname === base || page.url.pathname === `${base}/`}>Overview</a>
        <a href="{base}/settings" class:active={page.url.pathname.startsWith(`${base}/settings`)}>Settings</a>
      </nav>
      <div class="foot">
        <StatusDot />
      </div>
    </aside>
    <main>
      {@render children()}
    </main>
  </div>
{/if}

<Toast />

<style>
  .shell { display: grid; grid-template-columns: 220px 1fr; min-height: 100vh; }
  .sidebar {
    border-right: 1px solid var(--border);
    background: var(--bg);
    padding: var(--s-5) var(--s-4);
    display: flex; flex-direction: column;
  }
  .brand {
    display: flex; align-items: center; gap: var(--s-2);
    font-weight: 600; font-size: var(--fs-16);
    margin-bottom: var(--s-6);
  }
  .logo { color: var(--accent); font-size: 14px; }
  nav { display: flex; flex-direction: column; gap: 2px; }
  nav a {
    display: block;
    padding: 8px 12px;
    border-radius: var(--r-md);
    color: var(--text-muted);
    transition: background var(--dur-fast) var(--ease-out), color var(--dur-fast) var(--ease-out);
  }
  nav a:hover { background: var(--bg-elev-1); color: var(--text); }
  nav a.active { background: var(--bg-elev-2); color: var(--text); }
  .foot { margin-top: auto; padding-top: var(--s-4); }

  main { padding: var(--s-6) var(--s-7); max-width: 1400px; }

  @media (max-width: 720px) {
    .shell { grid-template-columns: 1fr; }
    .sidebar { display: none; }
    main { padding: var(--s-4); }
  }
</style>
