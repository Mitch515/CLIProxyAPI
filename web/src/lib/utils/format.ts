const compactFmt = new Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits: 1 });

export function compactNumber(n: number): string {
  if (!Number.isFinite(n)) return '—';
  return compactFmt.format(n);
}

export function pct(p: number): string {
  if (!Number.isFinite(p)) return '0%';
  const v = Math.max(0, Math.min(1, p));
  return `${(v * 100).toFixed(v >= 0.995 ? 0 : v >= 0.1 ? 1 : 2)}%`;
}

export function durationParts(ms: number): { h: number; m: number; s: number; total: number } {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  return { h, m, s, total };
}

export function formatRemaining(ms: number): string {
  if (ms <= 0) return 'now';
  const { h, m, s } = durationParts(ms);
  if (h >= 24) {
    const d = Math.floor(h / 24);
    const rh = h % 24;
    return `${d}d ${String(rh).padStart(2, '0')}h`;
  }
  if (h > 0) return `${h}h ${String(m).padStart(2, '0')}m ${String(s).padStart(2, '0')}s`;
  if (m > 0) return `${m}m ${String(s).padStart(2, '0')}s`;
  return `${s}s`;
}

export function colorRamp(p: number): string {
  // 0–60% green, 60–85% amber, 85–100% red. Simple stepwise — feels punchier
  // visually than a smooth gradient and matches typical "burn" intuition.
  const v = Math.max(0, Math.min(1, p));
  if (v >= 1) return 'var(--exhausted)';
  if (v >= 0.85) return 'var(--hot)';
  if (v >= 0.6) return 'var(--warn)';
  return 'var(--ok)';
}
