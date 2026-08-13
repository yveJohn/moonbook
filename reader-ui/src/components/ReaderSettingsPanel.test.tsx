import { cleanup, render, screen } from '@testing-library/react';
import type React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fallbackReaderPreference } from '../utils/readerPreference';
import { ReaderSettingsPanel } from './ReaderSettingsPanel';

vi.mock('animal-island-ui', () => ({
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => (
    <section className={className}>{children}</section>
  )
}));

describe('ReaderSettingsPanel', () => {
  afterEach(() => {
    cleanup();
  });

  it('centers the default 20px font size on the slider', () => {
    render(<ReaderSettingsPanel preference={fallbackReaderPreference} onChange={vi.fn()} />);

    const fontSizeInput = screen.getByLabelText('字号') as HTMLInputElement;
    const fontSizeControl = fontSizeInput.closest('.reader-setting-range__control');

    expect(fontSizeInput).toHaveAttribute('min', '12');
    expect(fontSizeInput).toHaveAttribute('max', '28');
    expect(fontSizeInput.value).toBe('20');
    expect(fontSizeControl).toHaveAttribute('data-value', '20');
    expect(fontSizeControl?.getAttribute('style')).toContain('--reader-range-percent: 50%');
  });
});
