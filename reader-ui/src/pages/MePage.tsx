import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  buyMembership,
  checkin as submitCheckin,
  ensureInviteDashboard,
  getCheckinStatus,
  getEntitlements,
  listMembershipProducts
} from '../api/reader';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { getStoredReaderToken } from '../auth/session';
import { AppShell } from '../components/AppShell';
import { InsufficientDiamondDialog } from '../components/me/InsufficientDiamondDialog';
import { InviteRewardDialog } from '../components/me/InviteRewardDialog';
import { MeCheckinCard } from '../components/me/MeCheckinCard';
import { MeInviteCard, type InviteCopyState } from '../components/me/MeInviteCard';
import { MeProfileHeader, type BonusBalanceAnimationCommand } from '../components/me/MeProfileHeader';
import { MeQuickLinks } from '../components/me/MeQuickLinks';
import { MembershipPlanList } from '../components/me/MembershipPlanList';
import { useMeResource } from '../components/me/useMeResource';
import type { ReaderEntitlements, ReaderLongValue, ReaderProduct } from '../types/reader';

function membershipText(entitlements: ReaderEntitlements | null): string | null {
  if (!entitlements) return null;
  if (entitlements.membershipPermanent) return '永久会员';
  if (entitlements.membershipActive && entitlements.membershipExpireTime) {
    const expireTime = entitlements.membershipExpireTime.replace(' ', 'T');
    return `会员有效至 ${new Date(expireTime).toLocaleString('zh-CN', { hour12: false })}`;
  }
  return '未开通会员';
}

