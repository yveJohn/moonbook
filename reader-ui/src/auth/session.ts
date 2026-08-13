import type { ReaderLoginResult, ReaderProfile } from '../types/reader';

export const READER_TOKEN_KEY = 'readerToken';
export const READER_PROFILE_KEY = 'readerProfile';
export const READER_SESSION_CHANGED_EVENT = 'moonbook-reader-session-changed';

export function getStoredReaderToken(): string {
  if (typeof window === 'undefined') return '';
  return window.localStorage.getItem(READER_TOKEN_KEY) || '';
}

export function getStoredReaderProfile(): ReaderProfile | null {
  if (typeof window === 'undefined') return null;
  const raw = window.localStorage.getItem(READER_PROFILE_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as ReaderProfile;
  } catch {
    window.localStorage.removeItem(READER_PROFILE_KEY);
    return null;
  }
}

export function storeReaderSession(result: ReaderLoginResult) {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(READER_TOKEN_KEY, result.accessToken);
  window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify(result.reader));
}

export function clearReaderSession() {
  if (typeof window === 'undefined') return;
  window.localStorage.removeItem(READER_TOKEN_KEY);
  window.localStorage.removeItem(READER_PROFILE_KEY);
  window.dispatchEvent(new Event(READER_SESSION_CHANGED_EVENT));
}
