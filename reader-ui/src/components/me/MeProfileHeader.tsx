import { ChevronRight } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import vipIcon from '../../assets/me/vip.svg';
import { CurrencyIcon } from '../CurrencyIcon';
import { useAnimatedLong } from '../../hooks/useAnimatedLong';
import { usePrefersReducedMotion } from '../../hooks/usePrefersReducedMotion';
import type { ReaderLongValue, ReaderProfile } from '../../types/reader';
import type { ReaderWallet } from '../../types/reader';
import { formatReaderLong } from '../../utils/formatReaderLong';

export interface BonusBalanceAnimationCommand {
  id: string;
  from: ReaderLongValue;
  to: ReaderLongValue;
  active: boolean;
}

export interface MeProfileHeaderProps {
  profile: ReaderProfile | null;
  membershipText: string | null;
  membershipActive?: boolean;
  membershipLoading: boolean;
  membershipError: string;
  onRetryMembership: () => void;
  wallet: ReaderWallet | null;
  walletLoading: boolean;
  walletError: string;
  onRetryWallet: () => void;
  bonusAnimation?: BonusBalanceAnimationCommand | null;
  onBonusAnimationComplete?: (id: string) => void;
}

function AnimatedBonusBalance({
  command,
  onComplete
}: {
  command: BonusBalanceAnimationCommand;
  onComplete?: (id: string) => void;
}) {
  const reducedMotion = usePrefersReducedMotion();
  const [target, setTarget] = useState<ReaderLongValue>(command.from);
  const animatedValue = useAnimatedLong(target, { durationMs: 640, reducedMotion });
  const completedCommand = useRef('');
  const formattedValue = formatReaderLong(animatedValue);

  useEffect(() => {
    if (command.active) setTarget(command.to);
  }, [command.active, command.to]);

  useEffect(() => {
    if (!command.active
      || formattedValue !== formatReaderLong(command.to)
      || completedCommand.current === command.id) return;
    completedCommand.current = command.id;
    onComplete?.(command.id);
  }, [command.active, command.id, command.to, formattedValue, onComplete]);

  return (
    <Link className="me-profile__balance" to="/me/coins" aria-label={`查看金币明细，当前余额 ${formattedValue}`}>
      <CurrencyIcon coinType="bonus" className="me-profile__currency-icon" />
      <span className="me-profile__balance-value me-profile__bonus-animation">{formattedValue}</span>
      <ChevronRight className="me-profile__balance-chevron" aria-hidden="true" size={16} />
    </Link>
  );
}

function WalletBalanceLink({ coin, value }: { coin: '钻石' | '金币'; value: ReaderLongValue }) {
  const formattedValue = formatReaderLong(value);
  const bonus = coin === '金币';

  return (
    <Link
      className="me-profile__balance"
      to={bonus ? '/me/coins' : '/me/diamonds'}
      aria-label={`查看${coin}明细，当前余额 ${formattedValue}`}
    >
      <CurrencyIcon coinType={bonus ? 'bonus' : 'recharge'} className="me-profile__currency-icon" />
      <span className="me-profile__balance-value">{formattedValue}</span>
      <ChevronRight className="me-profile__balance-chevron" aria-hidden="true" size={16} />
    </Link>
  );
}

export function MeProfileHeader({
  profile,
  membershipText,
  membershipActive = false,
  membershipLoading,
  membershipError,
  onRetryMembership,
  wallet,
  walletLoading,
  walletError,
  onRetryWallet,
  bonusAnimation = null,
  onBonusAnimationComplete
}: MeProfileHeaderProps) {
  const displayName = profile?.nickname?.trim() || profile?.username?.trim() || '我的阅读';
  const avatar = Array.from(displayName)[0] || '我';

  const membershipContent = membershipLoading && !membershipText ? (
    <span className="me-profile__status-skeleton" aria-label="会员状态加载中" aria-busy="true" />
  ) : membershipError && !membershipText ? (
    <span className="me-profile__status-error">
      <span>{membershipError}</span>
      <button type="button" onClick={onRetryMembership}>重试会员状态</button>
    </span>
  ) : (
    <span className="me-profile__status-content">
      <strong>{membershipText}</strong>
      <img
        className={`me-profile__vip-icon${membershipActive ? '' : ' me-profile__vip-icon--inactive'}`}
        src={vipIcon}
        alt=""
        aria-hidden="true"
      />
    </span>
  );

  return (
    <section className="me-profile-header" data-section="profile">
      <span className="me-profile__avatar" aria-hidden="true">{avatar}</span>
      <div className="me-profile__identity">
        <h1>{displayName}</h1>
      </div>
      <p className="me-profile__status">
        {membershipContent}
        {membershipText && membershipError ? (
          <span className="me-profile__status-stale-error">
            <span>{membershipError}</span>
            <button type="button" onClick={onRetryMembership}>重试会员状态</button>
          </span>
        ) : null}
      </p>
      <div className="me-profile__membership" aria-label="账户余额" aria-busy={walletLoading}>
        {wallet ? (
          <>
            <WalletBalanceLink coin="钻石" value={wallet.rechargeCoinBalance} />
            {bonusAnimation ? (
              <AnimatedBonusBalance key={bonusAnimation.id} command={bonusAnimation} onComplete={onBonusAnimationComplete} />
            ) : (
              <WalletBalanceLink coin="金币" value={wallet.bonusCoinBalance} />
            )}
          </>
        ) : walletLoading ? (
          <span className="me-profile__wallet-skeleton" aria-label="钱包加载中"><i /><i /></span>
        ) : walletError ? (
          <span className="me-profile__wallet-error">
            <span>{walletError}</span>
            <button type="button" onClick={onRetryWallet}>重新加载钱包</button>
          </span>
        ) : null}
        {wallet && walletError ? (
          <span className="me-profile__wallet-stale-error">
            <span>{walletError}</span>
            <button type="button" onClick={onRetryWallet}>重新加载钱包</button>
          </span>
        ) : null}
      </div>
    </section>
  );
}
