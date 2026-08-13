import { useEffect, useState } from 'react';
import diamondIcon from '../../assets/currency/moonbook-diamond.png';
import type { ReaderProduct } from '../../types/reader';
import { formatReaderLong } from '../../utils/formatReaderLong';
import { membershipImageFor } from './membershipImages';

export interface MembershipPlanCardProps {
  product: ReaderProduct;
  buying: boolean;
  disabled: boolean;
  priority: boolean;
  onBuy: (product: ReaderProduct) => void;
}

export function MembershipPlanCard({ product, buying, disabled, priority, onBuy }: MembershipPlanCardProps) {
  const image = membershipImageFor(product.durationDays);
  const [imageFailed, setImageFailed] = useState(false);
  const formattedPrice = formatReaderLong(product.priceCoin);

  useEffect(() => setImageFailed(false), [image]);

  return (
    <button
      type="button"
      className="membership-plan-card"
      disabled={disabled}
      aria-label={`${buying ? '正在购买' : '购买'}${product.productName}，${formattedPrice}钻石`}
      aria-busy={buying}
      onClick={() => onBuy(product)}
    >
      <span className="membership-plan-card__media">
        {image && !imageFailed ? (
          <img
            src={image}
            alt=""
            role="presentation"
            loading={priority ? 'eager' : 'lazy'}
            onError={() => setImageFailed(true)}
          />
        ) : (
          <span className="membership-plan-card__placeholder" aria-hidden="true">MOONBOOK</span>
        )}
        <span className="membership-plan-card__price" aria-hidden="true">
          <img className="membership-plan-card__price-icon" src={diamondIcon} alt="" />
          {formattedPrice}
        </span>
        {buying ? <span className="membership-plan-card__status">购买中...</span> : null}
      </span>
    </button>
  );
}
