import { Card } from 'animal-island-ui';
import type { ReaderChapterSummary } from '../types/reader';
import { formatReaderLong } from '../utils/formatReaderLong';
import { canInteractWithChapter, hasPermanentChapterAccess } from '../utils/readerAccess';

interface ChapterDrawerProps {
  open: boolean;
  chapters: ReaderChapterSummary[];
  currentChapterId: string;
  onClose: () => void;
  onSelect: (chapterId: string) => void;
}

export function getChapterAccessLabel(chapter: ReaderChapterSummary) {
  const status = chapter.accessStatus;
  if (!status) {
    return '';
  }
  if (hasPermanentChapterAccess(status)) {
    return '已购买';
  }
  if (!canInteractWithChapter(status)) {
    return '暂不可读';
  }
  if (status.membershipEntitled && status.accessReason === 'membership') {
    return '会员可读';
  }
  if (status.chargeMode === 'login_free' || status.accessReason === 'free_chapter' || String(status.chapterPrice ?? '') === '0') {
    return '免费';
  }
  if (status.accessReason === 'chapter_purchase_required' && status.chapterPrice !== null) {
    return `${formatReaderLong(status.chapterPrice)} 币`;
  }
  if (status.accessReason === 'book_purchase_required' && status.chargeMode === 'fixed_price'
    && chapter.productStatus?.priceCoin !== null && chapter.productStatus?.priceCoin !== undefined) {
    return `${formatReaderLong(chapter.productStatus.priceCoin)} 币`;
  }
  if (status.accessReason === 'membership_required') {
    return '仅限会员';
  }
  return status.readable ? '' : '暂不可读';
}

export function ChapterAccessStatus({ chapter }: { chapter: ReaderChapterSummary }) {
  const label = getChapterAccessLabel(chapter);
  return label ? <span className="chapter-access-status">{label}</span> : null;
}

export function ChapterDrawer({ open, chapters, currentChapterId, onClose, onSelect }: ChapterDrawerProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="reader-drawer reader-drawer--dock" role="dialog" aria-modal="true" aria-labelledby="reader-chapter-title">
      <div className="reader-drawer__mask" onClick={onClose} />
      <Card className="reader-drawer__panel reader-drawer__panel--dock">
        <div className="reader-drawer__head">
          <h2 className="panel-title" id="reader-chapter-title">章节目录</h2>
        </div>
        <div className="chapter-list">
          {chapters.map((chapter, index) => {
            const chapterNo = chapter.chapterNo > 0 ? chapter.chapterNo : index + 1;
            const label = `第${chapterNo}章：${chapter.chapterName}`;
            const accessLabel = getChapterAccessLabel(chapter);
            const interactive = canInteractWithChapter(chapter.accessStatus);
            return (
              <button
                className={chapter.chapterId === currentChapterId ? 'chapter-list__item is-active' : 'chapter-list__item'}
                key={chapter.chapterId}
                type="button"
                disabled={!interactive}
                aria-label={`${label}${accessLabel ? `，${accessLabel}` : ''}`}
                onClick={() => {
                  if (interactive) onSelect(chapter.chapterId);
                }}
              >
                <span className="chapter-list__name">{label}</span>
                <ChapterAccessStatus chapter={chapter} />
              </button>
            );
          })}
        </div>
      </Card>
    </div>
  );
}
