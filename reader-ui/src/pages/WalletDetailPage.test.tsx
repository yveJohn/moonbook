import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { StrictMode } from 'react';
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ReaderAuthProvider, useReaderAuth } from '../auth/ReaderAuthContext';
import { READER_PROFILE_KEY, READER_TOKEN_KEY } from '../auth/session';
import type { ReaderWallet, ReaderWalletLedger } from '../types/reader';
import { WalletDetailPage } from './WalletDetailPage';

const readerApi = vi.hoisted(() => ({
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
  queryWalletLedgers: vi.fn(),
  registerReader: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, reject, resolve };
}

function wallet(rechargeCoinBalance = '9223372036854775807', bonusCoinBalance = '88'): ReaderWallet {
  return {
    readerId: '9223372036854775801',
    rechargeCoinBalance,
    bonusCoinBalance,
    totalRechargeCoinIncome: '0',
    totalBonusCoinIncome: '0',
    totalRechargeCoinExpense: '0',
    totalBonusCoinExpense: '0',
    expiringBonusCoin: '0'
  };
}

function ledger(overrides: Partial<ReaderWalletLedger> = {}): ReaderWalletLedger {
  return {
    id: '9223372036854775802',
    readerId: '9223372036854775801',
    ledgerNo: 'LD202607140001',
    bizType: 'checkin',
    bizId: null,
    orderNo: null,
    direction: 'income',
    coinType: 'recharge',
    amount: '30',
    balanceBefore: '70',
    balanceAfter: '100',
    remark: null,
    createTime: '2026-07-14T09:08:07',
    ...overrides
  };
}

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{`${location.pathname}${location.search}`}</output>;
}

function CachedWalletLauncher() {
  const navigate = useNavigate();
  const { refreshWallet } = useReaderAuth();
  return (
    <button
      type="button"
      onClick={() => { void refreshWallet().then(() => navigate('/me/diamonds')); }}
    >
      缓存余额后打开明细
    </button>
  );
}

function establishSession(token = 'reader.token.a') {
  window.localStorage.setItem(READER_TOKEN_KEY, token);
  window.localStorage.setItem(READER_PROFILE_KEY, JSON.stringify({
    readerId: token === 'reader.token.b' ? '9223372036854775899' : '9223372036854775801',
    username: token === 'reader.token.b' ? 'reader-b' : 'reader-a',
    nickname: token === 'reader.token.b' ? '读者乙' : '读者甲',
    status: 'enabled'
  }));
}

function switchSession(token: string) {
  const oldValue = window.localStorage.getItem(READER_TOKEN_KEY);
  establishSession(token);
  window.dispatchEvent(new StorageEvent('storage', {
    key: READER_TOKEN_KEY,
    oldValue,
    newValue: token,
    storageArea: window.localStorage
  }));
}

function renderWalletDetail(coinType: 'recharge' | 'bonus' = 'recharge', authenticated = true, strictMode = false) {
  const route = coinType === 'recharge' ? '/me/diamonds' : '/me/coins';
  if (authenticated) establishSession();
  const content = (
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route path={route} element={<WalletDetailPage coinType={coinType} />} />
          <Route path="/me" element={<div>个人中心</div>} />
          <Route path="/me/recharge" element={<div>充值页面</div>} />
          <Route path="/auth/login" element={<div>登录页面</div>} />
        </Routes>
        <LocationProbe />
      </MemoryRouter>
    </ReaderAuthProvider>
  );
  return render(strictMode ? <StrictMode>{content}</StrictMode> : content);
}

function renderWalletDetailAfterCachingWallet() {
  establishSession();
  return render(
    <ReaderAuthProvider>
      <MemoryRouter initialEntries={['/wallet-cache']}>
        <Routes>
          <Route path="/wallet-cache" element={<CachedWalletLauncher />} />
          <Route path="/me/diamonds" element={<WalletDetailPage coinType="recharge" />} />
        </Routes>
      </MemoryRouter>
    </ReaderAuthProvider>
  );
}

