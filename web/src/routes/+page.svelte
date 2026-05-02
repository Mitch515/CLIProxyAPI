<script lang="ts">
  import { accountsStore } from '$lib/stores/accounts.svelte';
  import { toastStore } from '$lib/stores/toasts.svelte';
  import AccountCard from '$lib/components/AccountCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';

  let pinging = $state(false);

  async function pingAll() {
    if (pinging) return;
    pinging = true;
    try {
      const res = await accountsStore.fireWarmupAll();
      const ok = res.results.filter(r => r.ok).length;
      const total = res.results.length;
      toastStore.push({
        kind: ok > 0 ? 'success' : 'error',
        title: `Pinged ${total} accounts`,
        body: `${ok} succeeded · ${total - ok} failed`
      });
      await accountsStore.refresh();
    } catch (e: any) {
      toastStore.push({ kind: 'error', title: 'Ping all failed', body: e?.message ?? String(e) });
    } finally {
      pinging = false;
    }
  }
</script>

<div class="header">
  <div>
    <h1>Subscriptions</h1>
    <p class="sub">{accountsStore.list.length} connected · live updates via SSE</p>
  </div>
  <div class="actions">
    <button onclick={() => accountsStore.refresh()}>refresh</button>
    <button class="primary" disabled={pinging || accountsStore.list.length === 0} onclick={pingAll}>
      {pinging ? 'pinging…' : 'ping all'}
    </button>
  </div>
</div>

{#if accountsStore.loading && accountsStore.list.length === 0}
  <div class="grid">
    {#each [0,1,2,3,4,5] as _}
      <div class="skel"><Skeleton height="220px" radius="var(--r-lg)" /></div>
    {/each}
  </div>
{:else if accountsStore.error}
  <div class="empty error">
    <h2>Could not load accounts</h2>
    <p>{accountsStore.error}</p>
    <button onclick={() => accountsStore.refresh()}>retry</button>
  </div>
{:else if accountsStore.list.length === 0}
  <div class="empty">
    <h2>No accounts yet</h2>
    <p>Use the proxy CLI to log in a Claude, Codex, or Gemini account; it will appear here automatically within a minute.</p>
    <pre><code>./cli-proxy-api anthropic-login
./cli-proxy-api codex-login
./cli-proxy-api gemini-login</code></pre>
  </div>
{:else}
  <div class="grid">
    {#each accountsStore.list as a (a.id)}
      <AccountCard account={a} />
    {/each}
  </div>
{/if}

<style>
  .header {
    display: flex; align-items: flex-end; justify-content: space-between; gap: var(--s-4);
    margin-bottom: var(--s-6);
  }
  .actions { display: flex; gap: var(--s-2); }
  h1 { margin: 0; font-size: var(--fs-32); font-weight: 700; letter-spacing: -0.01em; }
  .sub { margin: var(--s-1) 0 0; color: var(--text-muted); font-size: var(--fs-13); }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: var(--s-3);
  }
  .skel { height: 140px; }

  .empty {
    background: var(--bg-elev-1);
    border: 1px dashed var(--border-strong);
    border-radius: var(--r-xl);
    padding: var(--s-7);
    text-align: center;
    color: var(--text-muted);
  }
  .empty h2 { color: var(--text); margin-top: 0; }
  .empty pre { display: inline-block; text-align: left; background: var(--bg-elev-2); padding: var(--s-3) var(--s-4); border-radius: var(--r-md); font-family: var(--font-mono); font-size: var(--fs-13); color: var(--text); margin-top: var(--s-4); }
  .empty.error { border-color: var(--hot); color: var(--hot); }
</style>
