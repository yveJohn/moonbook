import { describe, expect, it } from 'vitest';
import { keepId } from './id';

describe('keepId', () => {
  it('keeps large ids as strings without numeric conversion', () => {
    const id = '9223372036854775807';
    expect(keepId(id)).toBe('9223372036854775807');
  });

  it('keeps empty values safe for optional ids', () => {
    expect(keepId(null)).toBe('');
    expect(keepId(undefined)).toBe('');
  });
});
