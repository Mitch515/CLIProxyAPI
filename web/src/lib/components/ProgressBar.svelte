<script lang="ts">
  import { colorRamp } from '$lib/utils/format';

  interface Props {
    value: number;     // 0..1
    max?: number;
    height?: number;
    pulse?: boolean;
  }

  let { value, max = 1, height = 6, pulse = true }: Props = $props();

  let v = $derived(Math.max(0, Math.min(1, (value || 0) / (max || 1))));
  let color = $derived(colorRamp(v));
  let armed = $derived(pulse && v >= 0.85);
</script>

<div class="bar" class:armed style="--h: {height}px; --color: {color}; --pct: {v * 100}%;">
  <div class="track"></div>
  <div class="fill"></div>
</div>

<style>
  .bar {
    position: relative;
    width: 100%;
    height: var(--h);
    border-radius: var(--r-pill);
    overflow: hidden;
  }
  .track {
    position: absolute; inset: 0;
    background: var(--bg-elev-3);
    border-radius: inherit;
  }
  .fill {
    position: absolute; left: 0; top: 0; bottom: 0;
    width: var(--pct);
    background: var(--color);
    border-radius: inherit;
    transition: width 420ms cubic-bezier(.16,1,.3,1), background 220ms ease;
  }
  .bar.armed .fill {
    box-shadow: 0 0 8px var(--color);
    animation: pulse 1.6s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50%      { opacity: 0.7; }
  }
</style>
