import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReaderWallet } from '../../types/reader';
import { useMeResource } from './useMeResource';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, resolve, reject };
}

function wallet(rechargeCoinBalance: number, bonusCoinBalance: number, expiringBonusCoin: number): ReaderWallet {
  return {
    readerId: '9223372036854775801',
    rechargeCoinBalance,
    bonusCoinBalance,
    expiringBonusCoin,
    totalRechargeCoinIncome: 0,
    totalBonusCoinIncome: 0,
    totalRechargeCoinExpense: 0,
    totalBonusCoinExpense: 0
  };
}

describe('useMeResource', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps stale data visible while retrying and replaces only its own error', async () => {
    const second = deferred<ReaderWallet>();
    const load = vi.fn()
      .mockResolvedValueOnce(wallet(10, 20, 0))
      .mockReturnValueOnce(second.promise);
    const { result } = renderHook(() => useMeResource<ReaderWallet>(load, true, '钱包加载失败', 'token-a'));
    await waitFor(() => expect(result.current.data?.rechargeCoinBalance).toBe(10));

    act(() => { void result.current.reload(); });
    expect(result.current.loading).toBe(true);
    expect(result.current.data?.rechargeCoinBalance).toBe(10);
    await act(async () => {
      second.resolve(wallet(30, 40, 0));
      await second.promise;
    });
    await waitFor(() => expect(result.current.data?.rechargeCoinBalance).toBe(30));
  });

  it('ignores a late response after a newer request wins', async () => {
    const first = deferred<ReaderWallet>();
    const second = deferred<ReaderWallet>();
    const load = vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const { result } = renderHook(() => useMeResource<ReaderWallet>(load, true, '钱包加载失败', 'token-a'));
    act(() => { void result.current.reload(); });
    await act(async () => {
      second.resolve(wallet(20, 0, 0));
      await second.promise;
    });
    await waitFor(() => expect(result.current.data?.rechargeCoinBalance).toBe(20));
    await act(async () => {
      first.resolve(wallet(10, 0, 0));
      await first.promise;
    });
    expect(result.current.data?.rechargeCoinBalance).toBe(20);
  });

  it('stores a local error and preserves stale data when reload fails', async () => {
    const load = vi.fn()
      .mockResolvedValueOnce(wallet(10, 20, 0))
      .mockRejectedValueOnce(new Error('钱包网络异常'));
    const { result } = renderHook(() => useMeResource<ReaderWallet>(load, true, '钱包加载失败', 'token-a'));
    await waitFor(() => expect(result.current.data).not.toBeNull());
    await act(async () => { await result.current.reload(); });
    expect(result.current.data?.rechargeCoinBalance).toBe(10);
    expect(result.current.error).toBe('钱包网络异常');
    expect(result.current.loading).toBe(false);
  });

  it('lets actions replace data or error and invalidates older requests', async () => {
    const pending = deferred<ReaderWallet>();
    const load = vi.fn(() => pending.promise);
    const { result } = renderHook(() => useMeResource<ReaderWallet>(load, true, '钱包加载失败', 'token-a'));
    act(() => { result.current.setData(wallet(50, 60, 0)); });
    expect(result.current.data?.rechargeCoinBalance).toBe(50);
    act(() => { result.current.setError('签到失败'); });
    expect(result.current.error).toBe('签到失败');
    await act(async () => {
      pending.resolve(wallet(1, 1, 0));
      await pending.promise;
    });
    expect(result.current.data?.rechargeCoinBalance).toBe(50);
    expect(result.current.error).toBe('签到失败');
  });

  it('does not auto reload for inline loader identity changes and manual reload uses the latest loader', async () => {
    const load = vi.fn((balance: number) => Promise.resolve(wallet(balance, 0, 0)));
    const { result, rerender } = renderHook(
      ({ loader }) => useMeResource(loader, true, '钱包加载失败', 'token-a'),
      { initialProps: { loader: () => load(10) } }
    );
    await waitFor(() => expect(result.current.data?.rechargeCoinBalance).toBe(10));
    expect(load).toHaveBeenCalledTimes(1);

    rerender({ loader: () => load(20) });
    expect(load).toHaveBeenCalledTimes(1);
    expect(result.current.data?.rechargeCoinBalance).toBe(10);

    await act(async () => { await result.current.reload(); });
    expect(load).toHaveBeenCalledTimes(2);
    expect(result.current.data?.rechargeCoinBalance).toBe(20);
  });
});

describe('useMeResource identity isolation', () => {
  it('clears old state, reloads for the new identity, and ignores the old late result', async () => {
    const requestA = deferred<string>();
    const requestB = deferred<string>();
    const loader = vi.fn()
      .mockReturnValueOnce(requestA.promise)
      .mockReturnValueOnce(requestB.promise);
    const renderSnapshots: Array<{ identity: string; data: string | null; error: string }> = [];
    const { result, rerender } = renderHook(
      ({ identity }) => {
        const resource = useMeResource<string>(loader, true, '加载失败', identity);
        renderSnapshots.push({ identity, data: resource.data, error: resource.error });
        return resource;
      },
      { initialProps: { identity: 'token-a' } }
    );
    await waitFor(() => expect(loader).toHaveBeenCalledTimes(1));
    act(() => {
      result.current.setData('A缓存');
      result.current.setError('A错误');
    });
    expect(result.current.data).toBe('A缓存');
    expect(result.current.error).toBe('A错误');

    renderSnapshots.length = 0;
    rerender({ identity: 'token-b' });
    expect(renderSnapshots[0]).toEqual({ identity: 'token-b', data: null, error: '' });
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBe('');
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(loader).toHaveBeenCalledTimes(2));

    await act(async () => {
      requestB.resolve('B数据');
      await requestB.promise;
    });
    expect(result.current.data).toBe('B数据');

    await act(async () => {
      requestA.resolve('A迟到数据');
      await requestA.promise;
    });
    expect(result.current.data).toBe('B数据');
    expect(result.current.error).toBe('');
  });
});
