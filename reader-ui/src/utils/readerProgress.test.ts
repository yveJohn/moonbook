import { describe, expect, it } from 'vitest';
import { restorableScrollTop } from './readerProgress';
import type { ReaderChapterContent, ReaderReadingHistory } from '../types/reader';

const chapter: ReaderChapterContent = {
  chapterId: '9223372036854775807',
  bookId: '9223372036854775806',
  chapterNo: 12,
  chapterName: '第十二章',
  bookName: '月下长书',
  content: '正文',
  prevChapterId: '9223372036854775805',
  nextChapterId: '9223372036854775804'
};

const history: ReaderReadingHistory = {
  historyId: '9223372036854775803',
  bookId: '9223372036854775806',
  chapterId: '9223372036854775807',
  chapterNo: 12,
  chapterName: '第十二章',
  positionType: 'scroll',
  positionValue: 860,
  progressPercent: '48.20',
  lastReadTime: '2026-06-27 10:00:00'
};

describe('reader progress helpers', () => {
  it('restores saved scroll only when the string chapter id matches', () => {
    expect(restorableScrollTop(chapter, history)).toBe(860);
  });

  it('does not create an initial zero position for unread or different chapters', () => {
    expect(restorableScrollTop(chapter, null)).toBeNull();
    expect(restorableScrollTop(chapter, { ...history, chapterId: '9223372036854775800' })).toBeNull();
    expect(restorableScrollTop(chapter, { ...history, positionValue: 0 })).toBeNull();
  });
});
