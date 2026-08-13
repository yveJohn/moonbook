import type { ReaderChapterContent, ReaderReadingHistory } from '../types/reader';

export function restorableScrollTop(
  chapter: ReaderChapterContent,
  history: ReaderReadingHistory | null | undefined
): number | null {
  if (!history || history.chapterId !== chapter.chapterId || history.positionValue <= 0) {
    return null;
  }
  return history.positionValue;
}

export function currentScrollTop(): number {
  return Math.max(0, Math.round(window.scrollY));
}

export function progressPercent(): string {
  const maxScroll = document.documentElement.scrollHeight - window.innerHeight;
  if (maxScroll <= 0) {
    return '0.00';
  }
  const percent = Math.min(Math.max(window.scrollY / maxScroll, 0), 1) * 100;
  return percent.toFixed(2);
}
