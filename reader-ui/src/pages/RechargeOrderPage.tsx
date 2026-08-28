import { useCallback, useEffect, useRef, useState } from 'react';
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Clock3,
  Copy,
  Gem,
  Loader2,
  ReceiptText,
  RefreshCw,
  Tag,
  Wallet
} from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { useNavigate, useParams } from 'react-router-dom';
import { getRechargeOrder } from '../api/reader';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AppShell } from '../components/AppShell';
import type { ReaderRechargeOrder, ReaderRechargeStatus } from '../types/reader';
import { formatUsdtAmount } from '../utils/formatUsdtAmount';

const terminal = new Set<ReaderRechargeStatus>(['paid', 'superseded', 'expired', 'create_failed']);

interface OrderStatusViewProps {
  actionLabel: string;
  amount?: string;
  description: string;
  loading?: boolean;
  onAction: () => void;
  status: 'paid' | 'replaced' | 'expired' | 'checking' | 'failed';
  title: string;
}

function OrderStatusView({ actionLabel, amount, description, loading, onAction, status, title }: OrderStatusViewProps) {
  const icon = status === 'paid'
    ? <CheckCircle2 aria-hidden="true" />
    : status === 'replaced'
      ? <RefreshCw aria-hidden="true" />
      : status === 'checking'
        ? <Clock3 aria-hidden="true" />
        : <AlertTriangle aria-hidden="true" />;

  return (
    <section className={`recharge-order-status recharge-order-status--${status}`}>
      <span className="recharge-order-status__icon">{icon}</span>
      <h2>{title}</h2>
      {amount ? <strong>{amount}</strong> : null}
      <p>{description}</p>
      <button type="button" disabled={loading} onClick={onAction}>
        {loading ? <Loader2 className="spin" aria-hidden="true" /> : null}
        {loading ? '正在刷新' : actionLabel}
      </button>
    </section>
  );
}

