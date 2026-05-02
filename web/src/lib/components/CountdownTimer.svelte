<script lang="ts">
  import { onMount } from 'svelte';
  import { tickStore } from '$lib/stores/tick.svelte';
  import { formatRemaining } from '$lib/utils/format';

  interface Props {
    resetAt?: string | null;
    exhausted?: boolean;
    prefix?: string;
  }

  let { resetAt, exhausted = false, prefix = 'resets in' }: Props = $props();

  onMount(() => {
    tickStore.attach();
    return () => tickStore.detach();
  });

  let target = $derived(resetAt ? Date.parse(resetAt) : NaN);
  let remaining = $derived(Number.isNaN(target) ? null : Math.max(0, target - tickStore.now));
  let display = $derived(remaining == null ? '—' : formatRemaining(remaining));
  let label = $derived.by(() => {
    if (remaining == null) return '—';
    if (exhausted && remaining > 0) return `resets in ${display}`;
    if (remaining === 0) return exhausted ? 'reset due' : prefix;
    return `${prefix} ${display}`;
  });
</script>

<span class="countdown tabular" class:exhausted>{label}</span>

<style>
  .countdown { color: var(--text-muted); font-size: var(--fs-13); }
  .countdown.exhausted { color: var(--hot); font-weight: 500; }
</style>
