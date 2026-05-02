<script lang="ts">
  import { eventsStore } from '$lib/stores/events.svelte';

  let label = $derived(({
    open: 'live',
    connecting: 'connecting',
    error: 'reconnecting',
    closed: 'offline'
  } as const)[eventsStore.status]);

  let color = $derived(({
    open: 'var(--ok)',
    connecting: 'var(--warn)',
    error: 'var(--hot)',
    closed: 'var(--text-dim)'
  } as const)[eventsStore.status]);
</script>

<span class="dot" title={label}>
  <span class="bullet" style="background: {color}; box-shadow: 0 0 8px {color};"></span>
  <span class="label">{label}</span>
</span>

<style>
  .dot { display: inline-flex; align-items: center; gap: 6px; font-size: var(--fs-12); color: var(--text-muted); }
  .bullet {
    width: 8px; height: 8px; border-radius: 50%;
    transition: background var(--dur-med) var(--ease-out), box-shadow var(--dur-med) var(--ease-out);
  }
</style>
