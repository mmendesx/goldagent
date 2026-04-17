import { describe, it, expect } from 'vitest';
import { dailyPnl, cumulativePnl, winRate, maxDrawdown } from './analytics';
import type { Position } from '../types';

function makePosition(overrides: Partial<Position> = {}): Position {
  return {
    id: 1,
    symbol: 'BTCUSDT',
    side: 'LONG',
    entryPrice: '100',
    quantity: '1',
    takeProfitPrice: '110',
    stopLossPrice: '90',
    status: 'closed',
    openedAt: new Date().toISOString(),
    ...overrides,
  };
}

function todayAt(hour: number): string {
  const d = new Date();
  d.setHours(hour, 0, 0, 0);
  return d.toISOString();
}

function yesterdayAt(hour: number): string {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  d.setHours(hour, 0, 0, 0);
  return d.toISOString();
}

describe('dailyPnl', () => {
  it('sums closed positions from the current calendar day', () => {
    const positions = [
      makePosition({ id: 1, closedAt: todayAt(9), realizedPnl: '10' }),
      makePosition({ id: 2, closedAt: todayAt(12), realizedPnl: '-4' }),
      makePosition({ id: 3, closedAt: todayAt(15), realizedPnl: '6' }),
    ];
    expect(dailyPnl(positions)).toBe(12);
  });

  it('excludes positions closed yesterday', () => {
    const positions = [
      makePosition({ id: 1, closedAt: yesterdayAt(10), realizedPnl: '50' }),
      makePosition({ id: 2, closedAt: todayAt(10), realizedPnl: '-5' }),
    ];
    expect(dailyPnl(positions)).toBe(-5);
  });

  it('returns 0 when no positions were closed today', () => {
    const positions = [
      makePosition({ id: 1, closedAt: yesterdayAt(10), realizedPnl: '100' }),
      makePosition({ id: 2, status: 'open', closedAt: undefined, realizedPnl: undefined }),
    ];
    expect(dailyPnl(positions)).toBe(0);
  });

  it('returns 0 for an empty array', () => {
    expect(dailyPnl([])).toBe(0);
  });
});

describe('cumulativePnl', () => {
  it('sums realizedPnl across all positions', () => {
    const positions = [
      makePosition({ id: 1, realizedPnl: '10' }),
      makePosition({ id: 2, realizedPnl: '-4' }),
      makePosition({ id: 3, realizedPnl: '6' }),
      makePosition({ id: 4, realizedPnl: '50' }),
    ];
    expect(cumulativePnl(positions)).toBe(62);
  });

  it('returns 0 for an empty array', () => {
    expect(cumulativePnl([])).toBe(0);
  });

  it('treats missing realizedPnl as zero', () => {
    const positions = [
      makePosition({ id: 1, realizedPnl: '10' }),
      makePosition({ id: 2, realizedPnl: undefined }),
    ];
    expect(cumulativePnl(positions)).toBe(10);
  });
});

describe('winRate', () => {
  it('calculates win rate excluding break-even trades', () => {
    const positions = [
      makePosition({ id: 1, realizedPnl: '5' }),
      makePosition({ id: 2, realizedPnl: '-3' }),
      makePosition({ id: 3, realizedPnl: '2' }),
      makePosition({ id: 4, realizedPnl: '0' }),
      makePosition({ id: 5, realizedPnl: '-1' }),
    ];
    const result = winRate(positions);
    expect(result).not.toBeNull();
    expect(result!.wins).toBe(2);
    expect(result!.losses).toBe(2);
    expect(result!.rate).toBe(0.5);
  });

  it('returns null when there are no qualifying trades', () => {
    const positions = [
      makePosition({ id: 1, realizedPnl: '0' }),
      makePosition({ id: 2, realizedPnl: undefined }),
    ];
    expect(winRate(positions)).toBeNull();
  });

  it('returns null for an empty array', () => {
    expect(winRate([])).toBeNull();
  });

  it('returns rate of 1 when all trades are wins', () => {
    const positions = [
      makePosition({ id: 1, realizedPnl: '10' }),
      makePosition({ id: 2, realizedPnl: '5' }),
    ];
    const result = winRate(positions);
    expect(result).not.toBeNull();
    expect(result!.rate).toBe(1);
    expect(result!.wins).toBe(2);
    expect(result!.losses).toBe(0);
  });
});

describe('maxDrawdown', () => {
  it('computes peak-to-trough drawdown on a mixed equity curve', () => {
    // Equity curve: 10, 20, 15, 5, 25
    // Peak=20 at step 2, trough=5 at step 4 → dd=15, relative=15/20=0.75
    const base = new Date('2024-01-01T00:00:00Z');
    const positions = [
      makePosition({ id: 1, closedAt: new Date(base.getTime() + 1000).toISOString(), realizedPnl: '10' }),
      makePosition({ id: 2, closedAt: new Date(base.getTime() + 2000).toISOString(), realizedPnl: '10' }),
      makePosition({ id: 3, closedAt: new Date(base.getTime() + 3000).toISOString(), realizedPnl: '-5' }),
      makePosition({ id: 4, closedAt: new Date(base.getTime() + 4000).toISOString(), realizedPnl: '-10' }),
      makePosition({ id: 5, closedAt: new Date(base.getTime() + 5000).toISOString(), realizedPnl: '20' }),
    ];
    const result = maxDrawdown(positions);
    expect(result.absolute).toBe(15);
    expect(result.relative).toBeCloseTo(0.75);
  });

  it('returns relative=null when peak never exceeds 0 (all-negative curve)', () => {
    // pnls: -5, -5, +2 → cumulative: -5, -10, -8
    // peak stays 0, maxDd=10, peakAtMaxDd=0 → relative=null
    const base = new Date('2024-01-01T00:00:00Z');
    const positions = [
      makePosition({ id: 1, closedAt: new Date(base.getTime() + 1000).toISOString(), realizedPnl: '-5' }),
      makePosition({ id: 2, closedAt: new Date(base.getTime() + 2000).toISOString(), realizedPnl: '-5' }),
      makePosition({ id: 3, closedAt: new Date(base.getTime() + 3000).toISOString(), realizedPnl: '2' }),
    ];
    const result = maxDrawdown(positions);
    expect(result.absolute).toBe(10);
    expect(result.relative).toBeNull();
  });

  it('returns zero drawdown for a monotonically increasing equity curve', () => {
    const base = new Date('2024-01-01T00:00:00Z');
    const positions = [
      makePosition({ id: 1, closedAt: new Date(base.getTime() + 1000).toISOString(), realizedPnl: '5' }),
      makePosition({ id: 2, closedAt: new Date(base.getTime() + 2000).toISOString(), realizedPnl: '10' }),
      makePosition({ id: 3, closedAt: new Date(base.getTime() + 3000).toISOString(), realizedPnl: '3' }),
    ];
    const result = maxDrawdown(positions);
    expect(result.absolute).toBe(0);
    expect(result.relative).toBeNull();
  });

  it('returns zero drawdown for an empty array', () => {
    const result = maxDrawdown([]);
    expect(result.absolute).toBe(0);
    expect(result.relative).toBeNull();
  });

  it('excludes positions without a closedAt date', () => {
    const base = new Date('2024-01-01T00:00:00Z');
    const positions = [
      makePosition({ id: 1, closedAt: new Date(base.getTime() + 1000).toISOString(), realizedPnl: '10' }),
      makePosition({ id: 2, status: 'open', closedAt: undefined, realizedPnl: '-100' }),
    ];
    const result = maxDrawdown(positions);
    expect(result.absolute).toBe(0);
  });
});