export function RechargeOrderPage() {
  const { orderId = '' } = useParams();
  const navigate = useNavigate();
  const { isAuthenticated, refreshWallet, sessionReady } = useReaderAuth();
  const [order, setOrder] = useState<ReaderRechargeOrder | null>(null);
  const [error, setError] = useState('');
  const [remaining, setRemaining] = useState(0);
  const [copyStatus, setCopyStatus] = useState<'idle' | 'copied' | 'error'>('idle');
  const [refreshing, setRefreshing] = useState(false);
  const copyResetTimer = useRef<number | null>(null);

  const load = useCallback(async () => {
    try {
      const next = await getRechargeOrder(orderId);
      setOrder(next);
      setError('');
      if (next.status === 'paid') void refreshWallet();
    } catch (value) {
      setError(value instanceof Error ? value.message : '充值订单加载失败');
    }
  }, [orderId, refreshWallet]);

  useEffect(() => {
    if (!sessionReady) return;
    if (!isAuthenticated) {
      navigate(`/auth/login?redirect=${encodeURIComponent(`/me/recharge/orders/${orderId}`)}`, { replace: true });
      return;
    }
    void load();
  }, [isAuthenticated, load, navigate, orderId, sessionReady]);

  useEffect(() => {
    if (!order || terminal.has(order.status)) return;
    const timer = window.setInterval(() => void load(), 3000);
    return () => window.clearInterval(timer);
  }, [load, order]);

  useEffect(() => {
    const update = () => setRemaining(order?.expireTime ? Math.max(0, Math.ceil((new Date(order.expireTime.replace(' ', 'T')).getTime() - Date.now()) / 1000)) : 0);
    update();
    const timer = window.setInterval(update, 1000);
    return () => window.clearInterval(timer);
  }, [order?.expireTime]);

  useEffect(() => () => {
    if (copyResetTimer.current !== null) window.clearTimeout(copyResetTimer.current);
  }, []);

  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopyStatus('copied');
      if (copyResetTimer.current !== null) window.clearTimeout(copyResetTimer.current);
      copyResetTimer.current = window.setTimeout(() => setCopyStatus('idle'), 1800);
    } catch {
      setCopyStatus('error');
    }
  };

  const refresh = async () => {
    if (refreshing) return;
    setRefreshing(true);
    await load();
    setRefreshing(false);
  };

  const minutes = Math.floor(remaining / 60).toString().padStart(2, '0');
  const seconds = (remaining % 60).toString().padStart(2, '0');

  const statusView = order?.status === 'paid' ? (
    <OrderStatusView
      actionLabel="查看钻石明细"
      amount={`+${order.diamondAmount} 钻石`}
      description="钻石已自动存入你的账户"
      onAction={() => navigate('/me/diamonds')}
      status="paid"
      title="充值已到账"
    />
  ) : order?.status === 'superseded' ? (
    <OrderStatusView
      actionLabel="返回充值页"
      description="请按新订单显示的金额支付，不要继续向此地址转账"
      onAction={() => navigate('/me/recharge')}
      status="replaced"
      title="订单已被替换"
    />
  ) : order?.status === 'expired' ? (
    <OrderStatusView
      actionLabel="重新充值"
      description="收款地址已失效，请重新创建充值订单"
      onAction={() => navigate('/me/recharge')}
      status="expired"
      title="订单已过期"
    />
  ) : order?.status === 'callback_exception' ? (
    <OrderStatusView
      actionLabel="刷新状态"
      description="请勿重复付款，系统确认后钻石将自动到账"
      loading={refreshing}
      onAction={() => void refresh()}
      status="checking"
      title="支付状态核验中"
    />
  ) : order?.status === 'create_failed' ? (
    <OrderStatusView
      actionLabel="重新充值"
      description={order.failureMessage || '支付信息生成失败，请重新创建充值订单'}
      onAction={() => navigate('/me/recharge')}
      status="failed"
      title="订单创建失败"
    />
  ) : null;

  return (
    <AppShell title="USDT 充值" back onBack={() => navigate('/me/recharge')} hideTabs>
      <main className="recharge-order-page">
        {statusView || (order ? (
          <>
            <section className="recharge-order-amount">
              <span>需要支付</span>
              <strong>{formatUsdtAmount(order.actualAmount || order.priceUsdt)} <small>USDT</small></strong>
              <em>TRC20</em>
              {remaining > 0 ? <time><Clock3 aria-hidden="true" />{minutes}:{seconds}</time> : null}
            </section>
            <section className={`recharge-order-qr${order.receiveAddress ? '' : ' is-pending'}`} aria-label={order.receiveAddress ? '收款地址二维码' : '支付信息生成中'}>
              {order.receiveAddress ? (
                <div className="recharge-order-qr__frame">
                  <QRCodeSVG value={order.receiveAddress} size={208} level="M" />
                  <i aria-hidden="true" /><i aria-hidden="true" /><i aria-hidden="true" /><i aria-hidden="true" />
                </div>
              ) : (
                <><Loader2 className="spin" aria-hidden="true" /><span>支付信息生成中</span></>
              )}
            </section>
            <section className="recharge-order-address">
              <span className="recharge-order-address__icon"><Wallet aria-hidden="true" /></span>
              <div><span>收款地址</span><code>{order.receiveAddress || '支付信息生成中'}</code></div>
              {order.receiveAddress ? (
                <button type="button" aria-label={copyStatus === 'copied' ? '收款地址已复制' : '复制收款地址'} onClick={() => void copy(order.receiveAddress || '')}>
                  {copyStatus === 'copied' ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
                </button>
              ) : null}
            </section>
            <p className="sr-only" role="status" aria-live="polite">{copyStatus === 'copied' ? '收款地址已复制' : ''}</p>
            {copyStatus === 'error' ? <p className="recharge-error" role="alert">复制失败，请手动复制收款地址</p> : null}
            <dl className="recharge-order-meta">
              <div><span><Gem aria-hidden="true" /></span><dt>到账钻石</dt><dd>{order.diamondAmount}</dd></div>
              <div><span><Tag aria-hidden="true" /></span><dt>基础定价</dt><dd>{formatUsdtAmount(order.priceUsdt)} USDT</dd></div>
              <div><span><ReceiptText aria-hidden="true" /></span><dt>订单号</dt><dd>{order.orderNo}</dd></div>
            </dl>
            <button className="recharge-order-refresh" type="button" disabled={refreshing} onClick={() => void refresh()}>
              {refreshing ? <Loader2 className="spin" aria-hidden="true" /> : <RefreshCw aria-hidden="true" />}
              {refreshing ? '正在刷新' : '刷新状态'}
            </button>
          </>
        ) : error ? null : <section className="recharge-order-loading" role="status"><Loader2 className="spin" aria-hidden="true" /><span>订单加载中</span></section>)}
        {error ? <p className="recharge-error" role="alert">{error}</p> : null}
      </main>
    </AppShell>
  );
}
