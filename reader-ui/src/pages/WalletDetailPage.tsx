import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { queryWalletLedgers } from '../api/reader';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { getStoredReaderToken } from '../auth/session';
import { AppShell } from '../components/AppShell';
import { CurrencyIcon } from '../components/CurrencyIcon';
import type { ReaderWallet, ReaderWalletCoinType, ReaderWalletLedger } from '../types/reader';
import { formatReaderLong } from '../utils/formatReaderLong';

const PAGE_SIZE = 20;
const LEDGER_ERROR = '余额变更记录加载失败';

const pageConfig = {
  recharge: {
    title: '钻石明细',
    balanceLabel: '当前钻石',
    route: '/me/diamonds'
  },
  bonus: {
    title: '金币明细',
    balanceLabel: '当前金币',
    route: '/me/coins'
  }
} satisfies Record<ReaderWalletCoinType, { title: string; balanceLabel: string; route: string }>;

const bizTypeLabels: Record<string, string> = {
  epusdt_recharge: 'USDT 充值',
  checkin: '签到奖励',
  invite_register_reward: '邀请注册奖励',
  invite_first_recharge_reward: '邀请首充奖励',
  book_purchase: '整书购买',
  chapter_purchase: '章节购买',
  membership_purchase: '会员购买',
  ad_free_purchase: '免广告购买',
  bonus_expire: '金币过期',
  wallet_adjustment: '余额调整'
};

interface LedgerState {
  token: string;
  rows: ReaderWalletLedger[];
  total: number;
  page: number;
  loading: boolean;
  loadingMore: boolean;
  failedPage: number | null;
}

interface AuthoritativeWalletState {
  token: string;
  data: ReaderWallet | null;
  loading: boolean;
  error: boolean;
}

function freshLedgerState(token: string, loading: boolean): LedgerState {
  return {
    token,
    rows: [],
    total: 0,
    page: 0,
    loading,
    loadingMore: false,
    failedPage: null
  };
}

function freshWalletState(token: string, loading: boolean): AuthoritativeWalletState {
  return { token, data: null, loading, error: false };
}

function ledgerDescription(ledger: ReaderWalletLedger) {
  const remark = ledger.remark?.trim();
  return remark || bizTypeLabels[ledger.bizType] || '余额变更';
}

function formatLedgerTime(value: string) {
  const match = /^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2})/.exec(value.trim());
  return match ? `${match[1]} ${match[2]}` : value.replace('T', ' ');
}

