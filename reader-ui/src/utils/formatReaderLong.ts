import type { ReaderLongValue } from '../types/reader';

export function formatReaderLong(value: ReaderLongValue) {
  const normalized = typeof value === 'number' ? `${value}` : value.trim();
  const match = /^(-?)(\d+)$/.exec(normalized);
  if (!match) return normalized;
  const [, sign, digits] = match;
  return `${sign}${digits.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}`;
}
