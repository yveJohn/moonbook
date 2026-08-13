import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import type React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PasswordInput } from './PasswordInput';

vi.mock('animal-island-ui', () => ({
  Input: ({ suffix, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { suffix?: React.ReactNode }) => (
    <span><input aria-label="密码" {...props} />{suffix}</span>
  )
}));

describe('PasswordInput', () => {
  afterEach(cleanup);

  it('uses a focusable non-submit control to reveal and hide the password', () => {
    render(<PasswordInput name="password" />);

    const input = screen.getByLabelText('密码');
    const toggle = screen.getByRole('button', { name: '显示密码' });
    expect(input).toHaveAttribute('type', 'password');
    expect(toggle).toHaveAttribute('type', 'button');
    expect(toggle).toHaveClass('password-toggle');
    expect(toggle).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(toggle);
    expect(input).toHaveAttribute('type', 'text');
    expect(screen.getByRole('button', { name: '隐藏密码' })).toHaveAttribute('aria-pressed', 'true');
  });
});
