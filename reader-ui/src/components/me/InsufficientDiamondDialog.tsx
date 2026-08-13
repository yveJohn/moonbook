import { useEffect, useId, useRef } from 'react';
import { createPortal } from 'react-dom';

export interface InsufficientDiamondDialogProps {
  open: boolean;
  onClose: () => void;
  onRecharge: () => void;
}

const focusableSelector = 'button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])';

export function InsufficientDiamondDialog({ open, onClose, onRecharge }: InsufficientDiamondDialogProps) {
  const dialogRef = useRef<HTMLElement>(null);
  const rechargeRef = useRef<HTMLButtonElement>(null);
  const openerRef = useRef<HTMLElement | null>(null);
  const onCloseRef = useRef(onClose);
  const titleId = useId();

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!open) return;
    openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    rechargeRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onCloseRef.current();
        return;
      }
      if (event.key !== 'Tab' || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(focusableSelector));
      if (!focusable.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = previousOverflow;
      openerRef.current?.focus();
      openerRef.current = null;
    };
  }, [open]);

  if (!open) return null;

  return createPortal(
    <div className="insufficient-diamond-overlay" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section
        ref={dialogRef}
        className="insufficient-diamond-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
      >
        <h2 id={titleId}>钻石余额不足</h2>
        <p>当前钻石不足以购买该会员套餐，充值后即可继续购买。</p>
        <div className="insufficient-diamond-dialog__actions">
          <button type="button" className="insufficient-diamond-dialog__cancel" onClick={onClose}>取消</button>
          <button ref={rechargeRef} type="button" className="insufficient-diamond-dialog__recharge" onClick={onRecharge}>去充值</button>
        </div>
      </section>
    </div>,
    document.body
  );
}
