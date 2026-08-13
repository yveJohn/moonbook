import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RechargePage, calculateRechargeUsdt } from './RechargePage';

const readerApi = vi.hoisted(() => ({
  createRechargeOrder: vi.fn(),
  getRechargeCatalog: vi.fn(),
  quoteRecharge: vi.fn()
}));

vi.mock('../api/reader', () => readerApi);
vi.mock('../auth/ReaderAuthContext', () => ({
  useReaderAuth: () => ({ isAuthenticated: true, sessionReady: true })
}));

const catalog = {
  products: [
    { id: '9223372036854775801', productName: '17 钻石', diamondAmount: '17', priceUsdt: '2.00', saleStatus: 'enabled', sortOrder: 1 },
    { id: '9223372036854775802', productName: '35 钻石', diamondAmount: '35', priceUsdt: '5.00', saleStatus: 'enabled', sortOrder: 2 }
  ],
  customEnabled: true,
  diamondsPerUsdt: '7',
  minDiamondAmount: '7',
  maxDiamondAmount: '70000'
};

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/me/recharge']}>
      <Routes>
        <Route path="/me/recharge" element={<RechargePage />} />
        <Route path="/me/recharge/orders/:orderId" element={<div>订单支付页</div>} />
        <Route path="/me/diamonds" element={<div>钻石明细页</div>} />
      </Routes>
    </MemoryRouter>
  );
}

describe('RechargePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    readerApi.getRechargeCatalog.mockResolvedValue(catalog);
    readerApi.createRechargeOrder.mockResolvedValue({ id: '9223372036854775807' });
  });

  afterEach(cleanup);

  it.each([
    ['7', '1.00'],
    ['8', '1.15'],
    ['9', '1.29']
  ])('calculates %s diamonds as %s USDT without floating point rounding', (diamonds, expected) => {
    expect(calculateRechargeUsdt(diamonds, '7')).toBe(expected);
  });

  it('updates the USDT amount immediately and never requests a quote', async () => {
    renderPage();
    const input = await screen.findByRole('textbox', { name: '自定义钻石数量' });

    fireEvent.change(input, { target: { value: '7' } });
    expect(screen.getByText('1.00 USDT')).toBeInTheDocument();
    fireEvent.change(input, { target: { value: '8' } });
    expect(screen.getByText('1.15 USDT')).toBeInTheDocument();
    expect(screen.queryByText('1.00 USDT')).not.toBeInTheDocument();
    expect(readerApi.quoteRecharge).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: '计算金额' })).not.toBeInTheDocument();
  });

  it('hides the amount for empty or out-of-range values and shows the valid range', async () => {
    renderPage();
    const input = await screen.findByRole('textbox', { name: '自定义钻石数量' });

    fireEvent.change(input, { target: { value: '6' } });
    expect(screen.getByRole('alert')).toHaveTextContent('钻石数量必须在 7 到 70000 之间');
    expect(screen.getByRole('button', { name: '立即充值' })).toBeDisabled();
    fireEvent.change(input, { target: { value: '' } });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '立即充值' })).toBeDisabled();
  });

  it('selects a preset before creating its order', async () => {
    renderPage();
    const secondProduct = await screen.findByRole('button', { name: /35 钻石/ });

    fireEvent.click(secondProduct);
    expect(secondProduct).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: '立即充值' }));

    await waitFor(() => expect(readerApi.createRechargeOrder).toHaveBeenCalledWith({
      productId: '9223372036854775802',
      requestId: expect.any(String)
    }));
  });

  it('creates a custom order with the current diamond string', async () => {
    renderPage();
    fireEvent.change(await screen.findByRole('textbox', { name: '自定义钻石数量' }), { target: { value: '9' } });
    fireEvent.click(screen.getByRole('button', { name: '立即充值' }));

    await waitFor(() => expect(readerApi.createRechargeOrder).toHaveBeenCalledWith({
      customDiamondAmount: '9',
      requestId: expect.any(String)
    }));
    expect(await screen.findByText('订单支付页')).toBeInTheDocument();
  });
});
