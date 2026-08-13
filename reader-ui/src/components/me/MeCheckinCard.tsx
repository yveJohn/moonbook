import { useEffect, useState } from 'react';
import { useAnimatedLong } from '../../hooks/useAnimatedLong';
import { usePrefersReducedMotion } from '../../hooks/usePrefersReducedMotion';
import type { ReaderCheckinStatus } from '../../types/reader';
import type { ReaderLongValue } from '../../types/reader';
import { formatReaderLong } from '../../utils/formatReaderLong';

export interface MeCheckinCardProps {
  status: ReaderCheckinStatus | null;
  loading: boolean;
  error: string;
  saving: boolean;
  celebrating?: boolean;
  onCheckin: () => void;
  onRetry: () => void;
  onCelebrationEnd?: () => void;
}

function rewardText(status: ReaderCheckinStatus) {
  if (!status.todayChecked && status.rewardRandom) return status.rewardText || '随机金币奖励';
  return `${formatReaderLong(status.todayRewardCoin ?? 0)}金币`;
}

export function MeCheckinCard({
  status,
  loading,
  error,
  saving,
  celebrating = false,
  onCheckin,
  onRetry,
  onCelebrationEnd
}: MeCheckinCardProps) {
  const reducedMotion = usePrefersReducedMotion();
  const [rewardTarget, setRewardTarget] = useState<ReaderLongValue>(0);
  const [finishingCelebration, setFinishingCelebration] = useState(false);
  const animatedReward = useAnimatedLong(rewardTarget, {
    durationMs: 640,
    reducedMotion: reducedMotion || finishingCelebration
  });
  const disabled = loading || saving || Boolean(status?.todayChecked) || status?.checkinAvailable === false;

  useEffect(() => {
    setFinishingCelebration(false);
    setRewardTarget(celebrating ? status?.todayRewardCoin ?? 0 : 0);
  }, [celebrating, status?.todayRewardCoin]);

  useEffect(() => {
    if (!celebrating || !onCelebrationEnd) return;
    if (reducedMotion) {
      const timer = window.setTimeout(onCelebrationEnd, 120);
      return () => window.clearTimeout(timer);
    }
    const finishTimer = window.setTimeout(() => setFinishingCelebration(true), 720);
    const endTimer = window.setTimeout(onCelebrationEnd, 800);
    return () => {
      window.clearTimeout(finishTimer);
      window.clearTimeout(endTimer);
    };
  }, [celebrating, onCelebrationEnd, reducedMotion]);

  const formattedAnimatedReward = formatReaderLong(animatedReward);
  const formattedFinalReward = formatReaderLong(status?.todayRewardCoin ?? 0);
  return (
    <section className="me-checkin-card" aria-labelledby="checkin-title" aria-busy={loading} data-section="checkin">
      <div className="me-card-heading">
        <div className="me-checkin-heading">
          <span className="me-checkin-heading__icon" aria-hidden="true" />
          <h2 id="checkin-title">每日签到</h2>
        </div>
      </div>
      {loading && !status ? (
        <div className="me-checkin-skeleton" aria-label="签到状态加载中"><span /><span /></div>
      ) : error && !status ? (
        <div className="me-local-state me-local-state--error"><p>{error}</p><button type="button" onClick={onRetry}>重新加载签到状态</button></div>
      ) : status ? (
        <>
          <div className="me-checkin-stats">
            <div><span>连续签到</span><strong>{status.continuousDays.toLocaleString('zh-CN')}天</strong></div>
            <div><span>今日奖励</span><strong>{rewardText(status)}</strong></div>
          </div>
          <button type="button" className="me-checkin-action" disabled={disabled} onClick={onCheckin}>
            {status.todayChecked ? '已签到' : saving ? '签到中...' : '签到'}
          </button>
          {status.unavailableReason ? <p className="me-checkin-hint">{status.unavailableReason}</p> : null}
          {error ? <div className="me-inline-error"><span>{error}</span><button type="button" onClick={onRetry}>重试</button></div> : null}
        </>
      ) : null}
      {celebrating && status ? (
        <div className="me-checkin-celebration" aria-hidden="true">
          <strong aria-hidden="true">+{formattedAnimatedReward}金币</strong>
          <span aria-hidden="true">金币已到账</span>
        </div>
      ) : null}
      <span className="sr-only me-checkin-announcement" aria-live="polite" aria-atomic="true">
        {celebrating && status ? `签到成功，获得${formattedFinalReward}金币` : ''}
      </span>
    </section>
  );
}