export function WalletDetailPage({ coinType }: { coinType: ReaderWalletCoinType }) {
  const navigate = useNavigate();
  const {
    isAuthenticated,
    refreshWallet,
    sessionReady,
    token
  } = useReaderAuth();
  const config = pageConfig[coinType];
  const mountedRef = useRef(true);
  const tokenRef = useRef(token);
  const requestSequence = useRef(0);
  const requestInFlight = useRef(false);
  const walletRequestSequence = useRef(0);
  const ledgerRetryRef = useRef<HTMLButtonElement>(null);
  const [ledgerState, setLedgerState] = useState<LedgerState>(() => freshLedgerState(token, Boolean(token)));
  const [walletState, setWalletState] = useState<AuthoritativeWalletState>(() => freshWalletState(token, Boolean(token)));
  tokenRef.current = token;

  const refreshAuthoritativeWallet = useCallback(async (requestToken: string) => {
    if (!requestToken) return;
    const requestId = ++walletRequestSequence.current;
    setWalletState(freshWalletState(requestToken, true));
    const canPublish = () => mountedRef.current
      && walletRequestSequence.current === requestId
      && tokenRef.current === requestToken
      && getStoredReaderToken() === requestToken;

    try {
      const nextWallet = await refreshWallet();
      if (!canPublish()) return;
      setWalletState(nextWallet
        ? { token: requestToken, data: nextWallet, loading: false, error: false }
        : { token: requestToken, data: null, loading: false, error: true });
    } catch {
      if (canPublish()) {
        setWalletState({ token: requestToken, data: null, loading: false, error: true });
      }
    }
  }, [refreshWallet]);

  const loadPage = useCallback(async (page: number, requestToken: string, replace: boolean) => {
    if (!requestToken || requestInFlight.current) return;

    requestInFlight.current = true;
    const requestId = ++requestSequence.current;
    setLedgerState((current) => ({
      ...(replace ? freshLedgerState(requestToken, true) : current),
      token: requestToken,
      loading: replace,
      loadingMore: !replace,
      failedPage: null
    }));

    const canPublish = () => mountedRef.current
      && requestSequence.current === requestId
      && tokenRef.current === requestToken
      && getStoredReaderToken() === requestToken;

    try {
      const result = await queryWalletLedgers({ coinType, pageNum: page, pageSize: PAGE_SIZE });
      if (!canPublish()) return;
      setLedgerState((current) => ({
        token: requestToken,
        rows: replace ? result.rows : [...current.rows, ...result.rows],
        total: result.total,
        page,
        loading: false,
        loadingMore: false,
        failedPage: null
      }));
    } catch {
      if (!canPublish()) return;
      setLedgerState((current) => ({
        ...current,
        token: requestToken,
        loading: false,
        loadingMore: false,
        failedPage: page
      }));
    } finally {
      if (requestSequence.current === requestId) requestInFlight.current = false;
    }
  }, [coinType]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      requestSequence.current += 1;
      walletRequestSequence.current += 1;
      requestInFlight.current = false;
    };
  }, []);

  useEffect(() => {
    if (!sessionReady) return;
    requestSequence.current += 1;
    walletRequestSequence.current += 1;
    requestInFlight.current = false;
    setLedgerState(freshLedgerState(token, Boolean(token)));
    setWalletState(freshWalletState(token, Boolean(token)));

    if (!isAuthenticated || !token) {
      navigate(`/auth/login?redirect=${config.route}`, { replace: true });
      return;
    }

    void refreshAuthoritativeWallet(token);
    void loadPage(1, token, true);
  }, [config.route, isAuthenticated, loadPage, navigate, refreshAuthoritativeWallet, sessionReady, token]);

  const currentLedgerState = ledgerState.token === token
    ? ledgerState
    : freshLedgerState(token, Boolean(token));
  const currentWalletState = walletState.token === token
    ? walletState
    : freshWalletState(token, Boolean(token));
  const balance = currentWalletState.data
    ? coinType === 'recharge' ? currentWalletState.data.rechargeCoinBalance : currentWalletState.data.bonusCoinBalance
    : null;
  const retryLedgers = () => {
    void loadPage(currentLedgerState.failedPage || 1, token, (currentLedgerState.failedPage || 1) === 1);
  };

  useEffect(() => {
    if (ledgerState.token === token && ledgerState.failedPage !== null && ledgerState.failedPage > 1) {
      ledgerRetryRef.current?.focus();
    }
  }, [ledgerState.failedPage, ledgerState.token, token]);

  return (
    <AppShell title={config.title} back onBack={() => navigate('/me')} hideTabs>
      <main className="wallet-detail-page">
        <section
          className={`wallet-detail__summary${coinType === 'recharge' ? ' wallet-detail__summary--with-action' : ''}`}
          aria-label={config.balanceLabel}
          aria-busy={currentWalletState.loading}
        >
          <CurrencyIcon coinType={coinType} className="wallet-detail__icon" />
          <div>
            <div className="wallet-detail__balance-label">{config.balanceLabel}</div>
            {balance !== null ? (
              <div className="wallet-detail__balance" aria-live="polite">{formatReaderLong(balance)}</div>
            ) : currentWalletState.error ? (
              <div className="wallet-detail__balance-state" role="alert">当前余额加载失败</div>
            ) : (
              <div className="wallet-detail__summary-skeleton" role="status" aria-label="当前余额加载中">
                <span className="wallet-detail__summary-skeleton-value" aria-hidden="true" />
              </div>
            )}
            {currentWalletState.error ? (
              <button type="button" className="wallet-detail__balance-retry" onClick={() => { void refreshAuthoritativeWallet(token); }}>
                重试当前余额
              </button>
            ) : null}
          </div>
          {coinType === 'recharge' ? (
            <button type="button" className="wallet-detail__recharge" onClick={() => navigate('/me/recharge')}>
              USDT 充值
            </button>
          ) : null}
        </section>

        <section
          className="wallet-detail__ledgers"
          aria-labelledby="wallet-ledger-heading"
          aria-busy={currentLedgerState.loading || currentLedgerState.loadingMore}
        >
          <h2 id="wallet-ledger-heading" className="wallet-detail__ledger-title">余额变更记录</h2>
          {currentLedgerState.rows.length ? (
            <ul className="wallet-ledger-list" aria-label="余额变更记录" aria-live="polite">
              {currentLedgerState.rows.map((ledger) => (
                <li className="wallet-ledger-row" key={ledger.id}>
                  <div className="wallet-ledger__description">{ledgerDescription(ledger)}</div>
                  <time className="wallet-ledger__time" dateTime={ledger.createTime}>{formatLedgerTime(ledger.createTime)}</time>
                  <div className={`wallet-ledger__amount wallet-ledger__amount--${ledger.direction}`}>
                    {ledger.direction === 'income' ? '+' : '-'}{formatReaderLong(ledger.amount)}
                  </div>
                  <div className="wallet-ledger__balance">余额 {formatReaderLong(ledger.balanceAfter)}</div>
                </li>
              ))}
            </ul>
          ) : null}

          {currentLedgerState.loading ? (
            <div className="wallet-ledger-state" role="status" aria-label="余额变更记录加载中">余额变更记录加载中</div>
          ) : currentLedgerState.failedPage !== null ? (
            <>
              <div className="wallet-ledger-state" role="alert">{LEDGER_ERROR}</div>
              <button ref={ledgerRetryRef} type="button" className="wallet-ledger-more" onClick={retryLedgers}>重试余额变更记录</button>
            </>
          ) : currentLedgerState.rows.length === 0 ? (
            <div className="wallet-ledger-state" role="status" aria-label="暂无余额变更记录">暂无余额变更记录</div>
          ) : currentLedgerState.rows.length < currentLedgerState.total ? (
            <button
              type="button"
              className="wallet-ledger-more"
              disabled={currentLedgerState.loadingMore}
              onClick={() => { void loadPage(currentLedgerState.page + 1, token, false); }}
            >
              {currentLedgerState.loadingMore ? '加载中' : '加载更多'}
            </button>
          ) : null}
        </section>
      </main>
    </AppShell>
  );
}
