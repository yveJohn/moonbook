import type { ReaderProduct } from '../../types/reader';
import vipIcon from '../../assets/me/vip.svg';
import { MembershipPlanCard } from './MembershipPlanCard';

export interface MembershipPlanListProps {
  products: ReaderProduct[];
  membershipPermanent: boolean;
  buyingId: string;
  purchaseDisabled: boolean;
  error?: string;
  actionError?: string;
  loading?: boolean;
  onRetry?: () => void;
  onBuy: (product: ReaderProduct) => void;
}

function MembershipTitle() {
  return (
    <h2 id="membership-title" className="section-title membership-title">
      <span className="membership-title__wheat membership-title__wheat--left" aria-hidden="true" />
      <span>精选会员</span>
      <span className="membership-title__wheat membership-title__wheat--right" aria-hidden="true" />
    </h2>
  );
}

export function MembershipPlanList({
  products,
  membershipPermanent,
  buyingId,
  purchaseDisabled,
  error = '',
  actionError = '',
  loading = false,
  onRetry,
  onBuy
}: MembershipPlanListProps) {
  if (membershipPermanent) {
    return (
      <section className="content-section membership-section" aria-labelledby="membership-title" data-section="membership">
        <MembershipTitle />
        <div className="membership-permanent">
          <div className="membership-permanent__badge" aria-hidden="true">
            <img className="membership-permanent__vip-icon" src={vipIcon} alt="" />
            <span>VIP</span>
          </div>
          <div className="membership-permanent__content">
            <strong>永久会员</strong>
            <span>会员权益长期有效</span>
          </div>
        </div>
      </section>
    );
  }

  const buyingProduct = products.find((product) => product.id === buyingId);
  const purchaseStatus = buyingProduct ? `正在购买${buyingProduct.productName}` : actionError;
  const purchaseStatusClassName = actionError && !buyingProduct ? 'me-inline-error' : 'sr-only';

  return (
    <section className="content-section membership-section" aria-labelledby="membership-title" data-section="membership">
      <div className="section-head">
        <MembershipTitle />
        {loading && products.length ? <span className="section-refresh" role="status">正在刷新</span> : null}
      </div>
      {loading && !products.length ? (
        <div className="membership-plans-skeleton" aria-label="会员套餐加载中" aria-busy="true">
          <span /><span />
        </div>
      ) : error && !products.length ? (
        <div className="me-local-state me-local-state--error">
          <p>{error}</p>
          {onRetry ? <button type="button" onClick={onRetry}>重新加载会员套餐</button> : null}
        </div>
      ) : products.length ? (
        <div className="membership-plans">
          {products.map((product, index) => (
            <MembershipPlanCard
              key={product.id}
              product={product}
              buying={buyingId === product.id}
              disabled={purchaseDisabled || Boolean(buyingId)}
              priority={index === 0}
              onBuy={onBuy}
            />
          ))}
        </div>
      ) : (
        <p className="me-empty-state">暂无可购买套餐</p>
      )}
      {error && products.length ? <p className="me-inline-error">{error}</p> : null}
      <p className={purchaseStatusClassName} role="status" aria-live="polite">{purchaseStatus}</p>
    </section>
  );
}
