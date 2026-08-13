import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { usePrefersReducedMotion } from './usePrefersReducedMotion';

describe('usePrefersReducedMotion', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('reads the media query, follows changes, and removes its listener', () => {
    let listener: ((event: MediaQueryListEvent) => void) | undefined;
    const mediaQuery = {
      matches: false,
      addEventListener: vi.fn((_type: string, next: (event: MediaQueryListEvent) => void) => { listener = next; }),
      removeEventListener: vi.fn()
    };
    vi.stubGlobal('matchMedia', vi.fn(() => mediaQuery));

    const { result, unmount } = renderHook(() => usePrefersReducedMotion());
    expect(result.current).toBe(false);
    act(() => listener?.({ matches: true } as MediaQueryListEvent));
    expect(result.current).toBe(true);
    unmount();
    expect(mediaQuery.removeEventListener).toHaveBeenCalledWith('change', listener);
  });

  it('falls back to legacy media query listeners and removes them on cleanup', () => {
    let listener: ((event: MediaQueryListEvent) => void) | undefined;
    const mediaQuery = {
      matches: false,
      addListener: vi.fn((next: (event: MediaQueryListEvent) => void) => { listener = next; }),
      removeListener: vi.fn()
    };
    vi.stubGlobal('matchMedia', vi.fn(() => mediaQuery));

    const { result, unmount } = renderHook(() => usePrefersReducedMotion());
    act(() => listener?.({ matches: true } as MediaQueryListEvent));
    expect(result.current).toBe(true);
    unmount();
    expect(mediaQuery.removeListener).toHaveBeenCalledWith(listener);
  });

  it('returns false safely when matchMedia is unavailable', () => {
    vi.stubGlobal('matchMedia', undefined);
    const { result } = renderHook(() => usePrefersReducedMotion());
    expect(result.current).toBe(false);
  });
});
