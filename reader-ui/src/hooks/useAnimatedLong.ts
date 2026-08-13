import { useEffect, useMemo, useRef, useState } from 'react';
import type { ReaderLongValue } from '../types/reader';

const FRAME_DURATION_MS = 16;

export interface UseAnimatedLongOptions {
  durationMs?: number;
  reducedMotion?: boolean;
}

interface NormalizedLongValue {
  display: string;
  bigint: bigint | null;
}

function normalizeLongValue(value: ReaderLongValue): NormalizedLongValue {
  if (typeof value === 'number') {
    const display = `${value}`;
    if (!Number.isSafeInteger(value) || value < 0) {
      return { display, bigint: null };
    }
    const bigint = BigInt(value);
    return { display: bigint.toString(), bigint };
  }

  const display = value.trim();
  if (!/^\d+$/.test(display)) return { display, bigint: null };
  const bigint = BigInt(display);
  return { display: bigint.toString(), bigint };
}

export function useAnimatedLong(
  value: ReaderLongValue,
  { durationMs = 300, reducedMotion = false }: UseAnimatedLongOptions = {}
): string {
  const target = useMemo(() => normalizeLongValue(value), [value]);
  const [displayValue, setDisplayValue] = useState(() => target.display);
  const displayedValueRef = useRef(target);

  useEffect(() => {
    const start = displayedValueRef.current;
    let frameId: number | null = null;
    let cancelled = false;

    const updateDisplay = (next: NormalizedLongValue) => {
      displayedValueRef.current = next;
      setDisplayValue(next.display);
    };

    if (
      target.bigint === null
      || start.bigint === null
      || reducedMotion
      || !Number.isFinite(durationMs)
      || durationMs <= 0
      || start.bigint === target.bigint
    ) {
      updateDisplay(target);
      return;
    }

    const startValue = start.bigint;
    const targetValue = target.bigint;
    const frames = Math.max(1, Math.ceil(durationMs / FRAME_DURATION_MS));
    const totalFrames = BigInt(frames);
    let frame = 0;

    const animate = () => {
      if (cancelled) return;
      frame += 1;
      const nextValue = frame >= frames
        ? targetValue
        : startValue + ((targetValue - startValue) * BigInt(frame)) / totalFrames;
      updateDisplay({ display: nextValue.toString(), bigint: nextValue });
      if (frame < frames) frameId = requestAnimationFrame(animate);
    };

    frameId = requestAnimationFrame(animate);
    return () => {
      cancelled = true;
      if (frameId !== null) cancelAnimationFrame(frameId);
    };
  }, [durationMs, reducedMotion, target]);

  return displayValue;
}
