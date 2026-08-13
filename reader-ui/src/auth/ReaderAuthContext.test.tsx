import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useEffect } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ReaderLoginResult, ReaderProfile, ReaderWallet } from '../types/reader';
import { ReaderAuthProvider, useReaderAuth } from './ReaderAuthContext';

const readerApi = vi.hoisted(() => ({
  getReaderProfile: vi.fn(),
  getWallet: vi.fn(),
  loginReader: vi.fn(),
  logoutReader: vi.fn(),
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

function wallet(balance: string): ReaderWallet {
  return {
    readerId: '9223372036854775801', rechargeCoinBalance: balance, bonusCoinBalance: 0,
    totalRechargeCoinIncome: 0, totalBonusCoinIncome: 0, totalRechargeCoinExpense: 0,
    totalBonusCoinExpense: 0, expiringBonusCoin: 0
  };
}

function loginResult(accessToken: string, nickname: string): ReaderLoginResult {
  return {
    accessToken,
    expireIn: 7200,
    reader: { readerId: '9223372036854775802', username: 'new-reader', nickname, status: 'enabled' }
  };
}

function WalletHarness() {
  const {
    clearSession,
    clearSessionIfCurrent,
    isAuthenticated,
    login,
    logout,
    profile,
    refreshProfile,
    refreshWallet,
    sessionReady,
    token,
    wallet: value,
    walletLoading
  } = useReaderAuth();
  return (
    <div>
      <output data-testid="wallet">{value?.rechargeCoinBalance ?? ''}</output>
      <output data-testid="loading">{String(walletLoading)}</output>
      <output data-testid="authenticated">{String(isAuthenticated)}</output>
      <output data-testid="session-ready">{String(sessionReady)}</output>
      <output data-testid="token">{token}</output>
      <output data-testid="nickname">{profile?.nickname ?? ''}</output>
      <button type="button" onClick={() => { void refreshWallet(); }}>刷新钱包</button>
      <button type="button" onClick={() => { void refreshProfile(); }}>刷新资料</button>
      <button type="button" onClick={() => { void login('new-reader', 'password').catch(() => undefined); }}>新账号登录</button>
      <button type="button" onClick={clearSession}>清理会话</button>
      <button type="button" onClick={() => clearSessionIfCurrent('test-token')}>按当前令牌清理</button>
      <button type="button" onClick={() => clearSessionIfCurrent('stale-token')}>按过期令牌清理</button>
      <button type="button" onClick={() => { void logout().catch(() => undefined); }}>退出登录</button>
    </div>
  );
}

function DeferredSessionClear({ expectedToken, request }: { expectedToken: string; request: Promise<void> }) {
  const { clearSessionIfCurrent } = useReaderAuth();
  useEffect(() => {
    void request.then(() => clearSessionIfCurrent(expectedToken));
  }, [clearSessionIfCurrent, expectedToken, request]);
  return null;
}

describe('ReaderAuthContext wallet state', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    window.localStorage.setItem('readerToken', 'test-token');
    window.localStorage.setItem('readerProfile', JSON.stringify({ readerId: '1', username: 'reader', nickname: '读者', status: 'enabled' }));
    readerApi.logoutReader.mockResolvedValue(undefined);
  });

  afterEach(cleanup);

  it('restores a deferred browser session before publishing it as ready', async () => {
    const observedReadyStates: boolean[] = [];

    function ReadyObserver() {
      const { sessionReady } = useReaderAuth();
      useEffect(() => {
        observedReadyStates.push(sessionReady);
      }, [sessionReady]);
      return <WalletHarness />;
    }

    render(<ReaderAuthProvider deferClientSession><ReadyObserver /></ReaderAuthProvider>);

    expect(observedReadyStates).toContain(false);
    await waitFor(() => expect(screen.getByTestId('session-ready')).toHaveTextContent('true'));
    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('token')).toHaveTextContent('test-token');
    expect(observedReadyStates).toEqual([false, true]);
  });

  it('publishes a deferred session without a token as ready and anonymous', async () => {
    window.localStorage.clear();

    render(<ReaderAuthProvider deferClientSession><WalletHarness /></ReaderAuthProvider>);

    await waitFor(() => expect(screen.getByTestId('session-ready')).toHaveTextContent('true'));
    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('token')).toBeEmptyDOMElement();
  });

  it('keeps the latest wallet refresh when requests resolve out of order', async () => {
    const first = deferred<ReaderWallet>();
    const second = deferred<ReaderWallet>();
    readerApi.getWallet.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '刷新钱包' }));
    fireEvent.click(screen.getByRole('button', { name: '刷新钱包' }));
    await act(async () => { second.resolve(wallet('20')); await second.promise; });
    expect(screen.getByTestId('wallet')).toHaveTextContent('20');
    expect(screen.getByTestId('loading')).toHaveTextContent('false');

    await act(async () => { first.resolve(wallet('10')); await first.promise; });
    expect(screen.getByTestId('wallet')).toHaveTextContent('20');
  });

  it('does not publish a pending wallet response after logout', async () => {
    const pending = deferred<ReaderWallet>();
    readerApi.getWallet.mockReturnValueOnce(pending.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '刷新钱包' }));
    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    await waitFor(() => expect(screen.getByTestId('authenticated')).toHaveTextContent('false'));
    await act(async () => { pending.resolve(wallet('99')); await pending.promise; });

    expect(screen.getByTestId('wallet')).toHaveTextContent('');
    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
  });

  it('clears the loaded wallet and local session without remote logout', async () => {
    readerApi.getWallet.mockResolvedValueOnce(wallet('66'));
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '刷新钱包' }));
    await waitFor(() => expect(screen.getByTestId('wallet')).toHaveTextContent('66'));
    fireEvent.click(screen.getByRole('button', { name: '清理会话' }));

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('wallet')).toBeEmptyDOMElement();
    expect(screen.getByTestId('loading')).toHaveTextContent('false');
    expect(window.localStorage.getItem('readerToken')).toBeNull();
    expect(window.localStorage.getItem('readerProfile')).toBeNull();
    expect(readerApi.logoutReader).not.toHaveBeenCalled();
  });

  it('clears only when the persisted token matches the expected session', () => {
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '按过期令牌清理' }));
    expect(window.localStorage.getItem('readerToken')).toBe('test-token');
    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');

    fireEvent.click(screen.getByRole('button', { name: '按当前令牌清理' }));
    expect(window.localStorage.getItem('readerToken')).toBeNull();
    expect(window.localStorage.getItem('readerProfile')).toBeNull();
    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
  });

  it('clears the matching persisted session after the provider unmounts', async () => {
    const request = deferred<void>();
    const view = render(
      <ReaderAuthProvider>
        <DeferredSessionClear expectedToken="test-token" request={request.promise} />
      </ReaderAuthProvider>
    );
    view.unmount();

    await act(async () => {
      request.resolve();
      await request.promise;
    });

    expect(window.localStorage.getItem('readerToken')).toBeNull();
    expect(window.localStorage.getItem('readerProfile')).toBeNull();
  });

  it('does not publish a pending wallet response after clearing the session', async () => {
    const pending = deferred<ReaderWallet>();
    readerApi.getWallet.mockReturnValueOnce(pending.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '刷新钱包' }));
    expect(screen.getByTestId('loading')).toHaveTextContent('true');
    fireEvent.click(screen.getByRole('button', { name: '清理会话' }));
    expect(screen.getByTestId('loading')).toHaveTextContent('false');
    await act(async () => { pending.resolve(wallet('99')); await pending.promise; });

    expect(screen.getByTestId('wallet')).toBeEmptyDOMElement();
    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(readerApi.logoutReader).not.toHaveBeenCalled();
  });

  it('does not restore a profile after clearing its pending session', async () => {
    const pending = deferred<ReaderProfile>();
    readerApi.getReaderProfile.mockReturnValueOnce(pending.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '刷新资料' }));
    fireEvent.click(screen.getByRole('button', { name: '清理会话' }));
    await act(async () => {
      pending.resolve({ readerId: '1', username: 'reader', nickname: '过期资料', status: 'enabled' });
      await pending.promise;
    });

    expect(screen.getByTestId('nickname')).toBeEmptyDOMElement();
    expect(window.localStorage.getItem('readerProfile')).toBeNull();
  });

  it('does not authenticate after clearing a pending login', async () => {
    const pending = deferred<ReaderLoginResult>();
    readerApi.loginReader.mockReturnValueOnce(pending.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '新账号登录' }));
    fireEvent.click(screen.getByRole('button', { name: '清理会话' }));
    await act(async () => { pending.resolve(loginResult('stale-token', '过期登录')); await pending.promise; });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('nickname')).toBeEmptyDOMElement();
    expect(window.localStorage.getItem('readerToken')).toBeNull();
  });

  it('does not let a pending logout clear a newer login', async () => {
    const pendingLogout = deferred<void>();
    readerApi.logoutReader.mockReturnValueOnce(pendingLogout.promise);
    readerApi.loginReader.mockResolvedValueOnce(loginResult('new-token', '新账号'));
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    fireEvent.click(screen.getByRole('button', { name: '新账号登录' }));
    await waitFor(() => expect(screen.getByTestId('token')).toHaveTextContent('new-token'));
    await act(async () => { pendingLogout.resolve(); await pendingLogout.promise; });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('token')).toHaveTextContent('new-token');
    expect(screen.getByTestId('nickname')).toHaveTextContent('新账号');
    expect(window.localStorage.getItem('readerToken')).toBe('new-token');
  });

  it('lets a newer pending login commit after logout clears the old session', async () => {
    const pendingLogout = deferred<void>();
    const pendingLogin = deferred<ReaderLoginResult>();
    readerApi.logoutReader.mockReturnValueOnce(pendingLogout.promise);
    readerApi.loginReader.mockReturnValueOnce(pendingLogin.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    fireEvent.click(screen.getByRole('button', { name: '新账号登录' }));
    await act(async () => { pendingLogout.resolve(); await pendingLogout.promise; });
    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');

    await act(async () => {
      pendingLogin.resolve(loginResult('new-token', '新账号'));
      await pendingLogin.promise;
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('token')).toHaveTextContent('new-token');
    expect(screen.getByTestId('nickname')).toHaveTextContent('新账号');
    expect(window.localStorage.getItem('readerToken')).toBe('new-token');
  });

  it('clears the original session when a newer login attempt fails', async () => {
    const pendingLogout = deferred<void>();
    readerApi.logoutReader.mockReturnValueOnce(pendingLogout.promise);
    readerApi.loginReader.mockRejectedValueOnce(new Error('登录失败'));
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    fireEvent.click(screen.getByRole('button', { name: '新账号登录' }));
    await waitFor(() => expect(readerApi.loginReader).toHaveBeenCalledTimes(1));
    await act(async () => { pendingLogout.resolve(); await pendingLogout.promise; });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(window.localStorage.getItem('readerToken')).toBeNull();
    expect(window.localStorage.getItem('readerProfile')).toBeNull();
  });

  it('clears persisted session when logout rejects after provider unmount', async () => {
    const pendingLogout = deferred<void>();
    readerApi.logoutReader.mockReturnValueOnce(pendingLogout.promise);
    const view = render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    view.unmount();
    await act(async () => {
      pendingLogout.reject(new Error('退出请求失败'));
      await pendingLogout.promise.catch(() => undefined);
    });

    expect(window.localStorage.getItem('readerToken')).toBeNull();
    expect(window.localStorage.getItem('readerProfile')).toBeNull();
  });

  it('does not let a pending logout clear a storage-switched session', async () => {
    const pendingLogout = deferred<void>();
    readerApi.logoutReader.mockReturnValueOnce(pendingLogout.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    window.localStorage.setItem('readerToken', 'other-reader-token');
    window.localStorage.setItem('readerProfile', JSON.stringify({
      readerId: '2', username: 'other-reader', nickname: '其他账号', status: 'enabled'
    }));
    window.dispatchEvent(new StorageEvent('storage', {
      key: 'readerToken',
      oldValue: 'test-token',
      newValue: 'other-reader-token',
      storageArea: window.localStorage
    }));
    await act(async () => { pendingLogout.resolve(); await pendingLogout.promise; });

    expect(screen.getByTestId('token')).toHaveTextContent('other-reader-token');
    expect(screen.getByTestId('nickname')).toHaveTextContent('其他账号');
    expect(window.localStorage.getItem('readerToken')).toBe('other-reader-token');
  });

  it('clears the previous wallet when storage switches directly to another account', async () => {
    readerApi.getWallet.mockResolvedValueOnce(wallet('77'));
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);
    fireEvent.click(screen.getByRole('button', { name: '刷新钱包' }));
    await waitFor(() => expect(screen.getByTestId('wallet')).toHaveTextContent('77'));

    window.localStorage.setItem('readerToken', 'other-reader-token');
    window.dispatchEvent(new StorageEvent('storage', {
      key: 'readerToken',
      oldValue: 'test-token',
      newValue: 'other-reader-token',
      storageArea: window.localStorage
    }));

    await waitFor(() => expect(screen.getByTestId('wallet')).toBeEmptyDOMElement());
  });

  it('keeps a pending wallet refresh valid for a profile-only storage event', async () => {
    const pending = deferred<ReaderWallet>();
    readerApi.getWallet.mockReturnValueOnce(pending.promise);
    render(<ReaderAuthProvider><WalletHarness /></ReaderAuthProvider>);
    fireEvent.click(screen.getByRole('button', { name: '刷新钱包' }));

    window.localStorage.setItem('readerProfile', JSON.stringify({ readerId: '1', username: 'reader', nickname: '新昵称', status: 'enabled' }));
    window.dispatchEvent(new StorageEvent('storage', {
      key: 'readerProfile',
      oldValue: null,
      newValue: window.localStorage.getItem('readerProfile'),
      storageArea: window.localStorage
    }));
    expect(screen.getByTestId('loading')).toHaveTextContent('true');

    await act(async () => { pending.resolve(wallet('88')); await pending.promise; });
    expect(screen.getByTestId('wallet')).toHaveTextContent('88');
    expect(screen.getByTestId('loading')).toHaveTextContent('false');
  });
});
