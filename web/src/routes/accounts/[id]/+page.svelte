<script lang="ts">
  import { page } from '$app/state';
  import { base } from '$app/paths';
  import { accountsStore } from '$lib/stores/accounts.svelte';
  import { onMount } from 'svelte';
  import ProgressRing from '$lib/components/ProgressRing.svelte';
  import CountdownTimer from '$lib/components/CountdownTimer.svelte';
  import ProviderLogo from '$lib/components/ProviderLogo.svelte';
  import { compactNumber, pct } from '$lib/utils/format';
  import { toastStore } from '$lib/stores/toasts.svelte';

  let id = $derived(page.params.id);
  let account = $derived(id ? accountsStore.byId.get(id) : undefined);
  let w5 = $derived(account?.windows?.['5h']);
  let w7 = $derived(account?.windows?.['7d']);

  onMount(() => {
    if (id) accountsStore.refreshOne(id);
  });

  let detecting = $state(false);
  let deleting = $state(false);

  async function fireWarmup() {
    if (!id) return;
    try {
      await accountsStore.fireWarmup(id);
      toastStore.push({ kind: 'success', title: 'Warmup queued' });
    } catch (e: any) {
      toastStore.push({ kind: 'error', title: 'Warmup failed', body: e?.message });
    }
  }

  async function toggleWarmup() {
    if (!account || !id) return;
    try {
      await accountsStore.patch(id, { warmup_enabled: !account.warmup_enabled });
    } catch (e: any) {
      toastStore.push({ kind: 'error', title: 'Could not update', body: e?.message });
    }
  }

  async function renameLabel() {
    if (!account || !id) return;
    const next = prompt('New label', account.label ?? '');
    if (next === null) return;
    try {
      await accountsStore.patch(id, { label: next });
    } catch (e: any) {
      toastStore.push({ kind: 'error', title: 'Could not rename', body: e?.message });
    }
  }

  async function autoDetect() {
    if (!id || detecting) return;
    detecting = true;
    try {
      const result = await accountsStore.autoDetect(id);
      if (result.winner) {
        toastStore.push({ kind: 'success', title: `Auto-detected ${result.winner}`, body: `${result.attempts.length} attempts` });
      } else if (result.token_expired) {
        toastStore.push({ kind: 'error', title: 'Token expired', body: 'Re-OAuth this account or delete it' });
      } else {
        toastStore.push({ kind: 'error', title: 'No working model found', body: `${result.attempts.length} models tried` });
      }
    } catch (e: any) {
      toastStore.push({ kind: 'error', title: 'Auto-detect failed', body: e?.message });
    } finally {
      detecting = false;
    }
  }

  async function removeAccount() {
    if (!account || !id) return;
    if (!confirm(`Delete ${account.label || account.email || id}?\n\nThis removes the auth file from disk. Re-login required to bring it back.`)) return;
    deleting = true;
    try {
      await accountsStore.remove(id);
      toastStore.push({ kind: 'success', title: 'Account deleted' });
      goto(base || '/');
    } catch (e: any) {
      toastStore.push({ kind: 'error', title: 'Delete failed', body: e?.message });
      deleting = false;
    }
  }
</script>

<a class="back" href={base || '/'}>← all subscriptions</a>

{#if !account}
  <p class="muted">Loading…</p>
{:else}
  <header class="head">
    <ProviderLogo provider={account.provider} size={36} />
    <div class="ident">
      <h1>{account.label || account.email || account.provider}</h1>
      <div class="meta">{account.email ?? ''} · {account.provider}</div>
    </div>
    <div class="actions">
      <button onclick={renameLabel}>rename</button>
      <button onclick={toggleWarmup}>{account.warmup_enabled ? 'disable warmup' : 'enable warmup'}</button>
      <button onclick={autoDetect} disabled={detecting}>{detecting ? 'detecting…' : 'auto-detect model'}</button>
      <button class="danger" onclick={removeAccount} disabled={deleting}>{deleting ? 'deleting…' : 'delete'}</button>
      <button class="primary" onclick={fireWarmup}>fire warmup ping</button>
    </div>
  </header>

  <section class="windows">
    {#each [['5h', w5], ['7d', w7]] as [label, w] (label)}
      <div class="window">
        <ProgressRing value={w?.pct_used ?? 0} label={label} sublabel={pct(w?.pct_used ?? 0)} size={140} stroke={10} pulse />
        <div class="info">
          <div class="row">
            <span class="k">used</span>
            <span class="v tabular">{w?.tokens_used != null ? compactNumber(w.tokens_used) : '—'}</span>
          </div>
          <div class="row">
            <span class="k">limit</span>
            <span class="v tabular">{w?.tokens_limit ? compactNumber(w.tokens_limit) : '—'}</span>
          </div>
          <div class="row">
            <span class="k">resets</span>
            <span class="v"><CountdownTimer resetAt={w?.reset_at} exhausted={w?.exhausted ?? false} /></span>
          </div>
          <div class="row">
            <span class="k">status</span>
            <span class="v">{w?.exhausted ? 'exhausted' : 'available'}</span>
          </div>
        </div>
      </div>
    {/each}
  </section>

  {#if account.last_warmup}
    <section class="card">
      <h3>Last warmup</h3>
      <div class="row"><span class="k">model</span><span class="v">{account.last_warmup.model}</span></div>
      <div class="row"><span class="k">trigger</span><span class="v">{account.last_warmup.trigger}</span></div>
      <div class="row"><span class="k">result</span><span class="v">{account.last_warmup.ok ? 'ok' : 'failed'}</span></div>
      <div class="row"><span class="k">when</span><span class="v">{new Date(account.last_warmup.fired_at).toLocaleString()}</span></div>
      {#if account.last_warmup.error}
        <pre class="err">{account.last_warmup.error}</pre>
      {/if}
    </section>
  {/if}
{/if}

<style>
  .back { color: var(--text-muted); font-size: var(--fs-13); display: inline-block; margin-bottom: var(--s-4); }
  .back:hover { color: var(--text); }
  .head { display: flex; align-items: center; gap: var(--s-4); margin-bottom: var(--s-6); }
  .ident { flex: 1; }
  h1 { margin: 0; font-size: var(--fs-24); }
  .meta { color: var(--text-muted); font-size: var(--fs-13); }
  .actions { display: flex; gap: var(--s-2); }

  .windows { display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: var(--s-5); }
  .window {
    background: var(--bg-elev-1); border: 1px solid var(--border);
    border-radius: var(--r-lg); padding: var(--s-5);
    display: flex; align-items: center; gap: var(--s-5);
  }
  .info { flex: 1; display: flex; flex-direction: column; gap: var(--s-2); }
  .row { display: flex; justify-content: space-between; gap: var(--s-3); font-size: var(--fs-13); }
  .k { color: var(--text-muted); text-transform: lowercase; letter-spacing: 0.02em; }
  .v { color: var(--text); }

  .card {
    background: var(--bg-elev-1); border: 1px solid var(--border);
    border-radius: var(--r-lg); padding: var(--s-5); margin-top: var(--s-5);
  }
  .err { color: var(--hot); font-family: var(--font-mono); font-size: var(--fs-12); white-space: pre-wrap; }
  .muted { color: var(--text-muted); }
  .actions button.danger {
    color: var(--hot);
    border-color: rgba(255, 107, 107, 0.4);
  }
  .actions button.danger:hover {
    background: rgba(255, 107, 107, 0.1);
    border-color: var(--hot);
  }
</style>
