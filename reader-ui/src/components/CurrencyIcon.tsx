import coinIcon from '../assets/currency/moonbook-coin.png';
import diamondIcon from '../assets/currency/moonbook-diamond.png';
import type { ReaderWalletCoinType } from '../types/reader';

interface CurrencyIconProps {
  coinType: ReaderWalletCoinType;
  className?: string;
}

export function CurrencyIcon({ coinType, className }: CurrencyIconProps) {
  return (
    <img
      src={coinType === 'recharge' ? diamondIcon : coinIcon}
      alt=""
      aria-hidden="true"
      className={className}
    />
  );
}
