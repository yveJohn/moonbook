import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReaderInviteDashboard } from '../../types/reader';
import { MeInviteCard, type InviteCopyState } from './MeInviteCard';

const dashboard: ReaderInviteDashboard = {
  readerId: '9223372036854775801',
  inviteCode: 'MBABCDEFGH',
  inviteCodeAvailable: true,
  shareTextTemplate: '邀请阅读：{{link}}',
  registerRewardCoin: 30,
  firstRechargeRewardCoin: 100,
  invitedCount: 2,
  totalRewardCoin: 130,
  rewards: [{
    id: '9223372036854775811',
    rewardStage: 'register',
    rewardCoin: 30,
    grantTime: '2026-07-10 10:20:30'
  }]
};

function renderCard(options: {
  value?: ReaderInviteDashboard | null;
  loading?: boolean;
  error?: string;
  copyState?: InviteCopyState;
  shareCopyState?: InviteCopyState;
  onCopy?: () => void;
  onCopyShare?: () => void;
  onRetry?: () => void;
  onOpenRewards?: () => void;
} = {}) {
  return render(
    <MeInviteCard
      dashboard={options.value === undefined ? dashboard : options.value}
      loading={options.loading ?? false}
      error={options.error ?? ''}
      copyState={options.copyState ?? 'idle'}
      shareCopyState={options.shareCopyState ?? 'idle'}
      onCopy={options.onCopy ?? vi.fn()}
      onCopyShare={options.onCopyShare ?? vi.fn()}
      onRetry={options.onRetry ?? vi.fn()}
      onOpenRewards={options.onOpenRewards ?? vi.fn()}
    />
  );
}

describe('MeInviteCard', () => {
  afterEach(() => {
    cleanup();
  });

  it('shows only reader-facing invite data', () => {
    renderCard();
    expect(screen.getByText('MBABCDEFGH')).toBeInTheDocument();
    expect(screen.getByText('已邀请')).toBeInTheDocument();
    expect(screen.getByText('累计金币')).toBeInTheDocument();
    expect(screen.getByText('注册奖励')).toBeInTheDocument();
    expect(screen.getByText('首充奖励')).toBeInTheDocument();
    expect(screen.getAllByText('金币')).toHaveLength(2);
    expect(document.querySelectorAll('.me-invite-reward__coin')).toHaveLength(2);
    expect(screen.queryByText('9223372036854775801')).not.toBeInTheDocument();
  });

  it('formats reward copy amounts without losing precision', () => {
    renderCard({
      value: {
        ...dashboard,
        registerRewardCoin: '9007199254740992',
        firstRechargeRewardCoin: '9223372036854775807'
      }
    });

    expect(screen.getByText('9,007,199,254,740,992')).toBeInTheDocument();
    expect(screen.getByText('9,223,372,036,854,775,807')).toBeInTheDocument();
  });

  it('hides incomplete reward copy from legacy dashboard responses', () => {
    renderCard({ value: { ...dashboard, firstRechargeRewardCoin: undefined } });
    expect(screen.queryByText('注册奖励')).not.toBeInTheDocument();
    expect(screen.queryByText('首充奖励')).not.toBeInTheDocument();
  });

  it('formats serialized long invite statistics without losing precision', () => {
    renderCard({
      value: {
        ...dashboard,
        invitedCount: '9007199254740992',
        totalRewardCoin: '9223372036854775807'
      }
    });

    expect(screen.getByText('9,007,199,254,740,992')).toBeInTheDocument();
    expect(screen.getByText('9,223,372,036,854,775,807')).toBeInTheDocument();
  });

  it('uses a stable skeleton without fake statistics', () => {
    const { container } = renderCard({ value: null, loading: true });
    expect(container.querySelector('.me-invite-skeleton')).not.toBeNull();
    expect(screen.queryByText('已邀请')).not.toBeInTheDocument();
    expect(screen.queryByText('累计金币')).not.toBeInTheDocument();
  });

  it('renders local retry without hiding other page sections', () => {
    const retry = vi.fn();
    renderCard({ value: null, error: '邀请码生成失败', onRetry: retry });
    fireEvent.click(screen.getByRole('button', { name: '重新获取邀请码' }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it('shows the backend unavailable reason and disables copy', () => {
    const retry = vi.fn();
    renderCard({
      value: {
        ...dashboard,
        inviteCodeAvailable: false,
        inviteCodeUnavailableReason: '邀请码已禁用'
      },
      onRetry: retry
    });
    expect(screen.getByText('邀请码已禁用')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '复制邀请码' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '复制邀请文案' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: '重新检查邀请码' }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['idle', '复制邀请码'] as const,
    ['copying', '复制中...'] as const,
    ['copied', '已复制'] as const,
    ['error', '重新复制'] as const
  ])('renders %s copy feedback on the button', (copyState, buttonText) => {
    renderCard({ copyState });
    expect(screen.getByRole('button', { name: buttonText })).toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('delegates copy and reward opening', () => {
    const onCopy = vi.fn();
    const onCopyShare = vi.fn();
    const onOpenRewards = vi.fn();
    renderCard({ onCopy, onCopyShare, onOpenRewards });
    fireEvent.click(screen.getByRole('button', { name: '复制邀请码' }));
    fireEvent.click(screen.getByRole('button', { name: '复制邀请文案' }));
    fireEvent.click(screen.getByRole('button', { name: '奖励明细' }));
    expect(onCopy).toHaveBeenCalledTimes(1);
    expect(onCopyShare).toHaveBeenCalledTimes(1);
    expect(onOpenRewards).toHaveBeenCalledTimes(1);
  });
});