function errorText(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

function createRequestId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function normalizeUnsignedInteger(value: ReaderLongValue): string | null {
  const normalized = String(value).replaceAll(',', '').trim();
  if (!/^\d+$/.test(normalized)) return null;
  return normalized.replace(/^0+(?=\d)/, '');
}

function hasEnoughDiamonds(balance: ReaderLongValue, price: ReaderLongValue) {
  const normalizedBalance = normalizeUnsignedInteger(balance);
  const normalizedPrice = normalizeUnsignedInteger(price);
  if (normalizedBalance === null || normalizedPrice === null) return true;
  return normalizedBalance.length > normalizedPrice.length
    || (normalizedBalance.length === normalizedPrice.length && normalizedBalance.localeCompare(normalizedPrice) >= 0);
}

function isInsufficientDiamondError(error: unknown) {
  const message = error instanceof Error ? error.message : '';
  return message.includes('钻石余额不足') || message === '余额不足';
}

export function MePage() {
  const navigate = useNavigate();
  const {
    isAuthenticated,
    profile,
    refreshWallet,
    sessionReady,
    token,
    wallet: walletData,
    walletError,
    walletLoading
  } = useReaderAuth();
  const wallet = { data: walletData, loading: walletLoading, error: walletError, reload: refreshWallet };
  const authenticatedSessionReady = sessionReady && isAuthenticated;
  const checkin = useMeResource(getCheckinStatus, authenticatedSessionReady, '签到状态加载失败', token);
  const invite = useMeResource(ensureInviteDashboard, authenticatedSessionReady, '邀请码加载失败', token);
  const entitlements = useMeResource(getEntitlements, authenticatedSessionReady, '会员状态加载失败', token);
  const membershipProducts = useMeResource(listMembershipProducts, authenticatedSessionReady, '会员套餐加载失败', token);
  const [checkinSaving, setCheckinSaving] = useState(false);
  const [checkinCelebrating, setCheckinCelebrating] = useState(false);
  const [bonusAnimation, setBonusAnimation] = useState<BonusBalanceAnimationCommand | null>(null);
  const [membershipBuyingId, setMembershipBuyingId] = useState('');
  const [membershipActionError, setMembershipActionError] = useState('');
  const [insufficientDiamondOpen, setInsufficientDiamondOpen] = useState(false);
  const [copyState, setCopyState] = useState<InviteCopyState>('idle');
  const [shareCopyState, setShareCopyState] = useState<InviteCopyState>('idle');
  const [rewardsOpen, setRewardsOpen] = useState(false);
  const [localStateToken, setLocalStateToken] = useState(token);
  const copyResetTimer = useRef<number | null>(null);
  const shareCopyResetTimer = useRef<number | null>(null);
  const bonusAnimationSequence = useRef(0);
  const mountedRef = useRef(false);
  const tokenRef = useRef(token);
  const checkinInFlight = useRef(false);
  const checkinActionSequence = useRef(0);
  const membershipBuyingRef = useRef('');
  const membershipActionSequence = useRef(0);
  const copyActionSequence = useRef(0);
  const shareCopyActionSequence = useRef(0);
  const membershipRequestIds = useRef(new Map<string, string>());
  tokenRef.current = token;

  const canPublishForToken = (actionToken: string) => mountedRef.current
    && tokenRef.current === actionToken
    && getStoredReaderToken() === actionToken;

  useEffect(() => {
    if (sessionReady && !isAuthenticated) navigate('/auth/login?redirect=/me', { replace: true });
  }, [isAuthenticated, navigate, sessionReady]);

  useEffect(() => {
    if (sessionReady && token) void refreshWallet();
  }, [refreshWallet, sessionReady, token]);

  useEffect(() => {
    if (copyResetTimer.current !== null) window.clearTimeout(copyResetTimer.current);
    if (shareCopyResetTimer.current !== null) window.clearTimeout(shareCopyResetTimer.current);
    copyActionSequence.current += 1;
    shareCopyActionSequence.current += 1;
    setCopyState('idle');
    setShareCopyState('idle');
  }, [invite.data?.inviteCode, invite.data?.shareTextTemplate]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (copyResetTimer.current !== null) {
        window.clearTimeout(copyResetTimer.current);
        copyResetTimer.current = null;
      }
      if (shareCopyResetTimer.current !== null) {
        window.clearTimeout(shareCopyResetTimer.current);
        shareCopyResetTimer.current = null;
      }
    };
  }, []);

  useEffect(() => {
    checkinActionSequence.current += 1;
    membershipActionSequence.current += 1;
    copyActionSequence.current += 1;
    shareCopyActionSequence.current += 1;
    checkinInFlight.current = false;
    membershipBuyingRef.current = '';
    membershipRequestIds.current.clear();
    if (copyResetTimer.current !== null) {
      window.clearTimeout(copyResetTimer.current);
      copyResetTimer.current = null;
    }
    if (shareCopyResetTimer.current !== null) {
      window.clearTimeout(shareCopyResetTimer.current);
      shareCopyResetTimer.current = null;
    }
    setCheckinSaving(false);
    setCheckinCelebrating(false);
    setBonusAnimation(null);
    setMembershipBuyingId('');
    setMembershipActionError('');
    setInsufficientDiamondOpen(false);
    setCopyState('idle');
    setShareCopyState('idle');
    setRewardsOpen(false);
    setLocalStateToken(token);
  }, [token]);

  const handleCheckin = async () => {
    if (!mountedRef.current || checkinInFlight.current || checkinSaving || checkin.loading || !checkin.data || checkin.data.todayChecked
      || checkin.data.checkinAvailable === false) return;
    const actionToken = token;
    const actionSequence = ++checkinActionSequence.current;
    const canPublish = () => canPublishForToken(actionToken)
      && checkinActionSequence.current === actionSequence;
    checkinInFlight.current = true;
    const animation = wallet.data ? {
      id: `checkin-bonus-${++bonusAnimationSequence.current}`,
      from: `${wallet.data.bonusCoinBalance}`,
      to: `${wallet.data.bonusCoinBalance}`,
      active: false
    } satisfies BonusBalanceAnimationCommand : null;
    setBonusAnimation(animation);
    setCheckinSaving(true);
    checkin.setError('');
    try {
      const nextCheckin = await submitCheckin();
      if (!canPublish()) return;
      checkin.setData(nextCheckin);
      setCheckinCelebrating(true);
      const refreshedWallet = await wallet.reload();
      if (!canPublish()) return;
      if (animation && refreshedWallet) {
        setBonusAnimation({ ...animation, to: refreshedWallet.bonusCoinBalance, active: true });
      } else {
        setBonusAnimation(null);
      }
    } catch (error) {
      if (canPublish()) {
        setBonusAnimation(null);
        checkin.setError(errorText(error, '签到失败'));
      }
    } finally {
      if (canPublish()) {
        checkinInFlight.current = false;
        setCheckinSaving(false);
      }
    }
  };

  const handleCelebrationEnd = useCallback(() => {
    if (mountedRef.current) setCheckinCelebrating(false);
  }, []);

  const handleBonusAnimationComplete = useCallback((id: string) => {
    if (!mountedRef.current) return;
    setBonusAnimation((current) => current?.id === id ? null : current);
  }, []);

  const handleBuyMembership = async (product: ReaderProduct) => {
    if (!mountedRef.current || membershipBuyingRef.current || membershipBuyingId || !entitlements.data || entitlements.loading || entitlements.error
      || entitlements.data.membershipPermanent) return;
    if (wallet.data && !hasEnoughDiamonds(wallet.data.rechargeCoinBalance, product.priceCoin)) {
      setMembershipActionError('');
      setInsufficientDiamondOpen(true);
      return;
    }
    const actionToken = token;
    const actionSequence = ++membershipActionSequence.current;
    const canPublish = () => canPublishForToken(actionToken)
      && membershipActionSequence.current === actionSequence;
    const requestId = membershipRequestIds.current.get(product.id) || createRequestId();
    membershipRequestIds.current.set(product.id, requestId);
    membershipBuyingRef.current = product.id;
    setMembershipBuyingId(product.id);
    setMembershipActionError('');
    try {
      await buyMembership(product.id, requestId);
      if (!canPublish()) return;
      membershipRequestIds.current.delete(product.id);
      await Promise.allSettled([wallet.reload(), entitlements.reload()]);
    } catch (error) {
      if (canPublish()) {
        if (isInsufficientDiamondError(error)) {
          setMembershipActionError('');
          setInsufficientDiamondOpen(true);
        } else {
          setMembershipActionError(errorText(error, '会员购买失败'));
        }
      }
    } finally {
      if (canPublish()) {
        membershipBuyingRef.current = '';
        setMembershipBuyingId('');
      }
    }
  };

  const handleCopyInvite = async () => {
    const code = invite.data?.inviteCode;
    if (!mountedRef.current || !code || !invite.data?.inviteCodeAvailable || copyState === 'copying') return;
    const actionToken = token;
    const actionSequence = ++copyActionSequence.current;
    const canPublish = () => canPublishForToken(actionToken)
      && copyActionSequence.current === actionSequence;
    if (copyResetTimer.current !== null) window.clearTimeout(copyResetTimer.current);
    setCopyState('copying');
    try {
      if (!navigator.clipboard?.writeText) throw new Error('clipboard unavailable');
      await navigator.clipboard.writeText(code);
      if (!canPublish()) return;
      setCopyState('copied');
      copyResetTimer.current = window.setTimeout(() => {
        if (!canPublish()) return;
        setCopyState('idle');
        copyResetTimer.current = null;
      }, 2_000);
    } catch {
      if (canPublish()) setCopyState('error');
    }
  };

  const handleCopyInviteShare = async () => {
    const code = invite.data?.inviteCode;
    const template = invite.data?.shareTextTemplate;
    if (!mountedRef.current || !code || !template || !invite.data?.inviteCodeAvailable || shareCopyState === 'copying') return;
    const actionToken = token;
    const actionSequence = ++shareCopyActionSequence.current;
    const canPublish = () => canPublishForToken(actionToken)
      && shareCopyActionSequence.current === actionSequence;
    if (shareCopyResetTimer.current !== null) window.clearTimeout(shareCopyResetTimer.current);
    setShareCopyState('copying');
    try {
      if (!navigator.clipboard?.writeText) throw new Error('clipboard unavailable');
      const inviteUrl = new URL('/auth/register', window.location.origin);
      inviteUrl.searchParams.set('inviteCode', code);
      await navigator.clipboard.writeText(template.replaceAll('{{link}}', inviteUrl.toString()));
      if (!canPublish()) return;
      setShareCopyState('copied');
      shareCopyResetTimer.current = window.setTimeout(() => {
        if (!canPublish()) return;
        setShareCopyState('idle');
        shareCopyResetTimer.current = null;
      }, 2_000);
    } catch {
      if (canPublish()) setShareCopyState('error');
    }
  };

  const localStateCurrent = localStateToken === token;
  const visibleMembershipBuyingId = localStateCurrent ? membershipBuyingId : '';
  const purchaseDisabled = !localStateCurrent || !entitlements.data || entitlements.loading || Boolean(entitlements.error)
    || Boolean(entitlements.data?.membershipPermanent);

  return (
    <AppShell active="me" title="我的">
      <main className="reader-page me-page">
        <MeProfileHeader
          profile={profile}
          membershipText={membershipText(entitlements.data)}
          membershipActive={Boolean(entitlements.data?.membershipActive || entitlements.data?.membershipPermanent)}
          membershipLoading={entitlements.loading}
          membershipError={entitlements.error}
          onRetryMembership={entitlements.reload}
          wallet={wallet.data}
          walletLoading={wallet.loading}
          walletError={wallet.error}
          onRetryWallet={wallet.reload}
          bonusAnimation={localStateCurrent ? bonusAnimation : null}
          onBonusAnimationComplete={handleBonusAnimationComplete}
        />
        <MeCheckinCard
          status={checkin.data}
          loading={checkin.loading}
          error={checkin.error}
          saving={localStateCurrent ? checkinSaving : false}
          celebrating={localStateCurrent ? checkinCelebrating : false}
          onCheckin={() => { void handleCheckin(); }}
          onRetry={checkin.reload}
          onCelebrationEnd={handleCelebrationEnd}
        />
        <MembershipPlanList
          products={membershipProducts.data || []}
          membershipPermanent={Boolean(entitlements.data?.membershipPermanent)}
          buyingId={visibleMembershipBuyingId}
          purchaseDisabled={purchaseDisabled}
          error={membershipProducts.error}
          actionError={localStateCurrent ? membershipActionError : ''}
          loading={membershipProducts.loading}
          onRetry={membershipProducts.reload}
          onBuy={(product) => { void handleBuyMembership(product); }}
        />
        <MeInviteCard
          dashboard={invite.data}
          loading={invite.loading}
          error={invite.error}
          copyState={localStateCurrent ? copyState : 'idle'}
          shareCopyState={localStateCurrent ? shareCopyState : 'idle'}
          onCopy={() => { void handleCopyInvite(); }}
          onCopyShare={() => { void handleCopyInviteShare(); }}
          onRetry={invite.reload}
          onOpenRewards={() => setRewardsOpen(true)}
        />
        <MeQuickLinks />
        <InviteRewardDialog
          open={localStateCurrent && rewardsOpen}
          rewards={invite.data?.rewards || []}
          onClose={() => setRewardsOpen(false)}
        />
        <InsufficientDiamondDialog
          open={localStateCurrent && insufficientDiamondOpen}
          onClose={() => setInsufficientDiamondOpen(false)}
          onRecharge={() => {
            setInsufficientDiamondOpen(false);
            navigate('/me/recharge');
          }}
        />
      </main>
    </AppShell>
  );
}
