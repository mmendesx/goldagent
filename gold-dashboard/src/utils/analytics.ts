import type { Position } from '../types';

/** Positions closed within the local calendar day of `now`. */
export function dailyPnl(positions: Position[], now: Date = new Date()): number {
  const startOfDay = new Date(now);
  startOfDay.setHours(0, 0, 0, 0);
  return positions
    .filter((p) => p.closedAt && new Date(p.closedAt) >= startOfDay)
    .reduce((sum, p) => sum + parseFloat(p.realizedPnl ?? '0'), 0);
}

/** Sum of realizedPnl across all provided positions. */
export function cumulativePnl(positions: Position[]): number {
  return positions.reduce((sum, p) => sum + parseFloat(p.realizedPnl ?? '0'), 0);
}

/** Wins / (wins + losses), excluding break-even trades. Returns null if no qualifying trades. */
export function winRate(positions: Position[]): { rate: number; wins: number; losses: number } | null {
  const wins = positions.filter((p) => parseFloat(p.realizedPnl ?? '0') > 0).length;
  const losses = positions.filter((p) => parseFloat(p.realizedPnl ?? '0') < 0).length;
  const total = wins + losses;
  if (total === 0) return null;
  return { rate: wins / total, wins, losses };
}

/** Max peak-to-trough drawdown on the cumulative equity curve. Returns { absolute, relative } where relative is null if peak <= 0. */
export function maxDrawdown(positions: Position[]): { absolute: number; relative: number | null } {
  // Sort by closedAt ascending
  const sorted = [...positions]
    .filter((p) => p.closedAt)
    .sort((a, b) => new Date(a.closedAt!).getTime() - new Date(b.closedAt!).getTime());

  let peak = 0;
  let cumulative = 0;
  let maxDd = 0;
  let peakAtMaxDd = 0;

  for (const p of sorted) {
    cumulative += parseFloat(p.realizedPnl ?? '0');
    if (cumulative > peak) {
      peak = cumulative;
    }
    const dd = peak - cumulative;
    if (dd > maxDd) {
      maxDd = dd;
      peakAtMaxDd = peak;
    }
  }

  return {
    absolute: maxDd,
    relative: peakAtMaxDd > 0 ? maxDd / peakAtMaxDd : null,
  };
}
