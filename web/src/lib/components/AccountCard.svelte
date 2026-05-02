<script lang="ts">
  import type { Account, WindowState } from '$lib/api/types';
  import { base } from '$app/paths';
  import { goto } from '$app/navigation';
  import { accountsStore } from '$lib/stores/accounts.svelte';
  import { toastStore } from '$lib/stores/toasts.svelte';
  import { tickStore } from '$lib/stores/tick.svelte';
  import { onMount } from 'svelte';
  import ProviderLogo from './ProviderLogo.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import { compactNumber, pct, formatRemaining } from '$lib/utils/format';

  interface Props { account: Account; }
  let { account }: Props = $props();

  onMount(() => {
    tickStore.attach();
    return () => tickStore.detach();
  });

  // Show every window the backend reports for this account, in a stable order.
  let windowEntries = $derived.by(() => {
    const order = ['5h', '7d', '1d', '1m', '1h'];
    const seen = new Set<string>();
    const out: Array<[string, WindowState]> = [];
    for (const k of order) {
      if (account.windows?.[k as keyof typeof account.windows]) {
        out.push([k, account.windows[k as keyof typeof account.windows] as WindowState]);
        seen.add(k);
      }
    }
    for (const [k, v] of Object.entries(account.windows ?? {})) {
      if (!seen.has(k) && v) out.push([k, v as WindowState]);
    }
    return out;
  });

  let anyExhausted = $derived(windowEntries.some(([, w]) => w.exhausted));
  let warmupArmed = $derived(anyExhausted && account.warmup_enabled);
  let displayName = $derived(account.label || account.email || account.id);

  function remaining(w: WindowState): string {
    if (!w.reset_at) return '';
    const ms = Date.parse(w.reset_at) - tickStore.now;
    if (Number.isNaN(ms) || ms <= 0) return w.exhausted ? 'reset due' : '';
    return `↻ ${formatRemaining(ms)}`;
  }

  async function fireWarmup(e: Event) {
    e.stopPropagation();
    try {
      await accountsStore.fireWarmup(account.id);
      toastStore.push({ kind: 'success', title: 'Warmup queued', body: displayName });
    } catch (err: any) {
      toastStore.push({ kind: 'error', title: 'Warmup failed', body: err?.message ?? String(err) });
    }
  }

  function open() { goto(`${base}/accounts/${encodeURIComponent(account.id)}`); }
</script>

<button class="card" onclick={open} type="button">
  <header>
    <ProviderLogo provider={account.provider} size={20} />
    <div class="ident">
      <div class="name">{displayName}</div>
      <div class="meta">
        <span class="provider">{account.provider}</span>
        {#if account.warmup_model}
          <span class="dot">·</span>
          <span class="model" title="Warmup model">{account.warmup_model}</span>
        {/if}
      </div>
    </div>
    {#if anyExhausted}
      {#if warmupArmed}
        <span class="pill armed" title="Warmup will fire on reset">armed</span>
      {:else}
        <span class="pill exhausted">exhausted</span>
      {/if}
    {:else}
      <span class="pill ok">ok</span>
    {/if}
  </header>

  <div class="windows">
    {#each windowEntries as [key, w] (key)}
      <div class="row">
        <span class="key tabular">{key}</span>
        <ProgressBar value={w.pct_used ?? 0} pulse />
        <span class="pct tabular">{pct(w.pct_used ?? 0)}</span>
        <span class="reset tabular" class:exh={w.exhausted}>{remaining(w)}</span>
      </div>
    {/each}
    {#if windowEntries.length === 0}
      <div class="row empty">
        <span class="key">—</span>
        <span class="muted">no usage data yet · fire a ping or wait for traffic</span>
      </div>
    {/if}
  </div>

  <footer>
    {#if windowEntries.some(([, w]) => (w.tokens_used ?? 0) > 0)}
      {@const totalTok = windowEntries.reduce((s, [, w]) => s + (w.tokens_used ?? 0), 0)}
      <span class="muted tabular">{compactNumber(totalTok)} tok this window</span>
    {:else}
      <span class="muted">no recent traffic</span>
    {/if}
    <span class="grow"></span>
    <span class="ping" onclick={fireWarmup} role="button" tabindex="0"
          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') fireWarmup(e); }}>
      ping
    </span>
  </footer>
</button>

<style>
  .card {
    text-align: left;
    background: var(--bg-elev-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: var(--s-3) var(--s-4);
    display: flex; flex-direction: column; gap: var(--s-3);
    cursor: pointer;
    transition: border-color var(--dur-fast) var(--ease-out),
                box-shadow var(--dur-med) var(--ease-out);
    width: 100%;
    color: inherit;
    font: inherit;
  }
  .card:hover {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent), 0 0 18px var(--accent-glow);
  }

  header {
    display: flex; align-items: center; gap: var(--s-2);
  }
  .ident { flex: 1; min-width: 0; }
  .name {
    font-weight: 600;
    font-size: var(--fs-13);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .meta {
    color: var(--text-muted);
    font-size: 11px;
    display: flex; gap: 4px; align-items: center;
    margin-top: 1px;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .provider { text-transform: lowercase; }
  .model { font-family: var(--font-mono); font-size: 10.5px; }
  .dot { opacity: 0.5; }

  .pill {
    flex-shrink: 0;
    font-size: 10.5px;
    padding: 2px 7px;
    border-radius: var(--r-pill);
    text-transform: lowercase;
    border: 1px solid transparent;
    background: var(--bg-elev-2);
    color: var(--text-muted);
    line-height: 1.4;
  }
  .pill.ok { color: var(--ok); border-color: rgba(43,212,164,0.25); }
  .pill.exhausted { color: var(--exhausted); border-color: rgba(255,59,59,0.3); }
  .pill.armed { color: var(--warmup-armed); border-color: rgba(43,212,164,0.4); background: rgba(43,212,164,0.08); }

  .windows { display: flex; flex-direction: column; gap: 6px; }
  .row {
    display: grid;
    grid-template-columns: 22px 1fr 38px auto;
    align-items: center;
    gap: var(--s-2);
    font-size: 11px;
  }
  .row.empty { grid-template-columns: 22px 1fr; color: var(--text-dim); font-style: italic; }
  .key { color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.04em; font-weight: 500; }
  .pct { text-align: right; color: var(--text); font-weight: 500; }
  .reset { color: var(--text-dim); white-space: nowrap; min-width: 56px; text-align: right; }
  .reset.exh { color: var(--hot); }
  .muted { color: var(--text-dim); }

  footer {
    display: flex; align-items: center; gap: var(--s-2);
    font-size: 10.5px; color: var(--text-muted);
    border-top: 1px solid var(--border);
    padding-top: var(--s-2);
  }
  .grow { flex: 1; }
  .ping {
    color: var(--accent);
    cursor: pointer;
    padding: 2px 8px;
    border-radius: var(--r-sm);
    transition: background var(--dur-fast) var(--ease-out);
  }
  .ping:hover { background: rgba(124,92,255,0.12); color: var(--accent-hover); }
</style>
