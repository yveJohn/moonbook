import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReaderLongValue, ReaderProduct } from '../../types/reader';
import { MembershipPlanList } from './MembershipPlanList';

function product(id: string, durationDays: number | null, productName: string, priceCoin: ReaderLongValue): ReaderProduct {
  return {
    id,
    productType: 'membership',
    targetId: null,
    productName,
    priceCoin,
    allowBonusCoin: 0,
    durationDays,
    saleStatus: 'on_sale',
    sortOrder: 0,
    remark: '',
    createTime: '2026-07-11 20:00:00',
    updateTime: '2026-07-11 20:00:00'
  };
}

describe('MembershipPlanList', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders only the mapped media, price overlay and accessible purchase name', () => {
    const { container } = render(
      <MembershipPlanList
        products={[product('9223372036854775801', 30, '会员月卡', 99)]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        onBuy={vi.fn()}
      />
    );

    expect(container.querySelector('.membership-title__wheat--left')).toBeInTheDocument();
    expect(container.querySelector('.membership-title__wheat--right')).toBeInTheDocument();
    const button = screen.getByRole('button', { name: '购买会员月卡，99钻石' });
    expect(button).toBeEnabled();
    const image = within(button).getByRole('presentation');
    expect(image).toHaveAttribute('alt', '');
    expect(image).toHaveAttribute('src', expect.stringContaining('moonbook-membership-month.webp'));
    const price = within(button).getByText('99');
    expect(price).toHaveClass('membership-plan-card__price');
    const priceIcon = price.querySelector('.membership-plan-card__price-icon');
    expect(priceIcon).toHaveAttribute('src', expect.stringContaining('moonbook-diamond.png'));
    expect(priceIcon).toHaveAttribute('alt', '');
    expect(within(button).queryByText('会员月卡')).not.toBeInTheDocument();
    expect(within(button).queryByText('立即购买')).not.toBeInTheDocument();
    expect(button.querySelector('.membership-plan-card__body')).toBeNull();
  });

  it('loads only the first membership image eagerly', () => {
    render(
      <MembershipPlanList
        products={[
          product('9223372036854775801', 7, '会员周卡', 30),
          product('9223372036854775802', 30, '会员月卡', 99),
          product('9223372036854775803', 90, '会员季卡', 259)
        ]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        onBuy={vi.fn()}
      />
    );

    const images = screen.getAllByRole('presentation');
    expect(images).toHaveLength(3);
    expect(images[0]).toHaveAttribute('loading', 'eager');
    expect(images[1]).toHaveAttribute('loading', 'lazy');
    expect(images[2]).toHaveAttribute('loading', 'lazy');
  });

  it('uses the fixed media placeholder after an image error', () => {
    const { container } = render(
      <MembershipPlanList
        products={[product('9223372036854775801', 30, '会员月卡', 99)]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        onBuy={vi.fn()}
      />
    );
    fireEvent.error(screen.getByRole('presentation'));
    expect(container.querySelector('.membership-plan-card__placeholder')).not.toBeNull();
  });

  it('formats a serialized long price in visible and accessible names while preserving the product id', () => {
    const onBuy = vi.fn();
    const plan = product('9223372036854775807', 30, '大额会员月卡', '9007199254740992');
    render(
      <MembershipPlanList
        products={[plan]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        onBuy={onBuy}
      />
    );

    const button = screen.getByRole('button', { name: '购买大额会员月卡，9,007,199,254,740,992钻石' });
    expect(within(button).getByText('9,007,199,254,740,992')).toBeInTheDocument();
    fireEvent.click(button);
    expect(onBuy).toHaveBeenCalledWith(plan);
    expect(onBuy.mock.calls[0][0].id).toBe('9223372036854775807');
  });

  it('locks every plan while one string id is purchasing', () => {
    const plans = [
      product('9223372036854775801', 7, '会员周卡', 30),
      product('9223372036854775802', 365, '会员年卡', 699)
    ];
    const { rerender } = render(
      <MembershipPlanList
        products={plans}
        membershipPermanent={false}
        buyingId="9223372036854775801"
        purchaseDisabled={false}
        onBuy={vi.fn()}
      />
    );
    const buyingButton = screen.getByRole('button', { name: '正在购买会员周卡，30钻石' });
    const lockedButton = screen.getByRole('button', { name: '购买会员年卡，699钻石' });
    expect(buyingButton).toBeDisabled();
    expect(buyingButton).toHaveAttribute('aria-busy', 'true');
    expect(lockedButton).toBeDisabled();
    expect(lockedButton).toHaveAttribute('aria-busy', 'false');
    expect(within(buyingButton).getByText('购买中...')).toHaveClass('membership-plan-card__status');
    expect(within(buyingButton).queryByText('立即购买')).not.toBeInTheDocument();
    expect(screen.getAllByRole('status')).toHaveLength(1);
    expect(screen.getByRole('status')).toHaveClass('sr-only');
    expect(screen.getByRole('status')).toHaveTextContent('正在购买会员周卡');
    expect(buyingButton.querySelector('[aria-live]')).toBeNull();
    expect(lockedButton.querySelector('[aria-live]')).toBeNull();

    rerender(
      <MembershipPlanList
        products={plans}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        onBuy={vi.fn()}
      />
    );
    expect(screen.getByRole('button', { name: '购买会员周卡，30钻石' })).toBeEnabled();
    expect(screen.getByRole('status')).toHaveTextContent('');
  });

  it('hides plans for a permanent member', () => {
    const { container } = render(
      <MembershipPlanList
        products={[product('9223372036854775801', null, '永久会员', 999)]}
        membershipPermanent
        buyingId=""
        purchaseDisabled={false}
        onBuy={vi.fn()}
      />
    );
    expect(screen.getByRole('heading', { name: '精选会员' })).toBeInTheDocument();
    expect(screen.getByText('永久会员')).toBeInTheDocument();
    expect(container.querySelectorAll('.membership-title__wheat')).toHaveLength(2);
    expect(container.querySelector('.membership-permanent__badge')).toHaveTextContent('VIP');
    expect(container.querySelector('.membership-permanent__vip-icon')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /购买永久会员/ })).not.toBeInTheDocument();
  });

  it('uses a fixed placeholder for an unknown product duration', () => {
    const { container } = render(
      <MembershipPlanList
        products={[product('9223372036854775803', 180, '会员半年卡', 399)]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        onBuy={vi.fn()}
      />
    );
    expect(container.querySelector('.membership-plan-card__placeholder')).not.toBeNull();
    expect(screen.queryByRole('presentation')).not.toBeInTheDocument();
  });

  it('keeps products visible but disables purchase until entitlements are known', () => {
    render(
      <MembershipPlanList
        products={[product('9223372036854775804', 7, '会员周卡', 30)]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled
        onBuy={vi.fn()}
      />
    );
    expect(screen.getByRole('button', { name: '购买会员周卡，30钻石' })).toBeDisabled();
  });

  it('renders product and purchase errors locally with retry', () => {
    const retry = vi.fn();
    const { rerender } = render(
      <MembershipPlanList
        products={[]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        error="会员套餐加载失败"
        onRetry={retry}
        onBuy={vi.fn()}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: '重新加载会员套餐' }));
    expect(retry).toHaveBeenCalledTimes(1);

    rerender(
      <MembershipPlanList
        products={[product('9223372036854775805', 365, '会员年卡', 699)]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        actionError="会员购买失败"
        onRetry={retry}
        onBuy={vi.fn()}
      />
    );
    expect(screen.getByRole('status')).toHaveTextContent('会员购买失败');
    expect(screen.getByRole('button', { name: '购买会员年卡，699钻石' })).toBeEnabled();
  });

  it('does not render a retry button without a retry callback', () => {
    render(
      <MembershipPlanList
        products={[]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled={false}
        error="会员套餐加载失败"
        onBuy={vi.fn()}
      />
    );

    expect(screen.getByText('会员套餐加载失败')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '重新加载会员套餐' })).not.toBeInTheDocument();
  });

  it('uses a stable skeleton without fake products while loading', () => {
    const { container } = render(
      <MembershipPlanList
        products={[]}
        membershipPermanent={false}
        buyingId=""
        purchaseDisabled
        loading
        onBuy={vi.fn()}
      />
    );
    expect(container.querySelector('.membership-plans-skeleton')).not.toBeNull();
    expect(screen.queryByRole('button', { name: /购买/ })).not.toBeInTheDocument();
  });
});
