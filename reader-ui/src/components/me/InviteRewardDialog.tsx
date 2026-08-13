import { useEffect, useId, useRef } from 'react';
import { createPortal } from 'react-dom';
import type { ReaderInviteRewardRecord } from '../../types/reader';
import { formatReaderLong } from '../../utils/formatReaderLong';

export function inviteRewardLabel(reward: ReaderInviteRewardRecord) {
  if (reward.rewardStage === 'register') return '邀请注册奖励';
  if (reward.rewardStage === 'first_recharge') return '邀请首充奖励';
  return reward.remark?.trim() || '邀请奖励';
}

export function formatInviteTime(value: string | null | undefined) {
  const normalized = value?.trim() ?? '';
  if (!normalized) return '时间待确认';
  const date = new Date(normalized.replace(' ', 'T'));
  return Number.isNaN(date.getTime())
    ? normalized
    : date.toLocaleString('zh-CN', { hour12: false });
}

export interface InviteRewardDialogProps {
  open: boolean;
  rewards: ReaderInviteRewardRecord[];
  onClose: () => void;
}

const focusableSelector = [
  'button:not([disabled])',
  'a[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])'
].join(',');

interface DialogStackEntry {
  token: symbol;
  focusFallbacks: HTMLElement[];
}

const dialogStack: DialogStackEntry[] = [];
let bodyScrollLockCount = 0;
let bodyOverflowBeforeLock = '';

function registerDialog(token: symbol, opener: HTMLElement | null) {
  if (dialogStack.some((entry) => entry.token === token)) return;
  if (bodyScrollLockCount === 0) {
    bodyOverflowBeforeLock = document.body.style.overflow;
  }
  bodyScrollLockCount += 1;
  document.body.style.overflow = 'hidden';
  const focusFallbacks: HTMLElement[] = [];
  if (opener) focusFallbacks.push(opener);
  const topEntry = dialogStack[dialogStack.length - 1];
  for (const fallback of topEntry?.focusFallbacks ?? []) {
    if (!focusFallbacks.includes(fallback)) focusFallbacks.push(fallback);
  }
  dialogStack.push({ token, focusFallbacks });
}

function unregisterDialog(token: symbol) {
  const index = dialogStack.findIndex((entry) => entry.token === token);
  if (index < 0) return;
  const wasTop = index === dialogStack.length - 1;
  const [entry] = dialogStack.splice(index, 1);
  bodyScrollLockCount -= 1;
  if (bodyScrollLockCount === 0) {
    document.body.style.overflow = bodyOverflowBeforeLock;
    bodyOverflowBeforeLock = '';
  }
  if (wasTop) {
    entry.focusFallbacks.find((fallback) => fallback.isConnected)?.focus();
  }
}

function isTopDialog(token: symbol) {
  return dialogStack[dialogStack.length - 1]?.token === token;
}

export function InviteRewardDialog({ open, rewards, onClose }: InviteRewardDialogProps) {
  const dialogRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const onCloseRef = useRef(onClose);
  const dialogToken = useRef(Symbol('invite-reward-dialog')).current;
  const openerRef = useRef<HTMLElement | null>(null);
  const openerCapturedRef = useRef(false);
  const openRef = useRef(open);
  const titleId = useId();
  openRef.current = open;

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!open) return;
    if (!openerCapturedRef.current) {
      openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      openerCapturedRef.current = true;
    }
    registerDialog(dialogToken, openerRef.current);
    closeRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (!isTopDialog(dialogToken)) return;
      if (event.key === 'Escape') {
        event.preventDefault();
        onCloseRef.current();
        return;
      }
      if (event.key !== 'Tab' || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(focusableSelector));
      if (!focusable.length) {
        event.preventDefault();
        closeRef.current?.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;
      if (event.shiftKey && (active === first || !dialogRef.current.contains(active))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && (active === last || !dialogRef.current.contains(active))) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      unregisterDialog(dialogToken);
      if (!openRef.current) {
        openerRef.current = null;
        openerCapturedRef.current = false;
      }
    };
  }, [dialogToken, open]);

  if (!open) return null;

  return createPortal(
    <div
      className="invite-reward-overlay"
      onMouseDown={(event) => {
        if (isTopDialog(dialogToken) && event.target === event.currentTarget) onClose();
      }}
    >
      <section
        ref={dialogRef}
        className="invite-reward-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
      >
        <header className="invite-reward-dialog__head">
          <h2 id={titleId}>奖励明细</h2>
          <button ref={closeRef} type="button" aria-label="关闭奖励明细" onClick={onClose}>×</button>
        </header>
        {rewards.length ? (
          <ul className="invite-reward-list">
            {rewards.map((item) => (
              <li key={item.id}>
                <div><strong>{inviteRewardLabel(item)}</strong><time>{formatInviteTime(item.grantTime)}</time></div>
                <span>+{formatReaderLong(item.rewardCoin)}金币</span>
              </li>
            ))}
          </ul>
        ) : (
          <div className="invite-reward-empty">
            <strong>还没有邀请奖励</strong>
            <span>成功邀请好友后，奖励记录会显示在这里</span>
          </div>
        )}
      </section>
    </div>,
    document.body
  );
}
