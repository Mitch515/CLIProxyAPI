<script lang="ts">
  interface Props { provider: string; size?: number; }
  let { provider, size = 22 }: Props = $props();
  let key = $derived((provider ?? '').toLowerCase());
  let initial = $derived((provider?.[0] ?? '?').toUpperCase());
  // Distinct hue per provider so even unknown providers get a stable color.
  let hue = $derived.by(() => {
    let h = 0;
    for (const c of key) h = (h * 31 + c.charCodeAt(0)) >>> 0;
    return h % 360;
  });
</script>

<span class="logo" style="--size: {size}px; --hue: {hue};" title={provider}>
  {#if key === 'claude' || key === 'anthropic'}
    <!-- Anthropic asterisk (simplified) -->
    <svg viewBox="0 0 24 24" width={size} height={size} fill="#d97757">
      <path d="M12 2 L13.6 8.5 L20 10.1 L13.6 11.7 L12 18.2 L10.4 11.7 L4 10.1 L10.4 8.5 Z"/>
    </svg>
  {:else if key === 'codex' || key === 'openai'}
    <svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="#10a37f" stroke-width="1.6">
      <circle cx="12" cy="12" r="9" />
      <path d="M7 12 H17 M12 7 V17" />
    </svg>
  {:else if key === 'gemini' || key === 'gemini-cli' || key === 'aistudio' || key === 'antigravity' || key === 'vertex'}
    <svg viewBox="0 0 24 24" width={size} height={size} fill="#4285f4">
      <path d="M12 2 L14 10 L22 12 L14 14 L12 22 L10 14 L2 12 L10 10 Z"/>
    </svg>
  {:else}
    <span class="initial">{initial}</span>
  {/if}
</span>

<style>
  .logo {
    width: var(--size);
    height: var(--size);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--r-sm);
    background: hsl(var(--hue), 35%, 14%);
    color: hsl(var(--hue), 90%, 75%);
    font-size: calc(var(--size) * 0.55);
    font-weight: 600;
    flex-shrink: 0;
  }
  .initial { line-height: 1; }
</style>
