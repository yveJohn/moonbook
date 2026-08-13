import { isBookChargeMode, isReaderAccessReason } from '../types/reader';
import type { ReaderBookProductStatus, ReaderChapterAccess } from '../types/reader';

export function hasPermanentBookAccess(status: ReaderBookProductStatus) {
  return status.purchased || status.accessReason === 'book_owned';
}

export function hasPermanentChapterAccess(status: ReaderChapterAccess) {
  return status.bookPurchased
    || status.chapterPurchased
    || status.accessReason === 'book_owned'
    || status.accessReason === 'chapter_owned';
}

export function hasSupportedConfiguredAccess(status: { chargeMode: string | null; accessReason: string | null }) {
  return isBookChargeMode(status.chargeMode)
    && isReaderAccessReason(status.accessReason)
    && status.accessReason !== 'unsupported_mode';
}

export function canInteractWithChapter(status: ReaderChapterAccess) {
  return hasPermanentChapterAccess(status) || hasSupportedConfiguredAccess(status);
}
