import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useAnimatedLong } from './useAnimatedLong';

type FrameCallback = (time: number) => void;

let frameCallbacks: Map<number, FrameCallback>;
let nextFrameId: number;

function runNextFrame() {
  const next = frameCallbacks.entries().next().value as [number, FrameCallback] | undefined;
  if (!next) throw new Error('No pending animation frame');
  const [id, callback] = next;
  frameCallbacks.delete(id);
  act(() => callback(id * 16));
}

function runAllFrames() {
  while (frameCallbacks.size > 0) runNextFrame();
}

describe('useAnimatedLong', () => {
  beforeEach(() => {
    frameCallbacks = new Map();
    nextFrameId = 0;
    vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameCallback) => {
      const id = ++nextFrameId;
      frameCallbacks.set(id, callback);
      return id;
    }));
    vi.stubGlobal('cancelAnimationFrame', vi.fn((id: number) => {
      frameCallbacks.delete(id);
    }));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('animates a Long string beyond MAX_SAFE_INTEGER without losing precision', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useAnimatedLong(value, { durationMs: 64 }),
      { initialProps: { value: '9223372036854775800' } }
    );

    rerender({ value: '9223372036854775807' });
    runNextFrame();
    expect(BigInt(result.current)).toBeGreaterThan(9223372036854775800n);
    expect(BigInt(result.current)).toBeLessThan(9223372036854775807n);

    runAllFrames();
    expect(result.current).toBe('9223372036854775807');
  });

  it('animates an increasing value from the currently displayed value', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useAnimatedLong(value, { durationMs: 64 }),
      { initialProps: { value: 0 } }
    );

    rerender({ value: 60 });
    runNextFrame();
    expect(BigInt(result.current)).toBeGreaterThan(0n);
    expect(BigInt(result.current)).toBeLessThan(60n);
    runAllFrames();
    expect(result.current).toBe('60');
  });

  it('animates a decreasing value', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useAnimatedLong(value, { durationMs: 64 }),
      { initialProps: { value: '100' } }
    );

    rerender({ value: '35' });
    runNextFrame();
    expect(BigInt(result.current)).toBeLessThan(100n);
    expect(BigInt(result.current)).toBeGreaterThan(35n);
    runAllFrames();
    expect(result.current).toBe('35');
  });

  it('returns the final value immediately when reduced motion is enabled', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useAnimatedLong(value, { durationMs: 64, reducedMotion: true }),
      { initialProps: { value: '10' } }
    );

    rerender({ value: '80' });
    expect(result.current).toBe('80');
    expect(requestAnimationFrame).not.toHaveBeenCalled();
  });

  it('returns the final value immediately when duration is not positive', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useAnimatedLong(value, { durationMs: 0 }),
      { initialProps: { value: '10' } }
    );

    rerender({ value: '80' });
    expect(result.current).toBe('80');
    expect(requestAnimationFrame).not.toHaveBeenCalled();
  });

  it('continues from the displayed value and prevents a cancelled callback from overwriting it', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useAnimatedLong(value, { durationMs: 64 }),
      { initialProps: { value: '0' } }
    );

    rerender({ value: '80' });
    runNextFrame();
    const intermediateValue = BigInt(result.current);
    const cancelledFrameId = nextFrameId;
    const cancelledCallback = frameCallbacks.get(cancelledFrameId)!;

    rerender({ value: '100' });
    expect(cancelAnimationFrame).toHaveBeenCalledWith(cancelledFrameId);

    act(() => cancelledCallback(cancelledFrameId * 16));
    expect(BigInt(result.current)).toBe(intermediateValue);

    runNextFrame();
    expect(BigInt(result.current)).toBeGreaterThan(intermediateValue);
    expect(BigInt(result.current)).toBeLessThan(100n);
    runAllFrames();
    expect(result.current).toBe('100');
  });

  it('cancels a pending frame on unmount', () => {
    const { rerender, unmount } = renderHook(
      ({ value }) => useAnimatedLong(value, { durationMs: 64 }),
      { initialProps: { value: '0' } }
    );

    rerender({ value: '60' });
    const currentFrameId = nextFrameId;
    unmount();
    expect(cancelAnimationFrame).toHaveBeenCalledWith(currentFrameId);
    expect(frameCallbacks).toHaveLength(0);
  });

  it.each([
    ['not-a-number', 'not-a-number'],
    ['-12', '-12'],
    ['  invalid  ', 'invalid'],
    [Number.MAX_SAFE_INTEGER + 1, `${Number.MAX_SAFE_INTEGER + 1}`],
    [Number.NaN, `${Number.NaN}`],
    [Number.POSITIVE_INFINITY, `${Number.POSITIVE_INFINITY}`],
    [1.5, `${1.5}`]
  ])('returns invalid or negative input %s without throwing or animating', (value, expected) => {
    const { result, rerender } = renderHook(
      ({ currentValue }) => useAnimatedLong(currentValue, { durationMs: 64 }),
      { initialProps: { currentValue: '20' as string | number } }
    );

    expect(() => rerender({ currentValue: value })).not.toThrow();
    expect(result.current).toBe(expected);
    expect(requestAnimationFrame).not.toHaveBeenCalled();
    expect(frameCallbacks).toHaveLength(0);
  });
});
