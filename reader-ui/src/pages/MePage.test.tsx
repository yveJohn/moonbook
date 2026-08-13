import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { Profiler } from 'react';
import type React from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider } from '../auth/ReaderAuthContext';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import { MePage } from './MePage';

const readerApi = vi.hoisted(() => ({
  buyMembership: vi.fn(),
  checkin: vi.fn(),
  ensureInviteDashboard: vi.fn(),
  getEntitlements: vi.fn(),
  getCheckinStatus: vi.fn(),
  getPreference: vi.fn(),
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  listMembershipProducts: vi.fn(),
  listBookshelf: vi.fn(),
  listHistory: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  registerReader: vi.fn(),
  savePreference: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

vi.mock('animal-island-ui', () => ({
  Card: ({ children, className }: { children: React.ReactNode; className?: string }) => (
    <section className={className}>{children}</section>
  ),
  Button: ({ children, onClick }: { children: React.ReactNode; onClick?: () => void }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
  Radio: () => <div />
}));

interface MeCommitSnapshot {
  text: string;
  dialogOpen: boolean;
}

function renderMePage(onCommit?: (snapshot: MeCommitSnapshot) => void) {
  return render(
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={['/me']}>
        <Routes>
          <Route path="/me" element={(
            <Profiler id="me-page" onRender={() => onCommit?.({
              text: document.body.textContent || '',
              dialogOpen: Boolean(document.querySelector('[role="dialog"]'))
            })}>
              <MePage />
            </Profiler>
          )} />
          <Route path="/auth/login" element={<div>登录页</div>} />
          <Route path="/me/recharge" element={<div>充值页</div>} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

const inviteDashboard = {
  readerId: '9223372036854775801',
  inviteCode: 'MBABCDEFGH',
  inviteCodeAvailable: true,
  shareTextTemplate: '邀请你来月白书城：{{link}} 再次打开：{{link}}',
  invitedCount: 2,
  totalRewardCoin: 130,
  rewards: [{
    id: '9223372036854775811',
    rewardStage: 'register',
    rewardCoin: 30,
    grantTime: '2026-07-10 10:20:30'
  }]
};

function product(id: string, durationDays: number | null, productName: string, priceCoin: number) {
  return {
    id,
    productType: 'membership',
    productName,
    priceCoin,
    allowBonusCoin: 0,
    durationDays,
    saleStatus: 'on_sale'
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, resolve, reject };
}

function installAnimationFrameController() {
  const callbacks = new Map<number, FrameRequestCallback>();
  let frameId = 0;
  vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameRequestCallback) => {
    callbacks.set(++frameId, callback);
    return frameId;
  }));
  vi.stubGlobal('cancelAnimationFrame', vi.fn((id: number) => callbacks.delete(id)));
  return {
    runOne() {
      const next = callbacks.entries().next().value as [number, FrameRequestCallback] | undefined;
      if (!next) throw new Error('No pending animation frame');
      callbacks.delete(next[0]);
      act(() => next[1](next[0] * 16));
    },
    runBatch() {
      const batch = Array.from(callbacks.entries());
      act(() => {
        batch.forEach(([id, callback]) => {
          callbacks.delete(id);
          callback(id * 16);
        });
      });
    },
    runAll() {
      act(() => {
        while (callbacks.size) {
          const [id, callback] = callbacks.entries().next().value as [number, FrameRequestCallback];
          callbacks.delete(id);
          callback(id * 16);
        }
      });
    }
  };
}

function setReducedMotion(matches: boolean) {
  const mediaQuery = {
    matches,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn()
  };
  vi.stubGlobal('matchMedia', vi.fn(() => mediaQuery));
}

function switchReaderSession(token: string, nickname: string) {
  window.localStorage.setItem(READER_TOKEN_KEY, token);
  window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify({
    readerId: token === 'reader.token.b' ? '9223372036854775802' : '9223372036854775801',
    username: token === 'reader.token.b' ? 'reader-b' : 'reader-a',
    nickname,
    status: 'enabled'
  }));
  window.dispatchEvent(new StorageEvent('storage', { key: READER_TOKEN_KEY, newValue: token }));
}

function balance(coin: '钻石' | '金币') {
  return screen.getByLabelText(new RegExp(`^查看${coin}明细，当前余额(?: |$)`));
}

function celebrationLayer() {
  const layer = document.querySelector<HTMLElement>('.me-checkin-celebration');
  expect(layer).not.toBeNull();
  return layer as HTMLElement;
}

describe('MePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    window.localStorage.setItem(READER_TOKEN_KEY, 'reader.jwt.token');
    window.localStorage.setItem(
      READER_PROFILE_KEY,
      JSON.stringify({
        readerId: '9223372036854775801',
        username: 'reader',
        nickname: '读者',
        status: 'enabled'
      })
    );
    readerApi.listBookshelf.mockResolvedValue([]);
    readerApi.listHistory.mockResolvedValue([]);
    readerApi.getWallet.mockResolvedValue({
      readerId: '9223372036854775801',
      rechargeCoinBalance: 120,
      bonusCoinBalance: 35,
      totalRechargeCoinIncome: 200,
      totalBonusCoinIncome: 45,
      totalRechargeCoinExpense: 80,
      totalBonusCoinExpense: 10,
      expiringBonusCoin: 5
    });
    readerApi.getCheckinStatus.mockResolvedValue({
      todayChecked: false,
      continuousDays: 6,
      todayRewardCoin: 60,
      rewardRandom: false,
      checkinAvailable: true
    });
    readerApi.getEntitlements.mockResolvedValue({
      readerId: '9223372036854775801',
      adFreeActive: false,
      bookIds: [],
      membershipActive: false,
      membershipPermanent: false,
      membershipExpireTime: null
    });
    readerApi.listMembershipProducts.mockResolvedValue([]);
    readerApi.ensureInviteDashboard.mockResolvedValue(inviteDashboard);
    readerApi.checkin.mockResolvedValue({
      todayChecked: true,
      continuousDays: 7,
      todayRewardCoin: 60,
      rewardRandom: false,
      checkinAvailable: true
    });
    readerApi.getPreference.mockResolvedValue({
      preferenceId: '9223372036854775802',
      fontSize: 18,
      lineHeight: '1.8',
      theme: 'cream',
      readingMode: 'scroll'
    });
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn().mockResolvedValue(undefined) }
    });
    setReducedMotion(false);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    Reflect.deleteProperty(navigator, 'clipboard');
  });

  it('renders wallet balance and checkin status without bookshelf or history', async () => {
    renderMePage();

    expect(await screen.findByLabelText(/^查看钻石明细，当前余额(?: |$)/)).toHaveAccessibleName('查看钻石明细，当前余额 120');
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 35');
    const walletLinks = screen.getAllByRole('link', { name: /^查看(?:钻石|金币)明细，当前余额/ });
    expect(walletLinks.map((link) => link.getAttribute('href'))).toEqual(['/me/diamonds', '/me/coins']);
    walletLinks.forEach((link) => {
      const icon = link.querySelector('img');
      expect(icon).not.toBeNull();
      expect(icon).toHaveAttribute('alt', '');
      expect(icon).toHaveAttribute('aria-hidden', 'true');
    });
    expect(screen.getByText('连续签到')).toBeInTheDocument();
    expect(screen.getByText('6天')).toBeInTheDocument();
    expect(screen.getByText('今日奖励')).toBeInTheDocument();
    expect(screen.getByText('60金币')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: '书架' })).not.toBeInTheDocument();
    expect(screen.queryByText('历史')).not.toBeInTheDocument();
    expect(screen.queryByText('阅读设置')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '退出登录' })).not.toBeInTheDocument();
    await waitFor(() => expect(readerApi.getWallet).toHaveBeenCalled());
    expect(readerApi.getCheckinStatus).toHaveBeenCalled();
    expect(readerApi.listBookshelf).not.toHaveBeenCalled();
    expect(readerApi.listHistory).not.toHaveBeenCalled();
    expect(readerApi.getPreference).not.toHaveBeenCalled();
    expect(readerApi.savePreference).not.toHaveBeenCalled();
  });

  it('locks the old bonus balance, then animates to the authoritative wallet target instead of adding the reward', async () => {
    const refreshedWallet = deferred<Awaited<ReturnType<typeof readerApi.getWallet>>>();
    readerApi.getWallet
      .mockResolvedValueOnce({ readerId: '9223372036854775801', rechargeCoinBalance: 120, bonusCoinBalance: '35', expiringBonusCoin: 5, totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0, totalBonusCoinExpense: 0 })
      .mockReturnValueOnce(refreshedWallet.promise);
    renderMePage();
    const button = await screen.findByRole('button', { name: '签到' });
    vi.useFakeTimers();
    const frames = installAnimationFrameController();
    fireEvent.click(button);
    fireEvent.click(button);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    const reward = celebrationLayer();
    expect(reward).toHaveAttribute('aria-hidden', 'true');
    expect(reward).toHaveTextContent('+0金币');
    expect(readerApi.checkin).toHaveBeenCalledTimes(1);
    expect(readerApi.getWallet).toHaveBeenCalledTimes(2);
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 35');
    frames.runBatch();
    expect(reward).not.toHaveTextContent('+0金币');
    expect(reward).not.toHaveTextContent('+60金币');
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 35');
    await act(async () => {
      refreshedWallet.resolve({ readerId: '9223372036854775801', rechargeCoinBalance: 120, bonusCoinBalance: '125', expiringBonusCoin: 5, totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0, totalBonusCoinExpense: 0 });
      await refreshedWallet.promise;
      await Promise.resolve();
    });
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 35');
    frames.runBatch();
    expect(balance('金币')).not.toHaveAccessibleName('查看金币明细，当前余额 35');
    expect(balance('金币')).not.toHaveAccessibleName('查看金币明细，当前余额 125');
    expect(document.querySelector('.me-profile__bonus-animation')).not.toBeNull();
    act(() => vi.advanceTimersByTime(10_000));
    expect(balance('金币')).not.toHaveAccessibleName('查看金币明细，当前余额 125');
    expect(document.querySelector('.me-profile__bonus-animation')).not.toBeNull();
    frames.runAll();
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 125');
    expect(document.querySelector('.me-profile__bonus-animation')).toBeNull();
    expect(document.querySelector('.me-checkin-celebration')).toBeNull();
    expect(screen.getByRole('button', { name: '已签到' })).toBeDisabled();
    expect(readerApi.getWallet).toHaveBeenCalledTimes(2);
  });

  it('renders random checkin reward text before checkin', async () => {
    readerApi.getCheckinStatus.mockResolvedValue({
      todayChecked: false,
      continuousDays: 7,
      todayRewardCoin: 0,
      rewardRandom: true,
      rewardText: '随机金币奖励',
      checkinAvailable: true
    });

    renderMePage();

    expect(await screen.findByText('随机金币奖励')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '签到' })).toBeEnabled();
  });

  it('disables checkin when reward config is unavailable', async () => {
    readerApi.getCheckinStatus.mockResolvedValue({
      todayChecked: false,
      continuousDays: 1,
      todayRewardCoin: 0,
      rewardRandom: false,
      checkinAvailable: false,
      unavailableReason: '签到奖励暂未配置'
    });

    renderMePage();

    expect(await screen.findByText('签到奖励暂未配置')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '签到' })).toBeDisabled();
  });

  it('keeps wallet and checkin visible when membership plans fail to load', async () => {
    readerApi.listMembershipProducts.mockRejectedValue(new Error('会员套餐加载失败'));

    renderMePage();

    expect(await screen.findByLabelText(/^查看钻石明细，当前余额(?: |$)/)).toBeInTheDocument();
    expect(screen.getByText('120')).toBeInTheDocument();
    expect(screen.getByText('连续签到')).toBeInTheDocument();
    expect(screen.getByText('6天')).toBeInTheDocument();
    expect(await screen.findByText('会员套餐加载失败')).toBeInTheDocument();
  });

  it('renders timed membership status from backend timestamp', async () => {
    const expireTime = '2026-07-08 10:20:30';
    readerApi.getEntitlements.mockResolvedValue({
      readerId: '9223372036854775801',
      adFreeActive: false,
      bookIds: [],
      membershipActive: true,
      membershipPermanent: false,
      membershipExpireTime: expireTime
    });

    renderMePage();

    const formattedExpireTime = new Date(expireTime.replace(' ', 'T')).toLocaleString('zh-CN', { hour12: false });
    expect(await screen.findByText(`会员有效至 ${formattedExpireTime}`)).toBeInTheDocument();
  });

  it('starts all five resources before any one settles and keeps mixed outcomes local', async () => {
    const walletRequest = deferred<Awaited<ReturnType<typeof readerApi.getWallet>>>();
    const checkinRequest = deferred<Awaited<ReturnType<typeof readerApi.getCheckinStatus>>>();
    const inviteRequest = deferred<typeof inviteDashboard>();
    const entitlementRequest = deferred<Awaited<ReturnType<typeof readerApi.getEntitlements>>>();
    const productRequest = deferred<ReturnType<typeof product>[]>();
    readerApi.getWallet.mockReturnValueOnce(walletRequest.promise);
    readerApi.getCheckinStatus.mockReturnValueOnce(checkinRequest.promise);
    readerApi.ensureInviteDashboard.mockReturnValueOnce(inviteRequest.promise);
    readerApi.getEntitlements.mockReturnValueOnce(entitlementRequest.promise);
    readerApi.listMembershipProducts.mockReturnValueOnce(productRequest.promise);

    renderMePage();
    await waitFor(() => {
      expect(readerApi.getWallet).toHaveBeenCalledTimes(1);
      expect(readerApi.getCheckinStatus).toHaveBeenCalledTimes(1);
      expect(readerApi.ensureInviteDashboard).toHaveBeenCalledTimes(1);
      expect(readerApi.getEntitlements).toHaveBeenCalledTimes(1);
      expect(readerApi.listMembershipProducts).toHaveBeenCalledTimes(1);
    });

    await act(async () => {
      walletRequest.resolve({
        readerId: '9223372036854775801',
        rechargeCoinBalance: 120,
        bonusCoinBalance: 35,
        expiringBonusCoin: 5,
        totalRechargeCoinIncome: 200,
        totalBonusCoinIncome: 45,
        totalRechargeCoinExpense: 80,
        totalBonusCoinExpense: 10
      });
      checkinRequest.reject(new Error('签到独立失败'));
      inviteRequest.resolve(inviteDashboard);
      entitlementRequest.reject(new Error('会员状态独立失败'));
      productRequest.resolve([product('9223372036854775809', 30, '会员月卡', 99)]);
      await Promise.allSettled([
        walletRequest.promise,
        checkinRequest.promise,
        inviteRequest.promise,
        entitlementRequest.promise,
        productRequest.promise
      ]);
    });

    expect(screen.getByText('120')).toBeInTheDocument();
    expect(screen.getByText('签到独立失败')).toBeInTheDocument();
    expect(screen.getByText('MBABCDEFGH')).toBeInTheDocument();
    expect(screen.getByText('会员状态独立失败')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '购买会员月卡，99钻石' })).toBeDisabled();
  });

  it('loads all four resources for reader B and ignores reader A responses that arrive later', async () => {
    const checkinA = deferred<Awaited<ReturnType<typeof readerApi.getCheckinStatus>>>();
    const inviteA = deferred<typeof inviteDashboard>();
    const entitlementsA = deferred<Awaited<ReturnType<typeof readerApi.getEntitlements>>>();
    const productsA = deferred<ReturnType<typeof product>[]>();
    readerApi.getCheckinStatus
      .mockReturnValueOnce(checkinA.promise)
      .mockResolvedValueOnce({ todayChecked: false, continuousDays: 22, todayRewardCoin: 8, rewardRandom: false, checkinAvailable: true });
    readerApi.ensureInviteDashboard
      .mockReturnValueOnce(inviteA.promise)
      .mockResolvedValueOnce({ ...inviteDashboard, readerId: '9223372036854775802', inviteCode: 'MBBREADER22' });
    readerApi.getEntitlements
      .mockReturnValueOnce(entitlementsA.promise)
      .mockResolvedValueOnce({ readerId: '9223372036854775802', bookIds: [], membershipActive: true, membershipPermanent: false, membershipExpireTime: '2030-01-01 00:00:00' });
    readerApi.listMembershipProducts
      .mockReturnValueOnce(productsA.promise)
      .mockResolvedValueOnce([product('9223372036854775899', 30, 'B会员月卡', 88)]);

    renderMePage();
    await waitFor(() => expect(readerApi.listMembershipProducts).toHaveBeenCalledTimes(1));
    act(() => switchReaderSession('reader.token.b', '读者B'));
    expect(await screen.findByRole('heading', { name: '读者B' })).toBeInTheDocument();
    expect(await screen.findByText('22天')).toBeInTheDocument();
    expect(await screen.findByText('MBBREADER22')).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: '购买B会员月卡，88钻石' })).toBeInTheDocument();
    expect(await screen.findByText(/会员有效至/)).toBeInTheDocument();
    expect(readerApi.getCheckinStatus).toHaveBeenCalledTimes(2);
    expect(readerApi.ensureInviteDashboard).toHaveBeenCalledTimes(2);
    expect(readerApi.getEntitlements).toHaveBeenCalledTimes(2);
    expect(readerApi.listMembershipProducts).toHaveBeenCalledTimes(2);

    await act(async () => {
      checkinA.resolve({ todayChecked: false, continuousDays: 91, todayRewardCoin: 1, rewardRandom: false, checkinAvailable: true });
      inviteA.resolve({ ...inviteDashboard, inviteCode: 'MBAOLDLATE1' });
      entitlementsA.resolve({ readerId: '9223372036854775801', bookIds: [], membershipActive: false, membershipPermanent: false, membershipExpireTime: null });
      productsA.resolve([product('9223372036854775809', 30, 'A迟到套餐', 99)]);
      await Promise.all([checkinA.promise, inviteA.promise, entitlementsA.promise, productsA.promise]);
    });
    expect(screen.getByText('22天')).toBeInTheDocument();
    expect(screen.getByText('MBBREADER22')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '购买B会员月卡，88钻石' })).toBeInTheDocument();
    expect(screen.queryByText('91天')).not.toBeInTheDocument();
    expect(screen.queryByText('MBAOLDLATE1')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '购买A迟到套餐，99钻石' })).not.toBeInTheDocument();
  });

  it('resets local actions on token change and ignores reader A checkin and copy completion', async () => {
    const checkinA = deferred<Awaited<ReturnType<typeof readerApi.checkin>>>();
    const copyA = deferred<void>();
    readerApi.checkin.mockReturnValueOnce(checkinA.promise);
    vi.mocked(navigator.clipboard.writeText).mockReturnValueOnce(copyA.promise);
    const commits: MeCommitSnapshot[] = [];
    renderMePage((snapshot) => commits.push(snapshot));
    const checkinButton = await screen.findByRole('button', { name: '签到' });
    const copyButton = await screen.findByRole('button', { name: '复制邀请码' });
    fireEvent.click(screen.getByRole('button', { name: '奖励明细' }));
    fireEvent.click(checkinButton);
    fireEvent.click(copyButton);
    expect(screen.getByRole('dialog', { name: '奖励明细' })).toBeInTheDocument();
    expect(readerApi.checkin).toHaveBeenCalledTimes(1);

    const commitCountBeforeSwitch = commits.length;
    act(() => switchReaderSession('reader.token.b', '读者B'));
    expect(await screen.findByRole('heading', { name: '读者B' })).toBeInTheDocument();
    const firstBCommit = commits.slice(commitCountBeforeSwitch).find((snapshot) => snapshot.text.includes('读者B'));
    expect(firstBCommit?.text).not.toContain('MBABCDEFGH');
    expect(firstBCommit?.text).not.toContain('6天');
    expect(firstBCommit?.dialogOpen).toBe(false);
    expect(screen.queryByRole('dialog', { name: '奖励明细' })).not.toBeInTheDocument();
    expect(await screen.findByRole('button', { name: '签到' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '复制邀请码' })).toBeEnabled();
    expect(document.querySelector('.me-checkin-celebration')).toBeNull();

    await act(async () => {
      checkinA.resolve({ todayChecked: true, continuousDays: 99, todayRewardCoin: 999, rewardRandom: false, checkinAvailable: true });
      copyA.resolve();
      await Promise.all([checkinA.promise, copyA.promise]);
    });
    expect(screen.queryByText('99天')).not.toBeInTheDocument();
    expect(screen.queryByText('已复制')).not.toBeInTheDocument();
    expect(document.querySelector('.me-checkin-celebration')).toBeNull();
    expect(readerApi.getWallet).toHaveBeenCalledTimes(2);
  });

  it('keeps reader B purchase locked when reader A purchase resolves late and uses a new request id', async () => {
    const purchaseA = deferred<{ status: string }>();
    const purchaseB = deferred<{ status: string }>();
    readerApi.listMembershipProducts.mockResolvedValue([
      product('9223372036854775809', 30, '会员月卡', 99)
    ]);
    readerApi.buyMembership
      .mockReturnValueOnce(purchaseA.promise)
      .mockReturnValueOnce(purchaseB.promise);
    renderMePage();
    let buy = await screen.findByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(buy).toBeEnabled());
    fireEvent.click(buy);
    await waitFor(() => expect(readerApi.buyMembership).toHaveBeenCalledTimes(1));

    act(() => switchReaderSession('reader.token.b', '读者B'));
    buy = await screen.findByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(buy).toBeEnabled());
    fireEvent.click(buy);
    await waitFor(() => expect(readerApi.buyMembership).toHaveBeenCalledTimes(2));
    expect(buy).toBeDisabled();
    expect(readerApi.buyMembership.mock.calls[1][1]).not.toBe(readerApi.buyMembership.mock.calls[0][1]);

    await act(async () => {
      purchaseA.resolve({ status: 'paid' });
      await purchaseA.promise;
    });
    expect(buy).toBeDisabled();
    expect(readerApi.getWallet).toHaveBeenCalledTimes(2);
    expect(readerApi.getEntitlements).toHaveBeenCalledTimes(2);

    await act(async () => {
      purchaseB.resolve({ status: 'paid' });
      await purchaseB.promise;
    });
    await waitFor(() => expect(buy).toBeEnabled());
    expect(readerApi.getWallet).toHaveBeenCalledTimes(3);
    expect(readerApi.getEntitlements).toHaveBeenCalledTimes(3);
  });

  it('keeps checkin and invite visible when wallet fails', async () => {
    readerApi.getWallet.mockRejectedValueOnce(new Error('钱包独立失败'));
    renderMePage();
    expect(await screen.findByText('钱包独立失败')).toBeInTheDocument();
    expect(await screen.findByText('连续签到')).toBeInTheDocument();
    expect(await screen.findByText('MBABCDEFGH')).toBeInTheDocument();
  });

  it('keeps wallet and checkin visible when invite fails', async () => {
    readerApi.ensureInviteDashboard.mockRejectedValueOnce(new Error('邀请码生成失败'));
    renderMePage();
    expect(await screen.findByText('邀请码生成失败')).toBeInTheDocument();
    expect(await screen.findByLabelText(/^查看钻石明细，当前余额(?: |$)/)).toBeInTheDocument();
    expect(await screen.findByText('连续签到')).toBeInTheDocument();
  });

  it('keeps wallet and invite visible when membership products fail', async () => {
    readerApi.listMembershipProducts.mockRejectedValueOnce(new Error('会员套餐加载失败'));
    renderMePage();
    expect(await screen.findByLabelText(/^查看钻石明细，当前余额(?: |$)/)).toBeInTheDocument();
    expect(await screen.findByText('MBABCDEFGH')).toBeInTheDocument();
    expect(await screen.findByText('会员套餐加载失败')).toBeInTheDocument();
  });

  it('does not show zero balances before wallet resolves', () => {
    readerApi.getWallet.mockReturnValue(new Promise(() => undefined));
    const { container } = renderMePage();
    expect(container.querySelector('.me-profile__wallet-skeleton')).not.toBeNull();
    expect(screen.queryByLabelText(/^查看钻石明细，当前余额(?: |$)/)).not.toBeInTheDocument();
  });

  it('keeps the approved mobile section order and does not render the removed wallet section', async () => {
    const { container } = renderMePage();
    await screen.findByText('MBABCDEFGH');
    const main = container.querySelector('main');
    expect(main).not.toBeNull();
    expect(Array.from((main as HTMLElement).querySelectorAll(':scope > [data-section]'))
      .map((node) => node.getAttribute('data-section')))
      .toEqual(['profile', 'checkin', 'membership', 'invite', 'quick-links']);
    expect(container.querySelector('.me-action-grid')).toBeNull();
    expect(container.querySelector('.me-action-primary')).toBeNull();
    expect(container.querySelector('.me-wallet-section')).toBeNull();
    expect(screen.queryByRole('heading', { name: '我的钱包' })).not.toBeInTheDocument();
    expect(readerApi.listBookshelf).not.toHaveBeenCalled();
    expect(readerApi.listHistory).not.toHaveBeenCalled();
    expect(readerApi.getPreference).not.toHaveBeenCalled();
    expect(readerApi.savePreference).not.toHaveBeenCalled();
    expect(screen.queryByText('阅读设置')).not.toBeInTheDocument();
  });

  it('renders navigation links for likes and settings', async () => {
    renderMePage();
    const links = await screen.findByRole('navigation', { name: '我的功能' });
    expect(within(links).getByRole('link', { name: '赞过' })).toHaveAttribute('href', '/me/likes');
    expect(within(links).getByRole('link', { name: '设置' })).toHaveAttribute('href', '/me/settings');
  });

  it('retries only the invitation resource', async () => {
    readerApi.ensureInviteDashboard
      .mockRejectedValueOnce(new Error('邀请码生成失败'))
      .mockResolvedValueOnce(inviteDashboard);
    renderMePage();
    fireEvent.click(await screen.findByRole('button', { name: '重新获取邀请码' }));
    expect(await screen.findByText('MBABCDEFGH')).toBeInTheDocument();
    expect(readerApi.ensureInviteDashboard).toHaveBeenCalledTimes(2);
    expect(readerApi.getWallet).toHaveBeenCalledTimes(1);
    expect(readerApi.getCheckinStatus).toHaveBeenCalledTimes(1);
  });

  it('copies the exact invite code and opens rewards without another request', async () => {
    renderMePage();
    fireEvent.click(await screen.findByRole('button', { name: '复制邀请码' }));
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith('MBABCDEFGH'));
    expect(await screen.findByRole('button', { name: '已复制' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '奖励明细' }));
    expect(screen.getByRole('dialog', { name: '奖励明细' })).toBeInTheDocument();
    expect(readerApi.ensureInviteDashboard).toHaveBeenCalledTimes(1);
  });

  it('copies configured invite text with every link placeholder replaced by the current origin', async () => {
    renderMePage();
    fireEvent.click(await screen.findByRole('button', { name: '复制邀请文案' }));
    const inviteLink = `${window.location.origin}/auth/register?inviteCode=MBABCDEFGH`;

    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      `邀请你来月白书城：${inviteLink} 再次打开：${inviteLink}`
    ));
    expect(screen.getByRole('button', { name: '文案已复制' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '复制邀请码' })).toBeInTheDocument();
  });

  it('encodes the invite code in copied invite links', async () => {
    readerApi.ensureInviteDashboard.mockResolvedValueOnce({
      ...inviteDashboard,
      inviteCode: 'MB CODE/+',
      shareTextTemplate: '{{link}}'
    });
    renderMePage();
    fireEvent.click(await screen.findByRole('button', { name: '复制邀请文案' }));

    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      `${window.location.origin}/auth/register?inviteCode=MB+CODE%2F%2B`
    ));
  });

  it('copies configured text unchanged when it has no link placeholder', async () => {
    readerApi.ensureInviteDashboard.mockResolvedValueOnce({
      ...inviteDashboard,
      shareTextTemplate: '欢迎来月白书城阅读'
    });
    renderMePage();
    fireEvent.click(await screen.findByRole('button', { name: '复制邀请文案' }));

    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith('欢迎来月白书城阅读'));
  });

  it('keeps invite text copy failure separate and allows retry', async () => {
    vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(new Error('denied'));
    renderMePage();
    fireEvent.click(await screen.findByRole('button', { name: '复制邀请文案' }));

    expect(await screen.findByText('邀请文案复制失败，请重试')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重新复制文案' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '复制邀请码' })).toBeEnabled();
  });

  it('shows clipboard failure locally and allows another copy attempt', async () => {
    vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(new Error('denied'));
    renderMePage();
    fireEvent.click(await screen.findByRole('button', { name: '复制邀请码' }));
    expect(await screen.findByText('复制失败，请重试')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重新复制' })).toBeEnabled();
  });

  it('resets copied feedback after two seconds', async () => {
    renderMePage();
    await screen.findByRole('button', { name: '复制邀请码' });
    vi.useFakeTimers();
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '复制邀请码' }));
      await Promise.resolve();
    });
    expect(screen.getByRole('button', { name: '已复制' })).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(2_000); });
    expect(screen.getByRole('button', { name: '复制邀请码' })).toBeInTheDocument();
  });

  it('does not schedule copied feedback after unmount while clipboard is pending', async () => {
    const clipboardRequest = deferred<void>();
    vi.mocked(navigator.clipboard.writeText).mockReturnValueOnce(clipboardRequest.promise);
    const { unmount } = renderMePage();
    await screen.findByRole('button', { name: '复制邀请码' });
    vi.useFakeTimers();

    fireEvent.click(screen.getByRole('button', { name: '复制邀请码' }));
    expect(navigator.clipboard.writeText).toHaveBeenCalledTimes(1);
    unmount();
    await act(async () => {
      clipboardRequest.resolve();
      await clipboardRequest.promise;
    });

    expect(vi.getTimerCount()).toBe(0);
  });

  it('hides every purchase plan for permanent membership', async () => {
    readerApi.getEntitlements.mockResolvedValueOnce({
      readerId: '9223372036854775801',
      adFreeActive: false,
      bookIds: [],
      membershipActive: true,
      membershipPermanent: true,
      membershipExpireTime: null
    });
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', null, '永久会员', 999)
    ]);
    renderMePage();
    await waitFor(() => expect(screen.getAllByText('永久会员').length).toBeGreaterThan(0));
    expect(screen.queryByRole('button', { name: /购买永久会员/ })).not.toBeInTheDocument();
  });

  it('keeps plans purchasable for timed membership', async () => {
    readerApi.getEntitlements.mockResolvedValueOnce({
      readerId: '9223372036854775801',
      adFreeActive: false,
      bookIds: [],
      membershipActive: true,
      membershipPermanent: false,
      membershipExpireTime: '2026-08-10 10:20:30'
    });
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', 365, '会员年卡', 699)
    ]);
    renderMePage();
    expect(await screen.findByText(/会员有效至/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '购买会员年卡，699钻石' })).toBeEnabled();
  });

  it('locks all plans while one purchase is pending', async () => {
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', 7, '会员周卡', 30),
      product('9223372036854775810', 30, '会员月卡', 99)
    ]);
    readerApi.buyMembership.mockReturnValueOnce(new Promise(() => undefined));
    renderMePage();
    const week = await screen.findByRole('button', { name: '购买会员周卡，30钻石' });
    const month = screen.getByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(week).toBeEnabled());
    fireEvent.click(week);
    await waitFor(() => expect(readerApi.buyMembership).toHaveBeenCalledTimes(1));
    expect(week).toBeDisabled();
    expect(month).toBeDisabled();
  });

  it('synchronously locks a membership purchase before React state refreshes', async () => {
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', 7, '会员周卡', 30)
    ]);
    readerApi.buyMembership.mockReturnValueOnce(new Promise(() => undefined));
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员周卡，30钻石' });
    await waitFor(() => expect(buy).toBeEnabled());

    act(() => {
      buy.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      buy.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(readerApi.buyMembership).toHaveBeenCalledTimes(1);
    expect(readerApi.buyMembership.mock.calls[0][0]).toBe('9223372036854775809');
  });

  it('submits a large membership product id without numeric conversion', async () => {
    readerApi.listMembershipProducts.mockResolvedValue([
      product('9223372036854775809', 30, '会员月卡', 99)
    ]);
    readerApi.buyMembership.mockResolvedValue({ status: 'paid' });
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(buy).toBeEnabled());
    fireEvent.click(buy);
    await waitFor(() => expect(readerApi.buyMembership)
      .toHaveBeenCalledWith('9223372036854775809', expect.any(String)));
  });

  it('reuses the membership request id when an ambiguous failure is retried', async () => {
    readerApi.listMembershipProducts.mockResolvedValue([
      product('9223372036854775809', 30, '会员月卡', 99)
    ]);
    readerApi.buyMembership
      .mockRejectedValueOnce(new Error('响应未确认'))
      .mockResolvedValueOnce({ status: 'paid' });
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(buy).toBeEnabled());

    fireEvent.click(buy);
    expect(await screen.findByText('响应未确认')).toBeInTheDocument();
    await waitFor(() => expect(buy).toBeEnabled());
    fireEvent.click(buy);
    await waitFor(() => expect(readerApi.buyMembership).toHaveBeenCalledTimes(2));

    expect(readerApi.buyMembership.mock.calls[0][0]).toBe('9223372036854775809');
    expect(readerApi.buyMembership.mock.calls[1][0]).toBe('9223372036854775809');
    expect(readerApi.buyMembership.mock.calls[1][1]).toBe(readerApi.buyMembership.mock.calls[0][1]);
  });

  it('creates a new membership request id after a confirmed purchase succeeds', async () => {
    readerApi.listMembershipProducts.mockResolvedValue([
      product('9223372036854775809', 30, '会员月卡', 99)
    ]);
    readerApi.buyMembership.mockResolvedValue({ status: 'paid' });
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(buy).toBeEnabled());

    fireEvent.click(buy);
    await waitFor(() => expect(readerApi.buyMembership).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(buy).toBeEnabled());
    fireEvent.click(buy);
    await waitFor(() => expect(readerApi.buyMembership).toHaveBeenCalledTimes(2));

    expect(readerApi.buyMembership.mock.calls[1][1]).not.toBe(readerApi.buyMembership.mock.calls[0][1]);
  });

  it('does not refresh wallet or entitlements after a pending purchase resolves after unmount', async () => {
    const purchaseRequest = deferred<{ status: string }>();
    readerApi.listMembershipProducts.mockResolvedValue([
      product('9223372036854775809', 30, '会员月卡', 99)
    ]);
    readerApi.buyMembership.mockReturnValueOnce(purchaseRequest.promise);
    const { unmount } = renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(buy).toBeEnabled());

    fireEvent.click(buy);
    await waitFor(() => expect(readerApi.buyMembership).toHaveBeenCalledTimes(1));
    unmount();
    await act(async () => {
      purchaseRequest.resolve({ status: 'paid' });
      await purchaseRequest.promise;
    });

    expect(readerApi.getWallet).toHaveBeenCalledTimes(1);
    expect(readerApi.getEntitlements).toHaveBeenCalledTimes(1);
  });

  it('keeps a successful entitlement refresh when wallet refresh fails after purchase', async () => {
    readerApi.getWallet
      .mockResolvedValueOnce({ readerId: '9223372036854775801', rechargeCoinBalance: 1200, bonusCoinBalance: 35, expiringBonusCoin: 5, totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0, totalBonusCoinExpense: 0 })
      .mockRejectedValueOnce(new Error('钱包刷新失败'));
    readerApi.getEntitlements
      .mockResolvedValueOnce({ readerId: '9223372036854775801', bookIds: [], membershipActive: false, membershipPermanent: false, membershipExpireTime: null })
      .mockResolvedValueOnce({ readerId: '9223372036854775801', bookIds: [], membershipActive: true, membershipPermanent: true, membershipExpireTime: null });
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', null, '永久会员', 999)
    ]);
    readerApi.buyMembership.mockResolvedValueOnce({ status: 'paid' });
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买永久会员，999钻石' });
    await waitFor(() => expect(buy).toBeEnabled());
    fireEvent.click(buy);
    expect(await screen.findByText('钱包刷新失败')).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText('永久会员').length).toBeGreaterThan(0));
    expect(readerApi.getWallet).toHaveBeenCalledTimes(2);
    expect(readerApi.getEntitlements).toHaveBeenCalledTimes(2);
  });

  it('opens a recharge dialog without buying when the current diamond balance is insufficient', async () => {
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', 30, '会员月卡', 121)
    ]);
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员月卡，121钻石' });
    await waitFor(() => expect(buy).toBeEnabled());

    fireEvent.click(buy);
    expect(await screen.findByRole('dialog', { name: '钻石余额不足' })).toBeInTheDocument();
    expect(readerApi.buyMembership).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '去充值' }));
    expect(await screen.findByText('充值页')).toBeInTheDocument();
  });

  it('opens the recharge dialog when the server reports a concurrent insufficient balance', async () => {
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', 7, '会员周卡', 30)
    ]);
    readerApi.buyMembership.mockRejectedValueOnce(new Error('钻石余额不足'));
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员周卡，30钻石' });
    await waitFor(() => expect(buy).toBeEnabled());

    fireEvent.click(buy);
    expect(await screen.findByRole('dialog', { name: '钻石余额不足' })).toBeInTheDocument();
    expect(readerApi.buyMembership).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('钻石余额不足', { selector: '.me-inline-error' })).not.toBeInTheDocument();
  });

  it('shows membership purchase failure only in the membership section', async () => {
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', 7, '会员周卡', 30)
    ]);
    readerApi.buyMembership.mockRejectedValueOnce(new Error('会员购买失败'));
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员周卡，30钻石' });
    await waitFor(() => expect(buy).toBeEnabled());
    fireEvent.click(buy);
    expect(await screen.findByText('会员购买失败')).toBeInTheDocument();
    expect(screen.getByText('MBABCDEFGH')).toBeInTheDocument();
  });

  it('updates checkin first and lets only the wallet own its refresh failure', async () => {
    readerApi.getWallet
      .mockResolvedValueOnce({ readerId: '9223372036854775801', rechargeCoinBalance: 120, bonusCoinBalance: 35, expiringBonusCoin: 5, totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0, totalBonusCoinExpense: 0 })
      .mockRejectedValueOnce(new Error('签到后钱包刷新失败'))
      .mockResolvedValueOnce({ readerId: '9223372036854775801', rechargeCoinBalance: 120, bonusCoinBalance: '9007199254740992', expiringBonusCoin: 5, totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0, totalBonusCoinExpense: 0 });
    renderMePage();
    const button = await screen.findByRole('button', { name: '签到' });
    vi.useFakeTimers();
    const frames = installAnimationFrameController();
    fireEvent.click(button);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    const reward = celebrationLayer();
    expect(reward).toHaveTextContent('+0金币');
    frames.runAll();
    expect(reward).toHaveTextContent('+60金币');
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 35');
    expect(screen.getByText('签到后钱包刷新失败')).toBeInTheDocument();
    const frameCallsBeforeRetry = vi.mocked(requestAnimationFrame).mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '重新加载钱包' }));
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 9,007,199,254,740,992');
    expect(requestAnimationFrame).toHaveBeenCalledTimes(frameCallsBeforeRetry);
    expect(readerApi.getWallet).toHaveBeenCalledTimes(3);
    act(() => vi.advanceTimersByTime(800));
    expect(screen.getByRole('button', { name: '已签到' })).toBeDisabled();
    expect(readerApi.ensureInviteDashboard).toHaveBeenCalledTimes(1);
    expect(readerApi.getEntitlements).toHaveBeenCalledTimes(1);
  });

  it('shows a membership-purchase wallet refresh immediately without a checkin bonus animation', async () => {
    readerApi.listMembershipProducts.mockResolvedValueOnce([
      product('9223372036854775809', 30, '会员月卡', 99)
    ]);
    readerApi.getWallet
      .mockResolvedValueOnce({ readerId: '9223372036854775801', rechargeCoinBalance: 120, bonusCoinBalance: '35', expiringBonusCoin: 5, totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0, totalBonusCoinExpense: 0 })
      .mockResolvedValueOnce({ readerId: '9223372036854775801', rechargeCoinBalance: 20, bonusCoinBalance: '10', expiringBonusCoin: 5, totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0, totalBonusCoinExpense: 0 });
    readerApi.buyMembership.mockResolvedValueOnce({ status: 'paid' });
    renderMePage();
    const buy = await screen.findByRole('button', { name: '购买会员月卡，99钻石' });
    await waitFor(() => expect(buy).toBeEnabled());
    vi.useFakeTimers();
    installAnimationFrameController();
    fireEvent.click(buy);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); });
    expect(balance('金币')).toHaveAccessibleName('查看金币明细，当前余额 10');
    expect(requestAnimationFrame).not.toHaveBeenCalled();
  });

  it('shows the final reward immediately for reduced motion and then restores the checked-in card', async () => {
    setReducedMotion(true);
    renderMePage();
    const button = await screen.findByRole('button', { name: '签到' });
    vi.useFakeTimers();
    installAnimationFrameController();
    fireEvent.click(button);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(celebrationLayer()).toHaveTextContent('+60金币');
    expect(document.querySelector('.me-checkin-announcement')).toHaveTextContent('签到成功，获得60金币');
    expect(requestAnimationFrame).not.toHaveBeenCalled();
    act(() => vi.runAllTimers());
    expect(screen.getByRole('button', { name: '已签到' })).toBeDisabled();
  });

  it('clears the celebration timer when unmounted', async () => {
    const { unmount } = renderMePage();
    const button = await screen.findByRole('button', { name: '签到' });
    vi.useFakeTimers();
    installAnimationFrameController();
    fireEvent.click(button);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(celebrationLayer()).toHaveAttribute('aria-hidden', 'true');
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not refresh wallet after a pending checkin resolves after unmount', async () => {
    const checkinRequest = deferred<Awaited<ReturnType<typeof readerApi.checkin>>>();
    readerApi.checkin.mockReturnValueOnce(checkinRequest.promise);
    const { unmount } = renderMePage();
    const checkinButton = await screen.findByRole('button', { name: '签到' });

    fireEvent.click(checkinButton);
    expect(readerApi.checkin).toHaveBeenCalledTimes(1);
    unmount();
    await act(async () => {
      checkinRequest.resolve({
        todayChecked: true,
        continuousDays: 7,
        todayRewardCoin: 60,
        rewardRandom: false,
        checkinAvailable: true
      });
      await checkinRequest.promise;
    });

    expect(readerApi.getWallet).toHaveBeenCalledTimes(1);
  });
});
