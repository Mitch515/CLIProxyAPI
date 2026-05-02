<script lang="ts">
  import { tokenStore } from '$lib/stores/token.svelte';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { toastStore } from '$lib/stores/toasts.svelte';

  function logout() {
    tokenStore.clear();
    toastStore.push({ kind: 'info', title: 'Token cleared' });
    goto(`${base}/token`);
  }
</script>

<h1>Settings</h1>
<p class="sub">Dashboard preferences. Per-account warmup and labels live on each subscription card.</p>

<section class="card">
  <h3>Management token</h3>
  <p class="muted">Stored in browser localStorage and sent as <code>Authorization: Bearer …</code> on every request.</p>
  <button onclick={logout}>clear token (sign out)</button>
</section>

<section class="card">
  <h3>About</h3>
  <p class="muted">Fork of <a href="https://github.com/router-for-me/CLIProxyAPI" target="_blank" rel="noreferrer noopener">router-for-me/CLIProxyAPI</a> with per-account 5h / 7d window tracking and auto-warmup.</p>
</section>

<style>
  h1   { margin: 0 0 var(--s-1); font-size: var(--fs-24); }
  .sub { color: var(--text-muted); font-size: var(--fs-13); margin: 0 0 var(--s-5); }
  .card {
    background: var(--bg-elev-1); border: 1px solid var(--border);
    border-radius: var(--r-lg); padding: var(--s-5);
    margin-bottom: var(--s-4); max-width: 640px;
  }
  .card h3 { margin: 0 0 var(--s-2); font-size: var(--fs-16); }
  .muted   { color: var(--text-muted); margin: 0 0 var(--s-3); font-size: var(--fs-13); }
  code     { background: var(--bg-elev-2); padding: 2px 6px; border-radius: var(--r-sm); font-family: var(--font-mono); font-size: var(--fs-12); }
</style>
