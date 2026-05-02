<script lang="ts">
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { tokenStore } from '$lib/stores/token.svelte';
  import { accountsStore } from '$lib/stores/accounts.svelte';
  import { eventsStore } from '$lib/stores/events.svelte';

  let value = $state(tokenStore.value);
  let saving = $state(false);
  let err = $state<string | null>(null);

  async function save(e: Event) {
    e.preventDefault();
    saving = true;
    err = null;
    tokenStore.set(value.trim());
    try {
      await accountsStore.refresh();
      void eventsStore.start();
      goto(base || '/');
    } catch (e: any) {
      err = e?.message ?? 'failed to verify token';
      saving = false;
    }
  }
</script>

<div class="wrap">
  <form class="card" onsubmit={save}>
    <div class="brand">
      <span class="logo">●</span>
      <span class="name">CLIProxyAPI Dashboard</span>
    </div>
    <p class="hint">
      Paste your management secret. The dashboard sends it as a bearer token on every request.
    </p>
    <input type="password"
           autocomplete="off"
           spellcheck="false"
           placeholder="management secret"
           bind:value
           required />
    {#if err}<div class="err">{err}</div>{/if}
    <button class="primary" type="submit" disabled={saving || !value.trim()}>
      {saving ? 'verifying…' : 'continue'}
    </button>
  </form>
</div>

<style>
  .wrap {
    min-height: 100vh;
    display: grid; place-items: center;
    padding: var(--s-5);
  }
  .card {
    background: var(--bg-elev-1);
    border: 1px solid var(--border);
    border-radius: var(--r-xl);
    padding: var(--s-6);
    width: 100%; max-width: 420px;
    display: flex; flex-direction: column; gap: var(--s-4);
    box-shadow: 0 24px 80px rgba(0,0,0,0.4);
  }
  .brand { display: flex; align-items: center; gap: var(--s-2); font-weight: 600; font-size: var(--fs-16); }
  .logo  { color: var(--accent); font-size: 14px; }
  .hint  { margin: 0; color: var(--text-muted); font-size: var(--fs-13); }
  .err   { color: var(--hot); font-size: var(--fs-13); }
</style>
