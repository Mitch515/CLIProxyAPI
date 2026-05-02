<script lang="ts">
  import { colorRamp } from '$lib/utils/format';

  interface Props {
    value: number;       // 0..1
    label?: string;      // shown center (e.g. "5h")
    sublabel?: string;   // shown below center (e.g. "42%")
    size?: number;
    stroke?: number;
    pulse?: boolean;     // glow pulse near limit
  }

  let { value, label, sublabel, size = 96, stroke = 7, pulse = true }: Props = $props();

  let v = $derived(Math.max(0, Math.min(1, value || 0)));
  let r = $derived((size - stroke) / 2);
  let circ = $derived(2 * Math.PI * r);
  let dash = $derived(v * circ);
  let color = $derived(colorRamp(v));
  let armed = $derived(pulse && v >= 0.85);
</script>

<div class="ring" class:armed style="--ring-size: {size}px; --ring-color: {color};">
  <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden="true">
    <circle cx={size / 2} cy={size / 2} r={r} fill="none"
            stroke="var(--bg-elev-3)" stroke-width={stroke} />
    <circle cx={size / 2} cy={size / 2} r={r} fill="none"
            stroke={color}
            stroke-width={stroke}
            stroke-linecap="round"
            stroke-dasharray={`${dash} ${circ - dash}`}
            transform={`rotate(-90 ${size / 2} ${size / 2})`}
            style="transition: stroke-dasharray 420ms cubic-bezier(.16,1,.3,1), stroke 220ms ease;" />
  </svg>
  {#if label || sublabel}
    <div class="center">
      {#if label}<div class="label">{label}</div>{/if}
      {#if sublabel}<div class="sub tabular">{sublabel}</div>{/if}
    </div>
  {/if}
</div>

<style>
  .ring {
    width: var(--ring-size);
    height: var(--ring-size);
    position: relative;
    display: inline-block;
  }
  .center {
    position: absolute; inset: 0;
    display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    pointer-events: none;
  }
  .label   { font-size: var(--fs-12); color: var(--text-muted); letter-spacing: 0.06em; text-transform: uppercase; }
  .sub     { font-size: var(--fs-20); color: var(--text); font-weight: 600; }

  .ring.armed::after {
    content: '';
    position: absolute; inset: -4px;
    border-radius: 50%;
    box-shadow: 0 0 12px var(--ring-color);
    opacity: 0.5;
    animation: pulse 1.6s ease-in-out infinite;
    pointer-events: none;
  }
  @keyframes pulse {
    0%, 100% { opacity: 0.25; transform: scale(1); }
    50%      { opacity: 0.6; transform: scale(1.04); }
  }
</style>
