import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { InsufficientDiamondDialog } from './InsufficientDiamondDialog';

afterEach(cleanup);

function Harness({ onRecharge = vi.fn() }: { onRecharge?: () => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>购买会员</button>
      <InsufficientDiamondDialog open={open} onClose={() => setOpen(false)} onRecharge={onRecharge} />
    </>
  );
}

describe('InsufficientDiamondDialog', () => {
  it('focuses the recharge action and restores the opener after cancel', () => {
    render(<Harness />);
    const opener = screen.getByRole('button', { name: '购买会员' });
    opener.focus();
    fireEvent.click(opener);

    expect(screen.getByRole('dialog', { name: '钻石余额不足' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '去充值' })).toHaveFocus();
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it('calls the recharge action', () => {
    const onRecharge = vi.fn();
    render(<Harness onRecharge={onRecharge} />);
    fireEvent.click(screen.getByRole('button', { name: '购买会员' }));
    fireEvent.click(screen.getByRole('button', { name: '去充值' }));
    expect(onRecharge).toHaveBeenCalledTimes(1);
  });

  it('closes with Escape or an overlay click and traps tab focus', () => {
    render(<Harness />);
    const opener = screen.getByRole('button', { name: '购买会员' });
    fireEvent.click(opener);
    const recharge = screen.getByRole('button', { name: '去充值' });
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(screen.getByRole('button', { name: '取消' })).toHaveFocus();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    fireEvent.click(opener);
    const overlay = document.querySelector('.insufficient-diamond-overlay');
    expect(overlay).not.toBeNull();
    fireEvent.mouseDown(overlay as HTMLElement);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(recharge).not.toHaveFocus();
  });
});
