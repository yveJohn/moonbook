import { describe, expect, it } from 'vitest';
import { formatUsdtAmount } from './formatUsdtAmount';

describe('formatUsdtAmount', () => {
  it.each([
    ['2', '2.0000'],
    ['2.58', '2.5800'],
    ['2.5800', '2.5800'],
    ['2.00000000', '2.0000']
  ])('formats %s with four decimal places', (value, expected) => {
    expect(formatUsdtAmount(value)).toBe(expected);
  });

  it('does not silently change an unexpected higher-precision amount', () => {
    expect(formatUsdtAmount('1.00234567')).toBe('1.00234567');
  });

  it('preserves an invalid value for diagnostics', () => {
    expect(formatUsdtAmount('invalid')).toBe('invalid');
  });
});
