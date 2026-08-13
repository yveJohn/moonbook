import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReaderInviteRewardRecord, ReaderLongValue } from '../../types/reader';
import { formatInviteTime, InviteRewardDialog } from './InviteRewardDialog';

function reward(
  id: string,
  rewardStage: string,
  rewardCoin: ReaderLongValue,
  remark?: string
): ReaderInviteRewardRecord {
  return {
    id,
    rewardStage,
    rewardCoin,
    grantTime: '2026-07-10 10:20:30',
    remark
  };
}

describe('InviteRewardDialog', () => {
  afterEach(() => {
    cleanup();
    document.body.style.overflow = '';
    document.body.replaceChildren();
  });

  it('maps stages, falls back to remark, and hides record ids', () => {
    const rewards = [
      reward('9223372036854775811', 'register', 30),
      reward('9223372036854775812', 'first_recharge', 100),
      reward('9223372036854775813', 'campaign', 20, '夏日邀请奖励'),
      reward('9223372036854775814', 'other', 10, '   ')
    ];
    render(<InviteRewardDialog open rewards={rewards} onClose={vi.fn()} />);

    expect(screen.getByText('邀请注册奖励')).toBeInTheDocument();
    expect(screen.getByText('邀请首充奖励')).toBeInTheDocument();
    expect(screen.getByText('夏日邀请奖励')).toBeInTheDocument();
    expect(screen.getByText('邀请奖励')).toBeInTheDocument();
    expect(screen.getByText('+30金币')).toBeInTheDocument();
    for (const item of rewards) {
      expect(screen.queryByText(item.id)).not.toBeInTheDocument();
    }
  });

  it('shows an empty reward state', () => {
    render(<InviteRewardDialog open rewards={[]} onClose={vi.fn()} />);
    expect(screen.getByText('还没有邀请奖励')).toBeInTheDocument();
  });

  it('formats a serialized long reward without losing precision', () => {
    render(
      <InviteRewardDialog
        open
        rewards={[reward('9223372036854775811', 'register', '9223372036854775807')]}
        onClose={vi.fn()}
      />
    );

    expect(screen.getByText('+9,223,372,036,854,775,807金币')).toBeInTheDocument();
  });

  it('keeps an invalid backend time readable', () => {
    expect(formatInviteTime('not-a-time')).toBe('not-a-time');
  });

  it.each([null, undefined, '   '])('uses a readable placeholder for an absent time: %s', (value) => {
    expect(formatInviteTime(value)).toBe('时间待确认');
  });

  it('focuses and traps the close button, then handles Escape', async () => {
    const onClose = vi.fn();
    render(<InviteRewardDialog open rewards={[]} onClose={onClose} />);
    const close = screen.getByRole('button', { name: '关闭奖励明细' });
    await waitFor(() => expect(close).toHaveFocus());
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(close).toHaveFocus();
    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true });
    expect(close).toHaveFocus();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes only when the overlay itself is pressed', () => {
    const onClose = vi.fn();
    render(<InviteRewardDialog open rewards={[]} onClose={onClose} />);
    fireEvent.mouseDown(screen.getByRole('dialog', { name: '奖励明细' }));
    expect(onClose).not.toHaveBeenCalled();
    const overlay = document.body.querySelector('.invite-reward-overlay');
    expect(overlay).not.toBeNull();
    fireEvent.mouseDown(overlay as Element);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('restores body overflow and opener focus after closing', async () => {
    const opener = document.createElement('button');
    document.body.appendChild(opener);
    opener.focus();
    document.body.style.overflow = 'clip';
    const { rerender } = render(<InviteRewardDialog open rewards={[]} onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole('button', { name: '关闭奖励明细' })).toHaveFocus());
    expect(document.body.style.overflow).toBe('hidden');

    rerender(<InviteRewardDialog open={false} rewards={[]} onClose={vi.fn()} />);

    expect(document.body.style.overflow).toBe('clip');
    expect(opener).toHaveFocus();
    opener.remove();
    document.body.style.overflow = '';
  });

  it('coordinates nested dialogs without releasing global effects early', async () => {
    const opener = document.createElement('button');
    document.body.appendChild(opener);
    opener.focus();
    document.body.style.overflow = 'clip';
    const closeLower = vi.fn();
    const closeUpper = vi.fn();
    const { rerender } = render(
      <StrictMode>
        <InviteRewardDialog open rewards={[]} onClose={closeLower} />
        <InviteRewardDialog open rewards={[]} onClose={closeUpper} />
      </StrictMode>
    );

    const dialogs = screen.getAllByRole('dialog', { name: '奖励明细' });
    const labelIds = dialogs.map((dialog) => dialog.getAttribute('aria-labelledby'));
    expect(new Set(labelIds).size).toBe(2);
    const closeButtons = screen.getAllByRole('button', { name: '关闭奖励明细' });
    await waitFor(() => expect(closeButtons[1]).toHaveFocus());
    const overlays = document.body.querySelectorAll('.invite-reward-overlay');
    fireEvent.mouseDown(overlays[0]);
    expect(closeLower).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(closeLower).not.toHaveBeenCalled();
    expect(closeUpper).toHaveBeenCalledTimes(1);

    rerender(
      <StrictMode>
        <InviteRewardDialog open rewards={[]} onClose={closeLower} />
        <InviteRewardDialog open={false} rewards={[]} onClose={closeUpper} />
      </StrictMode>
    );
    expect(document.body.style.overflow).toBe('hidden');
    expect(closeButtons[0]).toHaveFocus();

    rerender(
      <StrictMode>
        <InviteRewardDialog open={false} rewards={[]} onClose={closeLower} />
        <InviteRewardDialog open={false} rewards={[]} onClose={closeUpper} />
      </StrictMode>
    );
    expect(document.body.style.overflow).toBe('clip');
    expect(opener).toHaveFocus();
  });

  it('restores the original opener when the lower dialog closes first', async () => {
    const opener = document.createElement('button');
    document.body.appendChild(opener);
    opener.focus();
    document.body.style.overflow = 'clip';
    const { rerender } = render(
      <StrictMode>
        <InviteRewardDialog open rewards={[]} onClose={vi.fn()} />
        <InviteRewardDialog open rewards={[]} onClose={vi.fn()} />
      </StrictMode>
    );
    await waitFor(() => expect(screen.getAllByRole('button', { name: '关闭奖励明细' })[1]).toHaveFocus());

    rerender(
      <StrictMode>
        <InviteRewardDialog open={false} rewards={[]} onClose={vi.fn()} />
        <InviteRewardDialog open rewards={[]} onClose={vi.fn()} />
      </StrictMode>
    );
    expect(document.body.style.overflow).toBe('hidden');

    rerender(
      <StrictMode>
        <InviteRewardDialog open={false} rewards={[]} onClose={vi.fn()} />
        <InviteRewardDialog open={false} rewards={[]} onClose={vi.fn()} />
      </StrictMode>
    );
    expect(document.body.style.overflow).toBe('clip');
    expect(opener).toHaveFocus();
  });

  it('restores the original opener when nested dialogs close together', async () => {
    const opener = document.createElement('button');
    document.body.appendChild(opener);
    opener.focus();
    document.body.style.overflow = 'clip';
    const { rerender } = render(
      <StrictMode>
        <InviteRewardDialog open rewards={[]} onClose={vi.fn()} />
        <InviteRewardDialog open rewards={[]} onClose={vi.fn()} />
      </StrictMode>
    );
    await waitFor(() => expect(screen.getAllByRole('button', { name: '关闭奖励明细' })[1]).toHaveFocus());

    rerender(
      <StrictMode>
        <InviteRewardDialog open={false} rewards={[]} onClose={vi.fn()} />
        <InviteRewardDialog open={false} rewards={[]} onClose={vi.fn()} />
      </StrictMode>
    );
    expect(document.body.style.overflow).toBe('clip');
    expect(opener).toHaveFocus();
  });
});
