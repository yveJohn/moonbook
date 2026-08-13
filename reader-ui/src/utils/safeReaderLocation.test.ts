import { describe, expect, it } from 'vitest';
import { safeReaderLocation } from './safeReaderLocation';

describe('safeReaderLocation wallet routes', () => {
  it.each(['/me/diamonds', '/me/coins'])('accepts the exact wallet route %s', (pathname) => {
    expect(safeReaderLocation(pathname)).toEqual({ pathname, search: '', hash: '' });
  });

  it.each(['/me/private', '/me/diamonds/history', '/me/coins/transfer'])('keeps other me sub-paths outside the allowlist: %s', (pathname) => {
    expect(safeReaderLocation(pathname)).toBeNull();
  });
});
