import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  clearReaderSession,
  getStoredReaderToken,
  READER_PROFILE_KEY,
  READER_SESSION_CHANGED_EVENT,
  READER_TOKEN_KEY,
  storeReaderSession
} from './session';

describe('reader session storage', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('clears token and profile while notifying auth context listeners', () => {
    const listener = vi.fn();
    window.addEventListener(READER_SESSION_CHANGED_EVENT, listener);
    storeReaderSession({
      accessToken: 'reader.jwt.token',
      expireIn: 3600,
      reader: {
        readerId: '9223372036854775807',
        username: 'reader',
        nickname: '读者',
        status: 'enabled'
      }
    });

    clearReaderSession();

    expect(getStoredReaderToken()).toBe('');
    expect(window.localStorage.getItem(READER_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(READER_PROFILE_KEY)).toBeNull();
    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener(READER_SESSION_CHANGED_EVENT, listener);
  });
});
