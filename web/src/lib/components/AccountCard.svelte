<script lang="ts">
  import type { Account } from '$lib/api/types';
  import { base } from '$app/paths';
  import { goto } from '$app/navigation';
  import { accountsStore } from '$lib/stores/accounts.svelte';
  import { toastStore } from '$lib/stores/toasts.svelte';
  import ProgressRing from './ProgressRing.svelte';
  import CountdownTimer from './CountdownTimer.svelte';
  import ProviderLogo from './ProviderLogo.svelte';
  import { compactNumber, pct } from '$lib/utils/format';

  interface Props { account: Account; }
  let { account }: Props = $props();

  let w5 = $derived(account.windows?.['5h'] ?? { pct_used: 0, exhausted: false });
  let w7 = $derived(account.windows?.['7d'] ?? { pct_used: 0, exhausted: false });
  let anyExhausted = $derived(Boolean(w5.exhausted || w7.exhausted));
  let warmupArmed = $derived(anyExhausted && account.warmup_enabled);

  async function fireWarmup(e: Event) {
    e.stopPropagation();
    try {
      await accountsStore.fireWarmup(account.id);
      toastStore.push({ kind: 'success', title: 'Warmup queued', body: account.label || account.id });
    } catch (err: any) {
      toastStore.push({ kind: 'error', title: 'Warmup failed', body: err?.message ?? String(err) });
    }
  }

  function open() {
    goto(`${base}/accounts/${encodeURIComponent(account.id)}`);
  }
</script>

<button class="card" onclick={open} type="button">
  <div class="head">
    <ProviderLogo provider={account.provider} />
    <div class="ident">
      <div class="label">{account.label || account.email || account.provider}</div>
      <div class="email">{account.email ?? ''}</div>
    </div>
    <div class="status">
      {#if w5.exhausted || w7.exhausted}
        {#if warmupArmed}
          <span class="pill armed" title="Warmup will fire on reset">⚡ armed</span>
        {:else}
          <span class="pill exhausted">exhausted</span>
        {/if}
      {:else}
        <span class="pill ok">active</span>
      {/if}
    </div>
  </div>

  <div class="rings">
    <div class="ring-block">
      <ProgressRing value={w5.pct_used} label="5h" sublabel={pct(w5.pct_used)} pulse />
      <CountdownTimer resetAt={w5.reset_at} exhausted={w5.exhausted} />
      <div class="tokens">{w5.tokens_used != null ? `${compactNumber(w5.tokens_used)} tok` : ''}</div>
    </div>
    <div class="ring-block">
      <ProgressRing value={w7.pct_used} label="7d" sublabel={pct(w7.pct_used)} pulse />
      <CountdownTimer resetAt={w7.reset_at} exhausted={w7.exhausted} />
      <div class="tokens">{w7.tokens_used != null ? `${compactNumber(w7.tokens_used)} tok` : ''}</div>
    </div>
  </div>

  <div class="foot">
    <span class="provider">{account.provider}</span>
    <span class="warmup-toggle" class:on={account.warmup_enabled}>warmup {account.warmup_enabled ? 'on' : 'off'}</span>
    <span class="grow"></span>
    <span class="warmup-btn" onclick={fireWarmup} role="button" tabindex="0"
          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') fireWarmup(e); }}>
      ping now
    </span>
  </div>
</button>

<style>
  .card {
    text-align: left;
    background: var(--bg-elev-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    padding: var(--s-5);
    display: flex; flex-direction: column; gap: var(--s-4);
    cursor: pointer;
    transition: border-color var(--dur-fast) var(--ease-out),
                box-shadow var(--dur-med) var(--ease-out),
                transform var(--dur-fast) var(--ease-out);
    width: 100%;
    color: inherit;
    font: inherit;
  }
  .card:hover {
    border-color: var(--border-strong);
    box-shadow: 0 0 0 1px var(--accent), 0 0 24px var(--accent-glow);
    transform: translateY(-1px);
  }
  .head { display: flex; align-items: center; gap: var(--s-3); }
  .ident { flex: 1; min-width: 0; }
  .label { font-weight: 600; font-size: var(--fs-16); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .email { color: var(--text-muted); font-size: var(--fs-13); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .pill {
    font-size: var(--fs-12);
    padding: 3px 10px;
    border-radius: var(--r-pill);
    text-transform: lowercase;
    letter-spacing: 0.02em;
    border: 1px solid transparent;
    background: var(--bg-elev-2);
    color: var(--text-muted);
  }
  .pill.ok { color: var(--ok); border-color: rgba(43,212,164,0.25); }
  .pill.exhausted { color: var(--exhausted); border-color: rgba(255,59,59,0.3); }
  .pill.armed { color: var(--warmup-armed); border-color: rgba(43,212,164,0.4); background: rgba(43,212,164,0.08); }

  .rings {
    display: flex; gap: var(--s-6); justify-content: center;
    padding: var(--s-3) 0 var(--s-2) 0;
  }
  .ring-block { display: flex; flex-direction: column; align-items: center; gap: 4px; }
  .tokens { font-size: var(--fs-12); color: var(--text-dim); }

  .foot {
    display: flex; align-items: center; gap: var(--s-3);
    font-size: var(--fs-12); color: var(--text-muted);
    border-top: 1px solid var(--border);
    padding-top: var(--s-3);
  }
  .grow { flex: 1; }
  .provider { text-transform: lowercase; letter-spacing: 0.02em; }
  .warmup-toggle.on { color: var(--ok); }
  .warmup-btn {
    color: var(--accent);
    cursor: pointer;
    padding: 2px 8px;
    border-radius: var(--r-sm);
    transition: background var(--dur-fast) var(--ease-out);
  }
  .warmup-btn:hover { background: rgba(124,92,255,0.1); color: var(--accent-hover); }
</style>
