import { describe, expect, it } from 'vitest';
import { formatReaderLong } from './formatReaderLong';

describe('formatReaderLong', () => {
  it.each([
    [12345, '12,345'],
    ['9007199254740991', '9,007,199,254,740,991'],
    ['9007199254740992', '9,007,199,254,740,992'],
    ['9223372036854775807', '9,223,372,036,854,775,807'],
    ['-9223372036854775808', '-9,223,372,036,854,775,808']
  ])('formats %s without losing precision', (value, expected) => {
    expect(formatReaderLong(value)).toBe(expected);
  });
});
