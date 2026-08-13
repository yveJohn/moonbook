import { act, cleanup, fireEvent, render as renderUi, screen } from '@testing-library/react';
import type React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReaderCheckinStatus, ReaderWallet } from '../../types/reader';
import { MeCheckinCard } from './MeCheckinCard';
import { MeProfileHeader } from './MeProfileHeader';

const wallet: ReaderWallet = {
  readerId: '9223372036854775801',
  rechargeCoinBalance: '12345',
  bonusCoinBalance: '67',
  expiringBonusCoin: 8,
  totalRechargeCoinIncome: 0,
  totalBonusCoinIncome: 0,
  totalRechargeCoinExpense: 0,
  totalBonusCoinExpense: 0
};

function render(ui: React.ReactNode) {
  return renderUi(ui, { wrapper: MemoryRouter });
}

function balance(coin: '钻石' | '金币') {
  return screen.getByLabelText(new RegExp(`^查看${coin}明细，当前余额(?: |$)`));
}

describe('Me account sections', () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('renders profile identity, membership, and avatar without a logout action', () => {
    render(
      <MeProfileHeader
        profile={{ readerId: '9223372036854775801', username: 'reader', nickname: '月读者', status: 'enabled' }}
        membershipText="会员有效至 2026/7/10 10:20:30"
        membershipActive
        membershipLoading={false}
        membershipError=""
        onRetryMembership={vi.fn()}
        wallet={wallet}
        walletLoading={false}
        walletError=""
        onRetryWallet={vi.fn()}
      />
    );
    const heading = screen.getByRole('heading', { name: '月读者' });
    const identity = heading.closest('.me-profile__identity');
    expect(identity?.children).toHaveLength(1);
    expect(identity?.firstElementChild).toBe(heading);
    expect(screen.getByText('月', { selector: '.me-profile__avatar' })).toBeInTheDocument();
    const membership = screen.getByText(/会员有效至/).closest('.me-profile__status');
    expect(membership).toBeInTheDocument();
    expect(membership?.parentElement).toBe(identity?.parentElement);
    expect(membership?.querySelector('.me-profile__vip-icon')).not.toHaveClass('me-profile__vip-icon--inactive');
    const walletLinks = screen.getAllByRole('link', { name: /^查看(?:钻石|金币)明细，当前余额/ });
    expect(walletLinks).toHaveLength(2);
    expect(walletLinks.map((link) => link.getAttribute('href'))).toEqual(['/me/diamonds', '/me/coins']);
    expect(balance('钻石')).toHaveAccessibleName('查看钻石明细，当前余额 12,345');
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 67');
    walletLinks.forEach((link) => {
      const icon = link.querySelector('img');
      expect(icon).not.toBeNull();
      expect(icon).toHaveAttribute('alt', '');
      expect(icon).toHaveAttribute('aria-hidden', 'true');
    });
    expect(screen.queryByRole('button', { name: '退出登录' })).not.toBeInTheDocument();
  });

  it('does not claim no membership while entitlements are loading and retries local failure', () => {
    const retry = vi.fn();
    const { container, rerender } = render(
      <MeProfileHeader profile={null} membershipText={null} membershipLoading membershipError="" onRetryMembership={retry}
        wallet={wallet} walletLoading={false} walletError="" onRetryWallet={vi.fn()} />
    );
    expect(screen.getByLabelText('会员状态加载中')).toBeInTheDocument();
    expect(container.querySelector('.me-profile__vip-icon')).toBeNull();
    expect(screen.queryByText('未开通会员')).not.toBeInTheDocument();
    rerender(
      <MeProfileHeader profile={null} membershipText={null} membershipLoading={false} membershipError="会员状态加载失败" onRetryMembership={retry}
        wallet={wallet} walletLoading={false} walletError="" onRetryWallet={vi.fn()} />
    );
    fireEvent.click(screen.getByRole('button', { name: '重试会员状态' }));
    expect(retry).toHaveBeenCalledTimes(1);
    expect(container.querySelector('.me-profile__vip-icon')).toBeNull();
  });

  it('shows a muted VIP icon for a non-member', () => {
    const { container } = render(
      <MeProfileHeader profile={null} membershipText="未开通会员" membershipLoading={false} membershipError=""
        onRetryMembership={vi.fn()} wallet={wallet} walletLoading={false} walletError="" onRetryWallet={vi.fn()} />
    );
    expect(container.querySelector('.me-profile__vip-icon')).toHaveClass('me-profile__vip-icon--inactive');
  });

  it('uses a checkin skeleton without fake zero and renders unavailable reason', () => {
    const { container, rerender } = render(
      <MeCheckinCard status={null} loading error="" saving={false} onCheckin={vi.fn()} onRetry={vi.fn()} />
    );
    expect(container.querySelector('.me-checkin-heading__icon')).toBeInTheDocument();
    expect(container.querySelector('.me-checkin-skeleton')).not.toBeNull();
    expect(screen.queryByText('0天')).not.toBeInTheDocument();
    rerender(
      <MeCheckinCard
        status={{ todayChecked: false, continuousDays: 1, todayRewardCoin: 0, rewardRandom: false, checkinAvailable: false, unavailableReason: '签到奖励暂未配置' }}
        loading={false}
        error=""
        saving={false}
        onCheckin={vi.fn()}
        onRetry={vi.fn()}
      />
    );
    expect(screen.getByText('签到奖励暂未配置')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '签到' })).toBeDisabled();
  });

  it('disables checkin while stale status is reloading', () => {
    render(
      <MeCheckinCard
        status={{ todayChecked: false, continuousDays: 1, todayRewardCoin: 10, rewardRandom: false, checkinAvailable: true }}
        loading
        error=""
        saving={false}
        onCheckin={vi.fn()}
        onRetry={vi.fn()}
      />
    );
    expect(screen.getByRole('button', { name: '签到' })).toBeDisabled();
  });

  it('retries a local checkin error', () => {
    const retry = vi.fn();
    render(<MeCheckinCard status={null} loading={false} error="签到状态加载失败" saving={false} onCheckin={vi.fn()} onRetry={retry} />);
    fireEvent.click(screen.getByRole('button', { name: '重新加载签到状态' }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it('uses a stable header wallet skeleton and retries a wallet failure without data', () => {
    const retry = vi.fn();
    const props = {
      profile: null, membershipText: '未开通会员', membershipLoading: false,
      membershipError: '', onRetryMembership: vi.fn(), onRetryWallet: retry
    };
    const { container, rerender } = render(
      <MeProfileHeader {...props} wallet={null} walletLoading walletError="" />
    );
    expect(container.querySelector('.me-profile__wallet-skeleton')).not.toBeNull();
    expect(screen.queryByLabelText(/^查看钻石明细，当前余额(?: |$)/)).not.toBeInTheDocument();
    rerender(<MeProfileHeader {...props} wallet={null} walletLoading={false} walletError="钱包加载失败" />);
    fireEvent.click(screen.getByRole('button', { name: '重新加载钱包' }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it('keeps stale wallet balances visible with a local retry action', () => {
    const retry = vi.fn();
    render(
      <MeProfileHeader
        profile={null} membershipText="未开通会员" membershipLoading={false} membershipError="" onRetryMembership={vi.fn()}
        wallet={wallet} walletLoading={false} walletError="钱包刷新失败" onRetryWallet={retry}
      />
    );
    expect(balance('钻石')).toHaveAccessibleName('查看钻石明细，当前余额 12,345');
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 67');
    fireEvent.click(screen.getByRole('button', { name: '重新加载钱包' }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it('formats serialized long wallet and checkin amounts without numeric conversion', () => {
    const serializedAmount = '9007199254740992';
    const serializedWallet: ReaderWallet = {
      readerId: '9223372036854775801',
      rechargeCoinBalance: serializedAmount,
      bonusCoinBalance: 0,
      expiringBonusCoin: 0,
      totalRechargeCoinIncome: 0,
      totalBonusCoinIncome: 0,
      totalRechargeCoinExpense: 0,
      totalBonusCoinExpense: 0
    };
    const serializedCheckin: ReaderCheckinStatus = {
      todayChecked: false,
      continuousDays: 1,
      todayRewardCoin: serializedAmount,
      rewardRandom: false,
      checkinAvailable: true
    };

    render(
      <>
        <MeProfileHeader profile={null} membershipText="未开通会员" membershipLoading={false} membershipError=""
          onRetryMembership={vi.fn()} wallet={serializedWallet} walletLoading={false} walletError="" onRetryWallet={vi.fn()} />
        <MeCheckinCard status={serializedCheckin} loading={false} error="" saving={false} onCheckin={vi.fn()} onRetry={vi.fn()} />
      </>
    );
    const fullValue = '9,007,199,254,740,992';
    expect(screen.getByText(fullValue)).toHaveClass('me-profile__balance-value');
    expect(balance('钻石')).toHaveAccessibleName(`查看钻石明细，当前余额 ${fullValue}`);
    expect(screen.getByText('9,007,199,254,740,992金币')).toBeInTheDocument();
  });

  it('animates bonus balance only for an explicit one-time command', () => {
    const callbacks = new Map<number, FrameRequestCallback>();
    let frameId = 0;
    vi.useFakeTimers();
    vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameRequestCallback) => {
      callbacks.set(++frameId, callback);
      return frameId;
    }));
    vi.stubGlobal('cancelAnimationFrame', vi.fn((id: number) => callbacks.delete(id)));
    const complete = vi.fn();
    const baseProps = {
      profile: null,
      membershipText: '未开通会员',
      membershipLoading: false,
      membershipError: '',
      onRetryMembership: vi.fn(),
      walletLoading: false,
      walletError: '',
      onRetryWallet: vi.fn(),
      onBonusAnimationComplete: complete
    };
    const wallet35 = { ...wallet, bonusCoinBalance: '35' };
    const wallet95 = { ...wallet, bonusCoinBalance: '95' };
    const { rerender } = render(<MeProfileHeader {...baseProps} wallet={wallet35} bonusAnimation={null} />);

    rerender(<MeProfileHeader {...baseProps} wallet={wallet95} bonusAnimation={null} />);
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 95');
    expect(requestAnimationFrame).not.toHaveBeenCalled();

    rerender(<MeProfileHeader {...baseProps} wallet={wallet95}
      bonusAnimation={{ id: 'checkin-1', from: '35', to: '35', active: false }} />);
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 35');
    expect(complete).not.toHaveBeenCalled();
    rerender(<MeProfileHeader {...baseProps} wallet={wallet95}
      bonusAnimation={{ id: 'checkin-1', from: '35', to: '95', active: true }} />);
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 35');
    const [firstId, firstFrame] = callbacks.entries().next().value as [number, FrameRequestCallback];
    callbacks.delete(firstId);
    act(() => firstFrame(16));
    expect(balance('金币')).not.toHaveAccessibleName('查看金币明细，当前余额 35');
    expect(balance('金币')).not.toHaveAccessibleName('查看金币明细，当前余额 95');
    expect(complete).not.toHaveBeenCalled();
    act(() => {
      while (callbacks.size) {
        const [id, callback] = callbacks.entries().next().value as [number, FrameRequestCallback];
        callbacks.delete(id);
        callback(id * 16);
      }
    });
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 95');
    expect(balance('金币')).toHaveAttribute('href', '/me/coins');
    expect(complete).toHaveBeenCalledTimes(1);
    expect(complete).toHaveBeenCalledWith('checkin-1');
    rerender(<MeProfileHeader {...baseProps} wallet={wallet95}
      bonusAnimation={{ id: 'checkin-1', from: '35', to: '95', active: true }} />);
    expect(complete).toHaveBeenCalledTimes(1);
  });

  it('completes an active bonus command immediately for reduced motion', () => {
    const complete = vi.fn();
    vi.stubGlobal('matchMedia', vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn()
    })));
    vi.stubGlobal('requestAnimationFrame', vi.fn());
    render(
      <MeProfileHeader profile={null} membershipText="未开通会员" membershipLoading={false} membershipError=""
        onRetryMembership={vi.fn()} wallet={{ ...wallet, bonusCoinBalance: '95' }} walletLoading={false}
        walletError="" onRetryWallet={vi.fn()} onBonusAnimationComplete={complete}
        bonusAnimation={{ id: 'checkin-reduced', from: '35', to: '95', active: true }} />
    );
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 95');
    expect(complete).toHaveBeenCalledOnce();
    expect(complete).toHaveBeenCalledWith('checkin-reduced');
    expect(requestAnimationFrame).not.toHaveBeenCalled();
  });

  it('animates the whole-card checkin reward from zero and announces only the final reward', () => {
    const callbacks = new Map<number, FrameRequestCallback>();
    let frameId = 0;
    vi.useFakeTimers();
    vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameRequestCallback) => {
      callbacks.set(++frameId, callback);
      return frameId;
    }));
    vi.stubGlobal('cancelAnimationFrame', vi.fn((id: number) => callbacks.delete(id)));
    const onCelebrationEnd = vi.fn();
    const checkinStatus: ReaderCheckinStatus = {
      todayChecked: false,
      continuousDays: 1,
      todayRewardCoin: '9007199254740992',
      rewardRandom: false,
      checkinAvailable: true
    };
    const { container, rerender } = render(
      <MeCheckinCard status={checkinStatus} loading={false} error="" saving={false} celebrating={false}
        onCheckin={vi.fn()} onRetry={vi.fn()} onCelebrationEnd={onCelebrationEnd} />
    );
    const announcement = container.querySelector('.me-checkin-announcement');
    expect(announcement).toBeInTheDocument();
    expect(announcement).toBeEmptyDOMElement();
    rerender(
      <MeCheckinCard status={{ ...checkinStatus, todayChecked: true }}
        loading={false} error="" saving={false} celebrating onCheckin={vi.fn()} onRetry={vi.fn()}
        onCelebrationEnd={onCelebrationEnd} />
    );
    const reward = container.querySelector('.me-checkin-celebration');
    expect(reward).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('status', { name: '签到奖励' })).not.toBeInTheDocument();
    expect(container.querySelectorAll('[aria-live="polite"]')).toHaveLength(1);
    expect(reward).toHaveTextContent('+0金币');
    const [firstId, firstFrame] = callbacks.entries().next().value as [number, FrameRequestCallback];
    callbacks.delete(firstId);
    act(() => firstFrame(16));
    expect(reward).not.toHaveTextContent('+0金币');
    expect(reward).not.toHaveTextContent('+9,007,199,254,740,992金币');
    act(() => vi.advanceTimersByTime(720));
    expect(reward).toHaveTextContent('+9,007,199,254,740,992金币');
    expect(screen.getByText('金币已到账')).toBeInTheDocument();
    expect(announcement).toHaveTextContent('签到成功，获得9,007,199,254,740,992金币');
    expect(onCelebrationEnd).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(80));
    expect(onCelebrationEnd).toHaveBeenCalledTimes(1);
  });
});
