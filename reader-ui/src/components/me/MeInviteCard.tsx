import { Check, ChevronRight, Copy, Gift } from 'lucide-react';
import { CurrencyIcon } from '../CurrencyIcon';
import type { ReaderInviteDashboard } from '../../types/reader';
import { formatReaderLong } from '../../utils/formatReaderLong';

export type InviteCopyState = 'idle' | 'copying' | 'copied' | 'error';

export interface MeInviteCardProps {
  dashboard: ReaderInviteDashboard | null;
  loading: boolean;
  error: string;
  copyState: InviteCopyState;
  shareCopyState: InviteCopyState;
  onCopy: () => void;
  onCopyShare: () => void;
  onRetry: () => void;
  onOpenRewards: () => void;
}

const copyLabels: Record<InviteCopyState, string> = {
  idle: '复制邀请码',
  copying: '复制中...',
  copied: '已复制',
  error: '重新复制'
};

const shareCopyLabels: Record<InviteCopyState, string> = {
  idle: '复制邀请文案',
  copying: '复制中...',
  copied: '文案已复制',
  error: '重新复制文案'
};

export function MeInviteCard({
  dashboard,
  loading,
  error,
  copyState,
  shareCopyState,
  onCopy,
  onCopyShare,
  onRetry,
  onOpenRewards
}: MeInviteCardProps) {
  if (loading && !dashboard) {
    return (
      <section className="me-invite-card me-invite-skeleton" aria-label="邀请信息加载中" aria-busy="true" data-section="invite">
        <span /><span /><span />
      </section>
    );
  }
  if (!dashboard) {
    return (
      <section className="me-invite-card me-local-state me-local-state--error" aria-labelledby="invite-title" data-section="invite">
        <h2 id="invite-title">邀请好友</h2>
        <p>{error || '邀请码暂不可用'}</p>
        <button type="button" onClick={onRetry}>重新获取邀请码</button>
      </section>
    );
  }

  const copyDisabled = loading || copyState === 'copying'
    || !dashboard.inviteCodeAvailable || !dashboard.inviteCode;
  const shareCopyDisabled = loading || shareCopyState === 'copying'
    || !dashboard.inviteCodeAvailable || !dashboard.inviteCode;
  const hasRewardCopy = dashboard.registerRewardCoin !== undefined && dashboard.registerRewardCoin !== null
    && dashboard.firstRechargeRewardCoin !== undefined && dashboard.firstRechargeRewardCoin !== null;

  return (
    <section className="me-invite-card" aria-labelledby="invite-title" aria-busy={loading} data-section="invite">
      <div className="me-card-heading">
        <div className="me-invite-heading">
          <span className="me-card-kicker">邀请计划</span>
          <h2 id="invite-title">邀请好友</h2>
          {hasRewardCopy ? (
            <div className="me-invite-reward-grid" aria-label="邀请奖励规则">
              <div className="me-invite-reward me-invite-reward--register">
                <span>注册奖励</span>
                <div>
                  <strong>{formatReaderLong(dashboard.registerRewardCoin!)}</strong>
                  <CurrencyIcon coinType="bonus" className="me-invite-reward__coin" />
                  <span className="sr-only">金币</span>
                </div>
              </div>
              <div className="me-invite-reward me-invite-reward--recharge">
                <span>首充奖励</span>
                <div>
                  <strong>{formatReaderLong(dashboard.firstRechargeRewardCoin!)}</strong>
                  <CurrencyIcon coinType="bonus" className="me-invite-reward__coin" />
                  <span className="sr-only">金币</span>
                </div>
              </div>
            </div>
          ) : null}
        </div>
        {loading ? <span className="section-refresh" role="status">正在刷新</span> : null}
      </div>
      <div className="me-invite-code">
        <span>我的邀请码</span>
        <code>{dashboard.inviteCode || '--'}</code>
        <button type="button" disabled={copyDisabled} onClick={onCopy}>
          {copyState === 'copied' ? <Check aria-hidden="true" size={16} /> : <Copy aria-hidden="true" size={16} />}
          {copyLabels[copyState]}
        </button>
      </div>
      <button className="me-invite-share" type="button" disabled={shareCopyDisabled} onClick={onCopyShare}>
        {shareCopyState === 'copied' ? <Check aria-hidden="true" size={16} /> : <Copy aria-hidden="true" size={16} />}
        {shareCopyLabels[shareCopyState]}
      </button>
      {copyState === 'error' ? <p className="me-inline-error" role="alert">复制失败，请重试</p> : null}
      {shareCopyState === 'error' ? <p className="me-inline-error" role="alert">邀请文案复制失败，请重试</p> : null}
      {!dashboard.inviteCodeAvailable ? (
        <div className="me-invite-unavailable-state">
          <p className="me-invite-unavailable">{dashboard.inviteCodeUnavailableReason || '邀请码暂不可用'}</p>
          <button type="button" onClick={onRetry}>重新检查邀请码</button>
        </div>
      ) : null}
      <dl className="me-invite-stats">
        <div>
          <dt>已邀请</dt>
          <dd>{formatReaderLong(dashboard.invitedCount)}</dd>
        </div>
        <div>
          <dt>累计金币</dt>
          <dd>{formatReaderLong(dashboard.totalRewardCoin)}</dd>
        </div>
      </dl>
      <button type="button" className="me-invite-rewards" aria-label="奖励明细" onClick={onOpenRewards}>
        <span><Gift aria-hidden="true" />奖励明细</span>
        <ChevronRight aria-hidden="true" />
      </button>
      {error ? (
        <div className="me-inline-error">
          <span>{error}</span>
          <button type="button" onClick={onRetry}>重新获取邀请码</button>
        </div>
      ) : null}
    </section>
  );
}
