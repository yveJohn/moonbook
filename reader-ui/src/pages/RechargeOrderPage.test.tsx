import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ReaderRechargeOrder } from '../types/reader';
import { RechargeOrderPage } from './RechargeOrderPage';

const readerApi = vi.hoisted(() => ({ getRechargeOrder: vi.fn() }));
const refreshWallet = vi.hoisted(() => vi.fn());
const writeText = vi.hoisted(() => vi.fn());

vi.mock('../api/reader', () => readerApi);
vi.mock('../auth/ReaderAuthContext', () => ({
  useReaderAuth: () => ({ isAuthenticated: true, refreshWallet, sessionReady: true })
}));

function order(status: ReaderRechargeOrder['status']): ReaderRechargeOrder {
  return {
    id: '9223372036854775807', orderNo: 'RC1', readerId: '9223372036854775801',
    sourceType: 'custom', productId: null, diamondAmount: '18', priceUsdt: '2.58',
    provider: 'epusdt', currency: 'usd', token: 'usdt', network: 'tron',
    gatewayTradeId: 'trade-1', actualAmount: '2.58', receiveAddress: 'TAddress',
    paymentUrl: 'https://pay.example.test/1', blockTransactionId: null, status,
    gatewayStatus: 1, walletLedgerId: null, expireTime: '2099-01-01 00:00:00', paidTime: null,
    failureCode: status === 'superseded' ? 'ORDER_REPLACED' : null,
    failureMessage: status === 'superseded' ? '订单已被新的充值订单替换' : null,
    createTime: '2026-07-30 12:00:00', updateTime: '2026-07-30 12:01:00'
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/me/recharge/orders/9223372036854775807']}>
      <Routes>
        <Route path="/me/recharge/orders/:orderId" element={<RechargeOrderPage />} />
        <Route path="/me/recharge" element={<div>充值页</div>} />
        <Route path="/me/diamonds" element={<div>钻石明细页</div>} />
      </Routes>
    </MemoryRouter>
  );
}

describe('RechargeOrderPage', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    writeText.mockResolvedValue(undefined);
    readerApi.getRechargeOrder.mockResolvedValue(order('superseded'));
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('shows a replaced message and stops polling a superseded order', async () => {
    renderPage();
    expect(await screen.findByRole('heading', { name: '订单已被替换' })).toBeInTheDocument();

    await act(async () => vi.advanceTimersByTime(6000));
    expect(readerApi.getRechargeOrder).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('2.58 USDT')).not.toBeInTheDocument();
  });

  it('returns to recharge creation from a superseded order', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '返回充值页' }));
    await waitFor(() => expect(screen.getByText('充值页')).toBeInTheDocument());
  });

  it('renders the pending payment details without an external payment link', async () => {
    readerApi.getRechargeOrder.mockResolvedValue(order('pending'));
    const { container } = renderPage();

    expect(await screen.findByText('需要支付')).toBeInTheDocument();
    expect(container.querySelector('.recharge-order-amount strong')).toHaveTextContent('2.58 USDT');
    expect(screen.getByLabelText('收款地址二维码')).toBeInTheDocument();
    expect(screen.getByText('TAddress')).toBeInTheDocument();
    expect(screen.getByText('18')).toBeInTheDocument();
    expect(screen.getByText('RC1')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: '打开支付页' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '刷新状态' })).toBeInTheDocument();
  });

  it('copies the receive address and announces success', async () => {
    readerApi.getRechargeOrder.mockResolvedValue(order('pending'));
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: '复制收款地址' }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith('TAddress'));
    expect(screen.getByRole('status')).toHaveTextContent('收款地址已复制');
    expect(screen.getByRole('button', { name: '收款地址已复制' })).toBeInTheDocument();
  });

  it('shows an accessible error when copying fails', async () => {
    writeText.mockRejectedValue(new Error('clipboard blocked'));
    readerApi.getRechargeOrder.mockResolvedValue(order('pending'));
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: '复制收款地址' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('复制失败，请手动复制收款地址');
  });

  it('shows the paid state, refreshes the wallet and opens diamond details', async () => {
    readerApi.getRechargeOrder.mockResolvedValue(order('paid'));
    renderPage();

    expect(await screen.findByRole('heading', { name: '充值已到账' })).toBeInTheDocument();
    expect(screen.getByText('+18 钻石')).toBeInTheDocument();
    await waitFor(() => expect(refreshWallet).toHaveBeenCalled());

    fireEvent.click(screen.getByRole('button', { name: '查看钻石明细' }));
    await waitFor(() => expect(screen.getByText('钻石明细页')).toBeInTheDocument());
  });

  it('shows the expired state and returns to recharge creation', async () => {
    readerApi.getRechargeOrder.mockResolvedValue(order('expired'));
    renderPage();

    expect(await screen.findByRole('heading', { name: '订单已过期' })).toBeInTheDocument();
    await act(async () => vi.advanceTimersByTime(6000));
    expect(readerApi.getRechargeOrder).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: '重新充值' }));
    await waitFor(() => expect(screen.getByText('充值页')).toBeInTheDocument());
  });

  it('shows a recoverable state when order creation fails', async () => {
    readerApi.getRechargeOrder.mockResolvedValue({ ...order('create_failed'), failureMessage: '网关暂时不可用' });
    renderPage();

    expect(await screen.findByRole('heading', { name: '订单创建失败' })).toBeInTheDocument();
    expect(screen.getByText('网关暂时不可用')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重新充值' })).toBeInTheDocument();
  });

  it('keeps polling and supports manual refresh while payment is being verified', async () => {
    readerApi.getRechargeOrder.mockResolvedValue(order('callback_exception'));
    renderPage();

    expect(await screen.findByRole('heading', { name: '支付状态核验中' })).toBeInTheDocument();
    await act(async () => vi.advanceTimersByTime(3100));
    expect(readerApi.getRechargeOrder).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole('button', { name: '刷新状态' }));
    await waitFor(() => expect(readerApi.getRechargeOrder).toHaveBeenCalledTimes(3));
  });

  it('shows a stable pending state while the payment address is being generated', async () => {
    readerApi.getRechargeOrder.mockResolvedValue({ ...order('creating'), receiveAddress: null });
    renderPage();

    expect(await screen.findByLabelText('支付信息生成中')).toBeInTheDocument();
    expect(screen.getAllByText('支付信息生成中')).toHaveLength(2);
    expect(screen.queryByRole('button', { name: '复制收款地址' })).not.toBeInTheDocument();
  });
});
