import { afterEach, describe, expect, it } from 'vitest';
import {
  fallbackReaderPreference,
  loadLocalReaderPreference,
  toPreferencePayload
} from './readerPreference';
import type { ReaderPreferencePayload } from '../types/reader';

describe('readerPreference', () => {
  afterEach(() => {
    window.localStorage.clear();
  });

  it('defaults new readers to 20px paged reading', () => {
    expect(fallbackReaderPreference).toMatchObject({
      fontSize: 20,
      readingMode: 'page'
    });
    expect(loadLocalReaderPreference()).toMatchObject({
      fontSize: 20,
      readingMode: 'page'
    });
  });

  it('falls back to 20px paged reading when a payload is incomplete or invalid', () => {
    const payload = {
      fontSize: 0,
      lineHeight: '',
      theme: 'cream',
      readingMode: 'invalid'
    } as unknown as ReaderPreferencePayload;

    expect(toPreferencePayload(payload)).toEqual({
      fontSize: 20,
      lineHeight: '1.8',
      theme: 'cream',
      readingMode: 'page'
    });
  });
});