describe('WalletDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    readerApi.getWallet.mockResolvedValue(wallet());
    readerApi.queryWalletLedgers.mockResolvedValue({ rows: [], total: 0 });
  });

  afterEach(cleanup);

  it.each([
    ['recharge' as const, '/me/diamonds'],
    ['bonus' as const, '/me/coins']
  ])('redirects anonymous %s readers to the matching login return path without requesting wallet data', async (coinType, route) => {
    renderWalletDetail(coinType, false);

    expect(await screen.findByTestId('location')).toHaveTextContent(`/auth/login?redirect=${route}`);
    expect(readerApi.getWallet).not.toHaveBeenCalled();
    expect(readerApi.queryWalletLedgers).not.toHaveBeenCalled();
  });

  it('renders the authoritative full diamond balance and requests recharge ledgers', async () => {
    readerApi.getWallet.mockResolvedValue(wallet('9223372036854775807'));
    renderWalletDetail('recharge');

    expect(await screen.findByRole('heading', { level: 1, name: '钻石明细' })).toBeInTheDocument();
    expect(screen.getByText('当前钻石')).toHaveClass('wallet-detail__balance-label');
    expect(await screen.findByText('9,223,372,036,854,775,807')).toHaveClass('wallet-detail__balance');
    expect(readerApi.queryWalletLedgers).toHaveBeenCalledWith({ coinType: 'recharge', pageNum: 1, pageSize: 20 });
    const icon = document.querySelector('.wallet-detail__icon');
    expect(icon).toHaveAttribute('src', expect.stringContaining('moonbook-diamond.png'));
    expect(icon).toHaveAttribute('alt', '');
    expect(icon).toHaveAttribute('aria-hidden', 'true');

    fireEvent.click(screen.getByRole('button', { name: '返回' }));
    expect(await screen.findByTestId('location')).toHaveTextContent('/me');
  });

  it('places the recharge action inside the diamond summary and navigates to recharge', async () => {
    renderWalletDetail('recharge');

    const summary = await screen.findByRole('region', { name: '当前钻石' });
    const recharge = within(summary).getByRole('button', { name: 'USDT 充值' });
    expect(recharge.parentElement).toBe(summary);
    fireEvent.click(recharge);
    expect(await screen.findByTestId('location')).toHaveTextContent('/me/recharge');
  });

  it('uses the gold title, balance, icon, and bonus query configuration', async () => {
    readerApi.getWallet.mockResolvedValue(wallet('7', '8,888'));
    renderWalletDetail('bonus');

    expect(await screen.findByRole('heading', { level: 1, name: '金币明细' })).toBeInTheDocument();
    expect(screen.getByText('当前金币')).toHaveClass('wallet-detail__balance-label');
    expect(await screen.findByText('8,888')).toHaveClass('wallet-detail__balance');
    expect(readerApi.queryWalletLedgers).toHaveBeenCalledWith({ coinType: 'bonus', pageNum: 1, pageSize: 20 });
    const icon = document.querySelector('.wallet-detail__icon');
    expect(icon).toHaveAttribute('src', expect.stringContaining('moonbook-coin.png'));
    expect(icon).toHaveAttribute('alt', '');
    expect(icon).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('button', { name: 'USDT 充值' })).not.toBeInTheDocument();
  });

  it('renders a visible semantic heading for the ledger section', async () => {
    renderWalletDetail();

    expect(await screen.findByRole('heading', { level: 2, name: '余额变更记录' })).toBeVisible();
  });

  it('keeps wallet and ledger loading states independent and publishes each deferred result', async () => {
    const walletRequest = deferred<ReaderWallet>();
    const ledgerRequest = deferred<{ rows: ReaderWalletLedger[]; total: number }>();
    readerApi.getWallet.mockReturnValue(walletRequest.promise);
    readerApi.queryWalletLedgers.mockReturnValue(ledgerRequest.promise);
    renderWalletDetail();

    expect(await screen.findByRole('status', { name: '当前余额加载中' })).toHaveClass('wallet-detail__summary-skeleton');
    const ledgerRegion = screen.getByRole('region', { name: '余额变更记录' });
    expect(ledgerRegion).toHaveAttribute('aria-busy', 'true');

    await act(async () => {
      walletRequest.resolve(wallet('900'));
      await walletRequest.promise;
    });
    expect(await screen.findByText('900')).toHaveClass('wallet-detail__balance');
    expect(ledgerRegion).toHaveAttribute('aria-busy', 'true');
    expect(screen.getByText('余额变更记录加载中')).toBeInTheDocument();

    await act(async () => {
      ledgerRequest.resolve({ rows: [ledger({ remark: '延迟到账记录' })], total: 1 });
      await ledgerRequest.promise;
    });
    expect(await screen.findByText('延迟到账记录')).toBeInTheDocument();
    expect(ledgerRegion).toHaveAttribute('aria-busy', 'false');
    expect(screen.getByRole('list', { name: '余额变更记录' })).toHaveAttribute('aria-live', 'polite');
  });

  it.each([
    ['checkin', '签到奖励'],
    ['invite_register_reward', '邀请注册奖励'],
    ['invite_first_recharge_reward', '邀请首充奖励'],
    ['book_purchase', '整书购买'],
    ['chapter_purchase', '章节购买'],
    ['membership_purchase', '会员购买'],
    ['ad_free_purchase', '免广告购买'],
    ['bonus_expire', '金币过期'],
    ['wallet_adjustment', '余额调整']
  ])('maps the %s business type to %s', async (bizType, label) => {
    readerApi.queryWalletLedgers.mockResolvedValue({
      total: 1,
      rows: [ledger({ remark: '   ', bizType })]
    });
    renderWalletDetail();

    const list = await screen.findByRole('list', { name: '余额变更记录' });
    expect(within(list).getByText(label)).toBeInTheDocument();
  });

  it('prefers a trimmed non-empty remark over a known business type', async () => {
    readerApi.queryWalletLedgers.mockResolvedValue({
      total: 1,
      rows: [ledger({ remark: '  人工补发  ', bizType: 'wallet_adjustment' })]
    });
    renderWalletDetail();

    const list = await screen.findByRole('list', { name: '余额变更记录' });
    expect(within(list).getByText('人工补发')).toBeInTheDocument();
    expect(within(list).queryByText('wallet_adjustment')).not.toBeInTheDocument();
    expect(within(list).queryByText('余额调整')).not.toBeInTheDocument();
  });

  it('uses the generic description for an unknown business type without leaking its code', async () => {
    readerApi.queryWalletLedgers.mockResolvedValue({
      total: 1,
      rows: [ledger({ remark: null, bizType: 'private_internal_code' })]
    });
    renderWalletDetail();

    const list = await screen.findByRole('list', { name: '余额变更记录' });
    expect(within(list).getByText('余额变更')).toBeInTheDocument();
    expect(within(list).queryByText('private_internal_code')).not.toBeInTheDocument();
  });

  it('renders signed long amounts, formatted occurrence time, and resulting balance without numeric coercion', async () => {
    readerApi.queryWalletLedgers.mockResolvedValue({
      total: 2,
      rows: [
        ledger({ id: '1', amount: '9223372036854775807', balanceAfter: '9223372036854775806' }),
        ledger({ id: '2', direction: 'expense', amount: '9223372036854775805', balanceAfter: '1', createTime: '2026-07-13 18:07:06' })
      ]
    });
    renderWalletDetail();

    expect(await screen.findByText('+9,223,372,036,854,775,807')).toHaveClass('wallet-ledger__amount', 'wallet-ledger__amount--income');
    expect(screen.getByText('-9,223,372,036,854,775,805')).toHaveClass('wallet-ledger__amount', 'wallet-ledger__amount--expense');
    expect(screen.getByText('2026-07-14 09:08')).toHaveClass('wallet-ledger__time');
    expect(screen.getByText('余额 9,223,372,036,854,775,806')).toHaveClass('wallet-ledger__balance');
  });

  it('shows the first-page empty state and no bottom main navigation', async () => {
    renderWalletDetail();

    expect(await screen.findByRole('status', { name: '暂无余额变更记录' })).toHaveClass('wallet-ledger-state');
    expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument();
  });

  it('publishes wallet ledgers when development StrictMode replays effects', async () => {
    readerApi.queryWalletLedgers.mockResolvedValue({ rows: [ledger({ remark: '严格模式记录' })], total: 1 });
    renderWalletDetail('recharge', true, true);

    expect(await screen.findByText('严格模式记录')).toBeInTheDocument();
  });

  it('retries a safe first-page failure independently from the wallet balance', async () => {
    readerApi.queryWalletLedgers
      .mockRejectedValueOnce(new Error('internal database details'))
      .mockResolvedValueOnce({ rows: [ledger()], total: 1 });
    renderWalletDetail();

    expect(await screen.findByRole('alert')).toHaveTextContent('余额变更记录加载失败');
    expect(screen.queryByText('internal database details')).not.toBeInTheDocument();
    expect(await screen.findByText('9,223,372,036,854,775,807')).toHaveClass('wallet-detail__balance');

    const retry = screen.getByRole('button', { name: '重试余额变更记录' });
    expect(retry).not.toHaveFocus();
    fireEvent.click(retry);
    expect(await screen.findByText('签到奖励')).toBeInTheDocument();
    expect(readerApi.queryWalletLedgers).toHaveBeenCalledTimes(2);
  });

  it('shows wallet failure and retry without replacing a successful ledger list', async () => {
    readerApi.getWallet
      .mockRejectedValueOnce(new Error('unsafe wallet error'))
      .mockResolvedValueOnce(wallet('500'));
    readerApi.queryWalletLedgers.mockResolvedValue({ rows: [ledger()], total: 1 });
    renderWalletDetail();

    expect(await screen.findByRole('alert')).toHaveTextContent('当前余额加载失败');
    expect(screen.queryByText('unsafe wallet error')).not.toBeInTheDocument();
    expect(screen.getByText('签到奖励')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '重试当前余额' }));
    expect(await screen.findByText('500')).toHaveClass('wallet-detail__balance');
  });

  it('never presents a cached wallet as authoritative after refresh fails and replaces it after retry', async () => {
    readerApi.getWallet
      .mockResolvedValueOnce(wallet('120'))
      .mockRejectedValueOnce(new Error('refresh failed'))
      .mockResolvedValueOnce(wallet('500'));
    renderWalletDetailAfterCachingWallet();

    fireEvent.click(screen.getByRole('button', { name: '缓存余额后打开明细' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('当前余额加载失败');
    expect(screen.queryByText('120')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '重试当前余额' }));
    expect(await screen.findByText('500')).toHaveClass('wallet-detail__balance');
    expect(screen.queryByRole('alert', { name: '当前余额加载失败' })).not.toBeInTheDocument();
  });

  it('appends the next page with the same filter', async () => {
    readerApi.queryWalletLedgers
      .mockResolvedValueOnce({ rows: [ledger({ id: '1', remark: '第一页' })], total: 2 })
      .mockResolvedValueOnce({ rows: [ledger({ id: '2', remark: '第二页' })], total: 2 });
    renderWalletDetail('bonus');

    fireEvent.click(await screen.findByRole('button', { name: '加载更多' }));

    expect(await screen.findByText('第二页')).toBeInTheDocument();
    expect(screen.getByText('第一页')).toBeInTheDocument();
    expect(readerApi.queryWalletLedgers).toHaveBeenLastCalledWith({ coinType: 'bonus', pageNum: 2, pageSize: 20 });
  });

  it('keeps existing rows when loading more fails and retries the failed page', async () => {
    readerApi.queryWalletLedgers
      .mockResolvedValueOnce({ rows: [ledger({ id: '1', remark: '保留记录' })], total: 2 })
      .mockRejectedValueOnce(new Error('more failed'))
      .mockResolvedValueOnce({ rows: [ledger({ id: '2', remark: '重试记录' })], total: 2 });
    renderWalletDetail();

    fireEvent.click(await screen.findByRole('button', { name: '加载更多' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('余额变更记录加载失败');
    expect(screen.getByText('保留记录')).toBeInTheDocument();

    const retry = screen.getByRole('button', { name: '重试余额变更记录' });
    expect(retry).toHaveFocus();
    fireEvent.click(retry);
    expect(await screen.findByText('重试记录')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(readerApi.queryWalletLedgers).toHaveBeenLastCalledWith({ coinType: 'recharge', pageNum: 2, pageSize: 20 });
  });

  it('prevents duplicate load-more requests while one is pending', async () => {
    const nextPage = deferred<{ rows: ReaderWalletLedger[]; total: number }>();
    readerApi.queryWalletLedgers
      .mockResolvedValueOnce({ rows: [ledger({ id: '1' })], total: 2 })
      .mockReturnValueOnce(nextPage.promise);
    renderWalletDetail();

    const more = await screen.findByRole('button', { name: '加载更多' });
    fireEvent.click(more);
    fireEvent.click(more);
    expect(readerApi.queryWalletLedgers).toHaveBeenCalledTimes(2);

    await act(async () => {
      nextPage.resolve({ rows: [ledger({ id: '2' })], total: 2 });
      await nextPage.promise;
    });
  });

  it('hides previous-account state immediately and never publishes its late response', async () => {
    const oldRequest = deferred<{ rows: ReaderWalletLedger[]; total: number }>();
    readerApi.queryWalletLedgers
      .mockResolvedValueOnce({ rows: [ledger({ id: '1', remark: '旧账户已有记录' })], total: 2 })
      .mockReturnValueOnce(oldRequest.promise)
      .mockResolvedValueOnce({ rows: [ledger({ id: '2', remark: '新账户记录' })], total: 1 });
    renderWalletDetail();

    fireEvent.click(await screen.findByRole('button', { name: '加载更多' }));
    act(() => switchSession('reader.token.b'));
    expect(screen.queryByText('余额变更记录加载失败')).not.toBeInTheDocument();
    expect(screen.queryByText('旧账户已有记录')).not.toBeInTheDocument();
    expect(await screen.findByText('新账户记录')).toBeInTheDocument();

    await act(async () => {
      oldRequest.resolve({ rows: [ledger({ id: '3', remark: '旧账户迟到记录' })], total: 2 });
      await oldRequest.promise;
    });
    await waitFor(() => expect(screen.queryByText('旧账户迟到记录')).not.toBeInTheDocument());
  });

  it('does not publish a deferred ledger response after the page unmounts', async () => {
    const pending = deferred<{ rows: ReaderWalletLedger[]; total: number }>();
    readerApi.queryWalletLedgers.mockReturnValue(pending.promise);
    renderWalletDetail();
    await waitFor(() => expect(readerApi.queryWalletLedgers).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: '返回' }));
    expect(await screen.findByText('个人中心')).toBeInTheDocument();

    await act(async () => {
      pending.resolve({ rows: [ledger({ remark: '卸载后迟到记录' })], total: 1 });
      await pending.promise;
    });
    expect(screen.getByText('个人中心')).toBeInTheDocument();
    expect(screen.queryByText('卸载后迟到记录')).not.toBeInTheDocument();
  });
});
