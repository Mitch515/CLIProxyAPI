<script lang="ts">
  import { toastStore } from '$lib/stores/toasts.svelte';
</script>

<div class="host" aria-live="polite" aria-atomic="true">
  {#each toastStore.items as t (t.id)}
    <div class="toast {t.kind}" role="status">
      <div class="title">{t.title}</div>
      {#if t.body}<div class="body">{t.body}</div>{/if}
      <button class="close" onclick={() => toastStore.dismiss(t.id)} aria-label="Dismiss">×</button>
    </div>
  {/each}
</div>

<style>
  .host {
    position: fixed; right: var(--s-5); bottom: var(--s-5);
    display: flex; flex-direction: column; gap: var(--s-3);
    z-index: 100; max-width: 360px;
  }
  .toast {
    background: var(--bg-elev-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    padding: var(--s-3) var(--s-5) var(--s-3) var(--s-4);
    box-shadow: 0 12px 40px rgba(0,0,0,0.35);
    position: relative;
    animation: slide-up 220ms cubic-bezier(.16,1,.3,1);
  }
  .toast.success { border-color: rgba(43,212,164,0.45); }
  .toast.error   { border-color: rgba(255,59,59,0.45); }

  .title { font-weight: 600; font-size: var(--fs-13); }
  .body  { color: var(--text-muted); font-size: var(--fs-12); margin-top: 2px; }
  .close {
    position: absolute; top: 4px; right: 4px;
    background: none; border: 0; padding: 4px 8px;
    color: var(--text-dim); cursor: pointer; font-size: 18px; line-height: 1;
    border-radius: var(--r-sm);
  }
  .close:hover { color: var(--text); background: var(--bg-elev-3); }

  @keyframes slide-up {
    from { opacity: 0; transform: translateY(12px); }
    to   { opacity: 1; transform: translateY(0); }
  }
</style>
